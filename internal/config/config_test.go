package config

import (
	"testing"
)

func TestParseDNSServer(t *testing.T) {
	cfg := &Config{}

	tests := []struct {
		input       string
		expected    string
		expectError bool
	}{
		// UDP tests
		{"8.8.8.8", "8.8.8.8:53", false},
		{"8.8.8.8:53", "8.8.8.8:53", false},
		{"114.114.114.114", "114.114.114.114:53", false},

		// TCP tests
		{"tcp://8.8.8.8", "tcp://8.8.8.8:53", false},
		{"tcp://8.8.8.8:53", "tcp://8.8.8.8:53", false},
		{"tcp://1.1.1.1", "tcp://1.1.1.1:53", false},

		// TLS tests
		{"tls://8.8.8.8", "tls://8.8.8.8:853", false},
		{"tls://8.8.8.8:853", "tls://8.8.8.8:853", false},
		{"tls://1.1.1.1", "tls://1.1.1.1:853", false},

		// Error cases
		{"", "", true},
		{"   ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := cfg.parseDNSServer(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("parseDNSServer(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidDomainName(t *testing.T) {
	cfg := &Config{}

	tests := []struct {
		domain  string
		isValid bool
	}{
		// Valid domains
		{"example.com", true},
		{"google.com", true},
		{"github.com", true},
		{"api.github.com", true},
		{"a", true},
		{"ab", true},
		{"a.b", true},
		{"x.y.z", true},
		{"google.com.hk", true},

		// Invalid domains
		{"", false},
		{"-example.com", false},
		{"example-.com", false},
		{"example.com-", false},
		{"example_underscore.com", false},
		{"example@com", false},
		{" example.com", false},
		{"example.com ", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := cfg.isValidDomainName(tt.domain)
			if result != tt.isValid {
				t.Errorf("isValidDomainName(%q) = %v, want %v", tt.domain, result, tt.isValid)
			}
		})
	}

	// Test length validation (max 253 chars per RFC 1035)
	longDomain := ""
	for i := 0; i < 254; i++ {
		longDomain += "a"
	}
	if cfg.isValidDomainName(longDomain) != false {
		t.Errorf("isValidDomainName should return false for domain longer than 253 chars")
	}

	validLongDomain := ""
	for i := 0; i < 253; i++ {
		validLongDomain += "a"
	}
	if cfg.isValidDomainName(validLongDomain) != true {
		t.Errorf("isValidDomainName should return true for domain of exactly 253 chars")
	}
}
