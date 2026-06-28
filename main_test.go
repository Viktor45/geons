package main

import (
	"net"
	"reflect"
	"testing"

	"github.com/oschwald/geoip2-golang/v2"
)

func TestExtractField_CountryRecord(t *testing.T) {
	// Test geoip2.Country structure with correct field paths for v2
	country := &geoip2.Country{
		Country: geoip2.CountryRecord{ISOCode: "US", Names: geoip2.Names{English: "United States"}},
	}

	v := reflect.ValueOf(country)

	got := extractField(v, "Country.ISOCode")
	if got != "US" {
		t.Fatalf("expected Country.ISOCode 'US', got '%s'", got)
	}

	got = extractField(v, "Country.Names.English")
	if got != "United States" {
		t.Fatalf("expected Country.Names.English 'United States', got '%s'", got)
	}
}

func TestExtractField_CityAndASN(t *testing.T) {
	city := &geoip2.City{
		City:         geoip2.CityRecord{Names: geoip2.Names{English: "San Francisco"}},
		Subdivisions: []geoip2.CitySubdivision{{ISOCode: "CA", Names: geoip2.Names{German: "Kalifornien"}}},
	}

	v := reflect.ValueOf(city)

	got := extractField(v, "City.Names.English")
	if got != "San Francisco" {
		t.Fatalf("expected City.Names.English 'San Francisco', got '%s'", got)
	}

	got = extractField(v, "Subdivisions[0].Names.German")
	if got != "Kalifornien" {
		t.Fatalf("expected Subdivisions[0].Names.German 'Kalifornien', got '%s'", got)
	}

	asn := &geoip2.ASN{AutonomousSystemNumber: 64496, AutonomousSystemOrganization: "Example ASN Org"}
	v = reflect.ValueOf(asn)

	got = extractField(v, "AutonomousSystemNumber")
	if got != "64496" {
		t.Fatalf("expected AutonomousSystemNumber '64496', got '%s'", got)
	}

	got = extractField(v, "AutonomousSystemOrganization")
	if got != "Example ASN Org" {
		t.Fatalf("expected AutonomousSystemOrganization 'Example ASN Org', got '%s'", got)
	}
}

func TestExtractField_NilPointerSafely(t *testing.T) {
	type Inner struct {
		Value string
	}
	type Outer struct {
		Ptr *Inner
	}

	o := &Outer{Ptr: nil}
	v := reflect.ValueOf(o)

	got := extractField(v, "Ptr.Value")
	if got != "" {
		t.Fatalf("expected empty string for nil pointer path, got '%s'", got)
	}
}

func TestIsAllowed(t *testing.T) {
	// prepare allowedNets
	allowedNets = nil
	cidrs := []string{"127.0.0.0/8", "192.168.0.0/16"}
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			t.Fatalf("failed to parse cidr %s: %v", c, err)
		}
		allowedNets = append(allowedNets, n)
	}

	if !isAllowed(net.ParseIP("127.0.0.1")) {
		t.Fatalf("127.0.0.1 should be allowed")
	}

	if isAllowed(net.ParseIP("8.8.8.8")) {
		t.Fatalf("8.8.8.8 should NOT be allowed")
	}
}
