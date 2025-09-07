package kafka

import (
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/parser"
)

func TestKafkaClient_SendAggregateReport(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create test configuration
	cfg := &config.KafkaConfig{
		Enabled:        true,
		Hosts:          []string{"localhost:9092"},
		Username:       "",
		Password:       "",
		SSL:            false,
		SkipVerify:     false,
		AggregateTopic: "dmarc.aggregate",
		ForensicTopic:  "dmarc.forensic",
		SMTPTLSTopic:   "dmarc.smtp_tls",
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

	// We expect a connection error since we don't have a Kafka broker running
	if err == nil {
		t.Error("Expected connection error, got nil")
	}

	// Check that error is related to connection, not parsing/formatting
	if err != nil && !strings.Contains(err.Error(), "connection") &&
		!strings.Contains(err.Error(), "dial") && !strings.Contains(err.Error(), "refused") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestKafkaClient_SendForensicReport(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := &config.KafkaConfig{
		Enabled:       true,
		Hosts:         []string{"localhost:9092"},
		ForensicTopic: "dmarc.forensic",
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

	if err != nil && !strings.Contains(err.Error(), "connection") &&
		!strings.Contains(err.Error(), "dial") && !strings.Contains(err.Error(), "refused") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestKafkaClient_SendSMTPTLSReport(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := &config.KafkaConfig{
		Enabled:      true,
		Hosts:        []string{"localhost:9092"},
		SMTPTLSTopic: "dmarc.smtp_tls",
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

	if err != nil && !strings.Contains(err.Error(), "connection") &&
		!strings.Contains(err.Error(), "dial") && !strings.Contains(err.Error(), "refused") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

func TestKafkaClient_DisabledClient(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create disabled Kafka configuration
	cfg := &config.KafkaConfig{
		Enabled:        false,
		Hosts:          []string{"localhost:9092"},
		AggregateTopic: "dmarc.aggregate",
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

func TestKafkaClient_EmptyTopic(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create configuration without topic
	cfg := &config.KafkaConfig{
		Enabled:        true,
		Hosts:          []string{"localhost:9092"},
		AggregateTopic: "", // Empty topic
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

	// Test that client doesn't send when topic is empty
	err := client.SendAggregateReport(report)
	if err != nil {
		t.Errorf("Client with empty topic should not return error, got: %v", err)
	}
}

func TestKafkaClient_NoHosts(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create configuration without hosts
	cfg := &config.KafkaConfig{
		Enabled:        true,
		Hosts:          []string{}, // Empty hosts
		AggregateTopic: "dmarc.aggregate",
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

	// Test that client returns error for no hosts
	err := client.SendAggregateReport(report)
	if err == nil {
		t.Error("Expected error for no hosts, got nil")
	}
}

func TestKafkaClient_TestConnection(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name    string
		config  *config.KafkaConfig
		wantErr bool
	}{
		{
			name: "Disabled client",
			config: &config.KafkaConfig{
				Enabled: false,
			},
			wantErr: true, // Should return error for disabled client
		},
		{
			name: "No hosts",
			config: &config.KafkaConfig{
				Enabled: true,
				Hosts:   []string{},
			},
			wantErr: true, // Should return error for no hosts
		},
		{
			name: "Invalid host",
			config: &config.KafkaConfig{
				Enabled: true,
				Hosts:   []string{"localhost:9092"},
			},
			wantErr: true, // Should return connection error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New(tt.config, logger)
			err := client.TestConnection()

			if (err != nil) != tt.wantErr {
				t.Errorf("TestConnection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKafkaClient_WithSSL(t *testing.T) {
	logger := zaptest.NewLogger(t)

	cfg := &config.KafkaConfig{
		Enabled:        true,
		Hosts:          []string{"localhost:9093"}, // SSL port
		SSL:            true,
		SkipVerify:     true, // Skip cert verification for test
		Username:       "test-user",
		Password:       "test-password",
		AggregateTopic: "dmarc.aggregate",
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

	// Test sending with SSL configuration (will fail with connection error)
	err := client.SendAggregateReport(report)

	// We expect a connection error
	if err == nil {
		t.Error("Expected connection error, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "connection") &&
		!strings.Contains(err.Error(), "dial") && !strings.Contains(err.Error(), "refused") {
		t.Errorf("Expected connection error, got: %v", err)
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
