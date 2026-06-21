package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
	"gopkg.in/yaml.v3"
)

// --- Configuration ---

type Config struct {
	Server ServerConfig `yaml:"server"`
	Zones  []ZoneConfig `yaml:"zones"`
}

type ServerConfig struct {
	Port         int      `yaml:"port"`
	BindAddress  string   `yaml:"bind_address"`
	AllowedCIDRs []string `yaml:"allowed_clients"`
}

type ZoneConfig struct {
	Name     string         `yaml:"name"`
	Database DatabaseConfig `yaml:"database"`
	Response ResponseConfig `yaml:"response"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
	Type string `yaml:"type"` // "country", "city", or "asn"
}

type ResponseConfig struct {
	Separator string   `yaml:"separator"`
	Fields    []string `yaml:"fields"`
}

// --- Runtime structures ---

type Zone struct {
	Name      string
	Database  *geoip2.Reader
	DbType    string
	Fields    []string
	Separator string
}

// --- Global variables ---

var (
	mu          sync.RWMutex
	cfg         Config
	zones       map[string]*Zone
	allowedNets []*net.IPNet
)

func main() {
	// 1. Initial configuration load
	if err := loadConfig("GEONS_CONFIG", "./data/config.yaml"); err != nil {
		log.Fatalf("Initial load error: %v", err)
	}

	// 2. Setup and start DNS server
	dns.HandleFunc(".", handleDNS)

	mu.RLock()
	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddress, cfg.Server.Port)
	mu.RUnlock()

	log.Printf("Starting server on %s (UDP)...", addr)

	server := &dns.Server{Addr: addr, Net: "udp"}

	// Channel for server errors
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ListenAndServe()
	}()

	// 3. Signal handling setup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	log.Println("Server started successfully. Waiting for signals...")

	// 4. Main signal processing loop
	for {
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP signal, reloading configuration...")
				if err := reloadConfig(); err != nil {
					log.Printf("Configuration reload error: %v", err)
				} else {
					log.Println("Configuration successfully reloaded")
				}
			case syscall.SIGINT, syscall.SIGTERM:
				log.Printf("Received %v signal, shutting down...", sig)
				if err := server.Shutdown(); err != nil {
					log.Printf("Graceful shutdown error: %v", err)
				}
				log.Println("Server stopped gracefully")
				return
			}
		case err := <-errChan:
			if err != nil {
				log.Fatalf("Server error: %v", err)
			}
			return
		}
	}
}

// --- Configuration loading and reloading ---

func loadConfig(name string, fallback string) error {
	configPath := os.Getenv(name)
	if configPath == "" {
		configPath = fallback
	}
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to resolve config path %s: %v", configPath, err)
	}
	info, err := os.Stat(absConfigPath)
	if err != nil {
		return fmt.Errorf("error statting config file %s: %v", absConfigPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("config path %s is a directory, expected a file", absConfigPath)
	}

	configData, err := os.ReadFile(absConfigPath)
	if err != nil {
		return fmt.Errorf("error reading %s: %v", absConfigPath, err)
	}

	var newCfg Config
	if err := yaml.Unmarshal(configData, &newCfg); err != nil {
		return fmt.Errorf("YAML parsing error: %v", err)
	}

	if newCfg.Server.Port == 0 {
		return fmt.Errorf("server.port must be set and greater than zero")
	}

	newCfg.Server.BindAddress = strings.TrimSpace(newCfg.Server.BindAddress)
	if newCfg.Server.BindAddress == "" {
		newCfg.Server.BindAddress = "127.0.0.1"
	}
	if net.ParseIP(newCfg.Server.BindAddress) == nil {
		return fmt.Errorf("invalid server.bind_address: %s", newCfg.Server.BindAddress)
	}

	// Parse new allowed CIDRs
	var newAllowedNets []*net.IPNet
	for _, cidr := range newCfg.Server.AllowedCIDRs {
		_, netIP, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("invalid CIDR %s: %v", cidr, err)
		}
		newAllowedNets = append(newAllowedNets, netIP)
	}

	// Load zones
	newZones := make(map[string]*Zone)
	for _, zoneCfg := range newCfg.Zones {
		zone, err := loadZone(zoneCfg, filepath.Dir(absConfigPath))
		if err != nil {
			// Close already opened zones on error
			for _, z := range newZones {
				if z.Database != nil {
					z.Database.Close()
				}
			}
			return fmt.Errorf("error loading zone %s: %v", zoneCfg.Name, err)
		}
		newZones[zone.Name] = zone
		log.Printf("Loaded zone: %s (database: %s, type: %s)", zone.Name, zoneCfg.Database.Path, zone.DbType)
	}

	if len(newZones) == 0 {
		return fmt.Errorf("at least one zone must be configured")
	}

	// Apply changes under lock
	mu.Lock()

	// Close old zones
	for _, z := range zones {
		if z.Database != nil {
			z.Database.Close()
		}
	}

	cfg = newCfg
	zones = newZones
	allowedNets = newAllowedNets

	mu.Unlock()

	return nil
}

func loadZone(zoneCfg ZoneConfig, configDir string) (*Zone, error) {
	// Validate zone name
	zoneCfg.Name = strings.TrimSpace(zoneCfg.Name)
	zoneCfg.Name = strings.TrimSuffix(zoneCfg.Name, ".")
	if zoneCfg.Name == "" {
		return nil, fmt.Errorf("zone name must be set")
	}
	if !strings.HasPrefix(zoneCfg.Name, ".") {
		zoneCfg.Name = "." + zoneCfg.Name
	}

	// Validate database type
	zoneCfg.Database.Type = strings.TrimSpace(strings.ToLower(zoneCfg.Database.Type))
	if zoneCfg.Database.Type == "" {
		return nil, fmt.Errorf("database.type must be set")
	}
	if zoneCfg.Database.Type != "country" && zoneCfg.Database.Type != "city" && zoneCfg.Database.Type != "asn" {
		return nil, fmt.Errorf("invalid database.type: %s (must be 'country', 'city', or 'asn')", zoneCfg.Database.Type)
	}

	// Resolve database path
	zoneCfg.Database.Path = strings.TrimSpace(zoneCfg.Database.Path)
	if zoneCfg.Database.Path == "" {
		return nil, fmt.Errorf("database.path must be set")
	}
	zoneCfg.Database.Path = filepath.Clean(zoneCfg.Database.Path)
	if !filepath.IsAbs(zoneCfg.Database.Path) {
		zoneCfg.Database.Path = filepath.Join(configDir, zoneCfg.Database.Path)
	}

	dbInfo, err := os.Stat(zoneCfg.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("error opening database file %s: %v", zoneCfg.Database.Path, err)
	}
	if dbInfo.IsDir() {
		return nil, fmt.Errorf("database.path %s is a directory, expected MMDB file", zoneCfg.Database.Path)
	}

	// Open database
	db, err := geoip2.Open(zoneCfg.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("error opening MMDB: %v", err)
	}

	return &Zone{
		Name:      zoneCfg.Name,
		Database:  db,
		DbType:    zoneCfg.Database.Type,
		Fields:    zoneCfg.Response.Fields,
		Separator: zoneCfg.Response.Separator,
	}, nil
}

func reloadConfig() error {
	return loadConfig("GEONS_CONFIG", "./data/config.yaml")
}

// --- DNS request handler ---

func handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	// ACL check (whitelist) under RLock
	remoteAddr := w.RemoteAddr().String()
	clientIPStr, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		clientIPStr = remoteAddr
	}
	clientIPStr = strings.Trim(clientIPStr, "[]")
	clientIP := net.ParseIP(clientIPStr)

	mu.RLock()
	allowed := isAllowed(clientIP)
	mu.RUnlock()

	if !allowed {
		log.Printf("Access denied for IP: %s", clientIPStr)
		m.Rcode = dns.RcodeRefused
		w.WriteMsg(m)
		return
	}

	if len(r.Question) == 0 {
		w.WriteMsg(m)
		return
	}

	q := r.Question[0]

	// We only handle TXT requests
	if q.Qtype != dns.TypeTXT {
		m.Rcode = dns.RcodeNotImplemented
		w.WriteMsg(m)
		return
	}

	// Parse IP and determine zone from domain name
	qname := strings.ToLower(strings.TrimSuffix(q.Name, "."))

	// Find matching zone
	mu.RLock()
	var matchedZone *Zone
	var ipStr string
	for zoneName, zone := range zones {
		suffix := strings.ToLower(strings.TrimPrefix(zoneName, "."))
		if strings.HasSuffix(qname, suffix) {
			matchedZone = zone
			ipStr = strings.TrimSuffix(qname, "."+suffix)
			break
		}
	}
	mu.RUnlock()

	if matchedZone == nil {
		m.Rcode = dns.RcodeNameError // NXDOMAIN
		w.WriteMsg(m)
		return
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		m.Rcode = dns.RcodeNameError
		w.WriteMsg(m)
		return
	}

	// MaxMind database lookup under RLock
	mu.RLock()
	var record interface{}
	var lookupErr error

	switch matchedZone.DbType {
	case "city":
		record, lookupErr = matchedZone.Database.City(ip)
	case "asn":
		record, lookupErr = matchedZone.Database.ASN(ip)
	default: // country
		record, lookupErr = matchedZone.Database.Country(ip)
	}

	fields := matchedZone.Fields
	separator := matchedZone.Separator
	mu.RUnlock()

	if lookupErr != nil {
		log.Printf("IP lookup error for %s: %v", ipStr, lookupErr)
		setTXTResponse(m, q.Name, "ERROR")
		w.WriteMsg(m)
		return
	}
	if record == nil {
		log.Printf("IP lookup returned nil record for %s", ipStr)
		setTXTResponse(m, q.Name, "UNKNOWN")
		w.WriteMsg(m)
		return
	}

	// Extract configured fields using reflection
	var parts []string
	v := reflect.ValueOf(record)
	for _, fieldPath := range fields {
		val := extractField(v, fieldPath)
		if val != "" {
			parts = append(parts, val)
		}
	}

	resStr := strings.Join(parts, separator)
	if resStr == "" {
		resStr = "UNKNOWN"
	}

	setTXTResponse(m, q.Name, resStr)
	w.WriteMsg(m)
}

func setTXTResponse(m *dns.Msg, name, text string) {
	txt := &dns.TXT{
		Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
		Txt: []string{text},
	}
	m.Answer = append(m.Answer, txt)
}

// --- Helper functions ---

func isAllowed(ip net.IP) bool {
	for _, n := range allowedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// extractField extracts value from geoip2 structure by path
// Supports array indexing like "Subdivisions[0].Names.en"
func extractField(v reflect.Value, path string) string {
	parts := strings.Split(path, ".")

	for _, part := range parts {
		if !v.IsValid() {
			return ""
		}

		// Safely dereference pointers and interfaces
		for v.IsValid() && (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) {
			if v.IsNil() {
				return ""
			}
			v = v.Elem()
		}

		if !v.IsValid() {
			return ""
		}

		// Check for array/slice indexing (e.g., "Subdivisions[0]")
		arrayMatch := regexp.MustCompile(`^([^\[]+)\[(\d+)\]$`).FindStringSubmatch(part)
		if arrayMatch != nil {
			fieldName := arrayMatch[1]
			indexStr := arrayMatch[2]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return ""
			}

			// Get the field first
			if v.Kind() == reflect.Struct {
				v = v.FieldByName(fieldName)
				if !v.IsValid() {
					return ""
				}
			} else {
				return ""
			}

			// Dereference if needed
			for v.IsValid() && (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) {
				if v.IsNil() {
					return ""
				}
				v = v.Elem()
			}

			// Now access the array/slice element
			if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
				if index >= v.Len() {
					return "" // Index out of bounds
				}
				v = v.Index(index)
			} else {
				return "" // Not a slice/array
			}
		} else {
			// Regular field access
			switch v.Kind() {
			case reflect.Struct:
				v = v.FieldByName(part)
				if !v.IsValid() {
					return ""
				}
			case reflect.Map:
				mapKey := reflect.ValueOf(part)
				v = v.MapIndex(mapKey)
				if !v.IsValid() {
					return ""
				}
				for v.IsValid() && (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) {
					if v.IsNil() {
						return ""
					}
					v = v.Elem()
				}
			default:
				return ""
			}
		}
	}

	if !v.IsValid() {
		return ""
	}

	for v.IsValid() && (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	if !v.IsValid() {
		return ""
	}

	if v.Kind() == reflect.String {
		return v.String()
	}
	return fmt.Sprintf("%v", v.Interface())
}
