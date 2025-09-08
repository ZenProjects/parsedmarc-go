package utils

import (
	"encoding/base64"
	"testing"
)

func TestDecodeBase64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Wikipedia example",
			input:    "YW55IGNhcm5hbCBwbGVhcw==",
			expected: "any carnal pleas",
		},
		{
			name:     "Hello world",
			input:    base64.StdEncoding.EncodeToString([]byte("Hello, World!")),
			expected: "Hello, World!",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeBase64(tt.input)
			if err != nil {
				t.Fatalf("DecodeBase64(%q) error = %v", tt.input, err)
			}
			if string(result) != tt.expected {
				t.Errorf("DecodeBase64(%q) = %q, want %q", tt.input, string(result), tt.expected)
			}
		})
	}
}

func TestDecodeBase64Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Invalid characters",
			input: "invalid!@#$%",
		},
		{
			name:  "Invalid length",
			input: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeBase64(tt.input)
			if err == nil {
				t.Errorf("DecodeBase64(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestGetBaseDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple subdomain",
			input:    "foo.example.com",
			expected: "example.com",
		},
		{
			name:     "Multiple subdomains",
			input:    "mail.subdomain.example.com",
			expected: "example.com",
		},
		{
			name:     "Akamai edge case",
			input:    "e3191.c.akamaiedge.net",
			expected: "c.akamaiedge.net",
		},
		{
			name:     "Already base domain",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "Top level domain",
			input:    "com",
			expected: "com",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBaseDomain(tt.input)
			if result != tt.expected {
				t.Errorf("GetBaseDomain(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple hostname",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "With trailing dot",
			input:    "example.com.",
			expected: "example.com",
		},
		{
			name:     "Uppercase",
			input:    "EXAMPLE.COM",
			expected: "example.com",
		},
		{
			name:     "Mixed case with trailing dot",
			input:    "Example.COM.",
			expected: "example.com",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeHost(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeHost(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
