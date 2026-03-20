package util

import (
	"testing"

	"github.com/miekg/dns"
)

func TestExtractIPAddresses(t *testing.T) {
	tests := []struct {
		name     string
		msg      *dns.Msg
		expected []string
	}{
		{
			name:     "nil message",
			msg:      nil,
			expected: nil,
		},
		{
			name:     "empty answer",
			msg:      &dns.Msg{},
			expected: nil,
		},
		{
			name: "single A record",
			msg: &dns.Msg{
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA},
						A:   []byte{1, 2, 3, 4},
					},
				},
			},
			expected: []string{"1.2.3.4"},
		},
		{
			name: "single AAAA record",
			msg: &dns.Msg{
				Answer: []dns.RR{
					&dns.AAAA{
						Hdr:  dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeAAAA},
						AAAA: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
					},
				},
			},
			expected: []string{"102:304:506:708:90a:b0c:d0e:f10"},
		},
		{
			name: "mixed A and AAAA records",
			msg: &dns.Msg{
				Answer: []dns.RR{
					&dns.A{
						Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA},
						A:   []byte{1, 2, 3, 4},
					},
					&dns.AAAA{
						Hdr:  dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeAAAA},
						AAAA: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
					},
				},
			},
			expected: []string{"1.2.3.4", "102:304:506:708:90a:b0c:d0e:f10"},
		},
		{
			name: "only AAAA when looking for A",
			msg: &dns.Msg{
				Answer: []dns.RR{
					&dns.AAAA{
						Hdr:  dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeAAAA},
						AAAA: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
					},
				},
			},
			expected: []string{"102:304:506:708:90a:b0c:d0e:f10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractIPAddresses(tt.msg)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d IPs, got %d", len(tt.expected), len(result))
				return
			}
			for i, ip := range result {
				if ip != tt.expected[i] {
					t.Errorf("expected IP %s at index %d, got %s", tt.expected[i], i, ip)
				}
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"google.com", "google.com"},
		{"www.google.com", "google.com"},
		{"api.github.com", "github.com"},
		{"google.com.hk", "google.com.hk"},
		{"google.co.uk", "google.co.uk"},
		{"example.com", "example.com"},
		{"a.b", "a.b"},
		{"com", "com"},
		{"localhost", "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ExtractDomain(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractDomain(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
