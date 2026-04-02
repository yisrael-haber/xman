package main

import (
	"strconv"
	"testing"

	"xman/internal/sshtransport"
)

func TestSplitSSHHost(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPort string
		wantErr  bool
	}{
		{name: "hostname default port", input: "example.com", wantHost: "example.com", wantPort: "22"},
		{name: "hostname explicit port", input: "example.com:2222", wantHost: "example.com", wantPort: "2222"},
		{name: "ipv4 explicit port", input: "192.168.1.10:2200", wantHost: "192.168.1.10", wantPort: "2200"},
		{name: "ipv6 default port", input: "2001:db8::10", wantHost: "2001:db8::10", wantPort: "22"},
		{name: "ipv6 explicit port", input: "[2001:db8::10]:2200", wantHost: "2001:db8::10", wantPort: "2200"},
		{name: "bracketed ipv6 default port", input: "[2001:db8::10]", wantHost: "2001:db8::10", wantPort: "22"},
		{name: "invalid port", input: "example.com:abc", wantErr: true},
		{name: "empty", input: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sshtransport.ParseHost(tt.input, 22)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseHost(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseHost(%q) error = %v", tt.input, err)
			}
			if got.Host != tt.wantHost || strconv.Itoa(got.Port) != tt.wantPort {
				t.Fatalf("ParseHost(%q) = (%q, %d), want (%q, %q)", tt.input, got.Host, got.Port, tt.wantHost, tt.wantPort)
			}
		})
	}
}
