package main

import "testing"

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
			gotHost, gotPort, err := splitSSHHost(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("splitSSHHost(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitSSHHost(%q) error = %v", tt.input, err)
			}
			if gotHost != tt.wantHost || gotPort != tt.wantPort {
				t.Fatalf("splitSSHHost(%q) = (%q, %q), want (%q, %q)", tt.input, gotHost, gotPort, tt.wantHost, tt.wantPort)
			}
		})
	}
}
