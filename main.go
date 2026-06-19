package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"

	"github.com/miekg/dns"
	"github.com/oschwald/geoip2-golang"
	"gopkg.in/yaml.v3"
)

// --- Configuration ---

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Response ResponseConfig `yaml:"response"`
}

type ServerConfig struct {
	Port         int      `yaml:"port"`
	BindAddress  string   `yaml:"bind_address"`
	DomainSuffix string   `yaml:"domain_suffix"`
	AllowedCIDRs []string `yaml:"allowed_clients"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type ResponseConfig struct {
	Separator string   `yaml:"separator"`
	Fields    []string `yaml:"fields"`
}

// --- Global variables ---

var (
	mu          sync.RWMutex
	cfg         Config
	db          *geoip2.Reader
	allowedNets []*net.IPNet
)

func main() {
	// 1. Initial configuration load
	if err := loadConfig(); err != nil {
		log.Fatalf("Initial load error: %v", err)
	}

	// 2. Setup and start DNS server
	dns.HandleFunc(".", handleDNS)

	mu.RLock()
	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddress, cfg.Server.Port)
	mu.RUnlock()

	log.Printf("Starting DNS server on %s (UDP)...", addr)

	server := &dns.Server{Addr: addr, Net: "udp"}

	// Channel for server errors
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.ListenAndServe()
	}()

	// 3. Signal handling setup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	log.Println("Server started. Press Ctrl+C to stop or send SIGHUP to reload configuration")

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

func loadConfig() error {
	configPath := "config.yaml"
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

	newCfg.Server.DomainSuffix = strings.TrimSpace(newCfg.Server.DomainSuffix)
	newCfg.Server.DomainSuffix = strings.TrimSuffix(newCfg.Server.DomainSuffix, ".")
	if newCfg.Server.DomainSuffix == "" {
		return fmt.Errorf("server.domain_suffix must be set")
	}
	if !strings.HasPrefix(newCfg.Server.DomainSuffix, ".") {
		newCfg.Server.DomainSuffix = "." + newCfg.Server.DomainSuffix
	}

	newCfg.Server.BindAddress = strings.TrimSpace(newCfg.Server.BindAddress)
	if newCfg.Server.BindAddress == "" {
		newCfg.Server.BindAddress = "127.0.0.1"
	}
	if net.ParseIP(newCfg.Server.BindAddress) == nil {
		return fmt.Errorf("invalid server.bind_address: %s", newCfg.Server.BindAddress)
	}

	newCfg.Database.Path = strings.TrimSpace(newCfg.Database.Path)
	if newCfg.Database.Path == "" {
		return fmt.Errorf("database.path must be set")
	}
	newCfg.Database.Path = filepath.Clean(newCfg.Database.Path)
	if !filepath.IsAbs(newCfg.Database.Path) {
		newCfg.Database.Path = filepath.Join(filepath.Dir(absConfigPath), newCfg.Database.Path)
	}

	dbInfo, err := os.Stat(newCfg.Database.Path)
	if err != nil {
		return fmt.Errorf("error opening database file %s: %v", newCfg.Database.Path, err)
	}
	if dbInfo.IsDir() {
		return fmt.Errorf("database.path %s is a directory, expected MMDB file", newCfg.Database.Path)
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

	// Open new database
	newDB, err := geoip2.Open(newCfg.Database.Path)
	if err != nil {
		return fmt.Errorf("error opening MMDB: %v", err)
	}

	// Apply changes under lock
	mu.Lock()

	// Close old database after successful opening of new one
	if db != nil {
		db.Close()
	}

	cfg = newCfg
	db = newDB
	allowedNets = newAllowedNets

	mu.Unlock()

	return nil
}

func reloadConfig() error {
	return loadConfig()
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
		// fallback: remote may be an IP without port
		clientIPStr = remoteAddr
	}
	// Trim IPv6 brackets if present
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

	// Parse IP from domain name
	qname := strings.ToLower(strings.TrimSuffix(q.Name, "."))

	mu.RLock()
	suffix := strings.ToLower(strings.TrimPrefix(cfg.Server.DomainSuffix, "."))
	mu.RUnlock()

	if !strings.HasSuffix(qname, suffix) {
		m.Rcode = dns.RcodeNameError // NXDOMAIN
		w.WriteMsg(m)
		return
	}

	ipStr := strings.TrimSuffix(qname, "."+suffix)
	ip := net.ParseIP(ipStr)
	if ip == nil {
		m.Rcode = dns.RcodeNameError
		w.WriteMsg(m)
		return
	}

	// MaxMind database lookup under RLock
	mu.RLock()
	record, err := db.Country(ip)
	fields := cfg.Response.Fields
	separator := cfg.Response.Separator
	mu.RUnlock()

	if err != nil {
		log.Printf("IP lookup error for %s: %v", ipStr, err)
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

// extractField extracts value from geoip2.Country structure by path
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
