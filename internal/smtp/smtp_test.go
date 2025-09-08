package smtp

import (
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/parser"
)

func TestSMTPClient_SendAggregateReport(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create test configuration
	cfg := &config.SMTPConfig{
		Enabled:    true,
		Host:       "localhost",
		Port:       25,
		SSL:        false,
		Username:   "",
		Password:   "",
		From:       "test@example.com",
		To:         []string{"recipient@example.com"},
		Subject:    "Test DMARC Report",
		Attachment: "",
		Message:    "Test message",
	}

	client := New(cfg, logger)

	// Create test aggregate report
	report := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:   "Test Org",
			OrgEmail:  "test@example.com",
			ReportID:  "test-123",
			BeginDate: time.Now().Add(-24 * time.Hour),
			EndDate:   time.Now(),
		},
		PolicyPublished: parser.PolicyPublished{
			Domain: "example.com",
			ADKIM:  "r",
			ASPF:   "r",
			P:      "none",
			SP:     "none",
			PCT:    "100",
			FO:     "0",
		},
		Records: []parser.Record{
			{
				Source: parser.Source{
					IPAddress:  "192.168.1.1",
					Country:    "US",
					ReverseDNS: "mail.example.com",
				},
				Count: 1,
				PolicyEvaluated: parser.PolicyEvaluated{
					Disposition: "none",
					DKIM:        "pass",
					SPF:         "pass",
				},
				Identifiers: parser.Identifiers{
					HeaderFrom: "example.com",
				},
			},
		},
	}

	// Test sending (this will fail with connection error, but we can test the logic)
	err := client.SendAggregateReport(report)

	// We expect a connection error since we don't have an SMTP server running
	if err == nil {
		t.Error("Expected connection error, got nil")
	}

	// Check that error is related to connection, not parsing/formatting
	if err != nil && !strings.Contains(err.Error(), "connection") && !strings.Contains(err.Error(), "dial") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestSMTPClient_SendForensicReport(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := &config.SMTPConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    25,
		From:    "test@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Forensic Report",
	}

	client := New(cfg, logger)

	// Create test forensic report
	report := &parser.ForensicReport{
		FeedbackType: "auth-failure",
		UserAgent:    stringPtr("Test/1.0"),
		Version:      stringPtr("1.0"),
		ArrivalDate:  time.Now(),
		Subject:      "Test failure report",
		MessageID:    "<test@example.com>",
		Source: parser.Source{
			IPAddress: "192.168.1.100",
			Country:   "US",
		},
		AuthFailure:    []string{"dmarc"},
		ReportedDomain: "example.com",
	}

	// Test sending (will fail with connection error)
	err := client.SendForensicReport(report)

	// We expect a connection error
	if err == nil {
		t.Error("Expected connection error, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "connection") && !strings.Contains(err.Error(), "dial") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestSMTPClient_SendSMTPTLSReport(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := &config.SMTPConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    25,
		From:    "test@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test SMTP TLS Report",
	}

	client := New(cfg, logger)

	// Create test SMTP TLS report
	report := &parser.SMTPTLSReport{
		OrganizationName: "Test Org",
		BeginDate:        time.Now().Add(-24 * time.Hour),
		EndDate:          time.Now(),
		ContactInfo:      "test@example.com",
		ReportID:         "smtp-tls-123",
		Policies: []parser.SMTPTLSPolicy{
			{
				PolicyDomain:           "example.com",
				PolicyType:             "tlsa",
				SuccessfulSessionCount: 100,
				FailedSessionCount:     5,
			},
		},
	}

	// Test sending (will fail with connection error)
	err := client.SendSMTPTLSReport(report)

	// We expect a connection error
	if err == nil {
		t.Error("Expected connection error, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "connection") && !strings.Contains(err.Error(), "dial") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestSMTPClient_DisabledClient(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create disabled SMTP configuration
	cfg := &config.SMTPConfig{
		Enabled: false,
		Host:    "localhost",
		Port:    25,
		From:    "test@example.com",
		To:      []string{"recipient@example.com"},
	}

	client := New(cfg, logger)

	// Create a dummy report
	report := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:  "Test Org",
			ReportID: "test-123",
		},
	}

	// Test that disabled client doesn't send
	err := client.SendAggregateReport(report)
	if err != nil {
		t.Errorf("Disabled client should not return error, got: %v", err)
	}
}

func TestSMTPClient_NoRecipients(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create configuration without recipients
	cfg := &config.SMTPConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    25,
		From:    "test@example.com",
		To:      []string{}, // Empty recipients
	}

	client := New(cfg, logger)

	// Create a dummy report
	report := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:  "Test Org",
			ReportID: "test-123",
		},
	}

	// Test that client returns error for no recipients
	err := client.SendAggregateReport(report)
	if err == nil {
		t.Error("Expected error for no recipients, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "recipients") {
		t.Errorf("Expected recipients error, got: %v", err)
	}
}

func TestEncodeBase64(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "Simple text",
			input:    []byte("Hello, World!"),
			expected: "SGVsbG8sIFdvcmxkIQ==",
		},
		{
			name:     "Empty data",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "JSON data",
			input:    []byte(`{"test": "data"}`),
			expected: "eyJ0ZXN0IjogImRhdGEifQ==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodeBase64(tt.input)

			// Remove line breaks for comparison
			result = strings.ReplaceAll(result, "\r\n", "")
			result = strings.TrimSpace(result)

			if result != tt.expected {
				t.Errorf("encodeBase64(%q) = %q, want %q", string(tt.input), result, tt.expected)
			}
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
