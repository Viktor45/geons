package main

import (
	"context"
	"fmt"
	"net"
)

func main() {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := &net.Dialer{}
			return dialer.DialContext(ctx, "udp", "127.0.0.1:5300")
		},
	}

	answers, err := resolver.LookupTXT(context.Background(), "8.8.8.8.geons")
	if err != nil {
		panic(err)
	}

	fmt.Printf("TXT answers: %v\n", answers)
}
