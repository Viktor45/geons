package main

import (
	"net"
	"reflect"
	"testing"
)

func TestExtractField_NestedAndMap(t *testing.T) {
	type CountryStruct struct {
		IsoCode string
		Names   map[string]string
	}
	type Record struct {
		Country CountryStruct
	}

	r := &Record{Country: CountryStruct{IsoCode: "US", Names: map[string]string{"en": "United States"}}}

	v := reflect.ValueOf(r)

	got := extractField(v, "Country.IsoCode")
	if got != "US" {
		t.Fatalf("expected IsoCode 'US', got '%s'", got)
	}

	got = extractField(v, "Country.Names.en")
	if got != "United States" {
		t.Fatalf("expected Names.en 'United States', got '%s'", got)
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
