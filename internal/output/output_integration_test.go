package output

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	"parsedmarc-go/internal/parser"
)

// MockSMTPSender implements SMTPSender for testing
type MockSMTPSender struct {
	SentReports []interface{}
	ShouldError bool
}

func (m *MockSMTPSender) SendAggregateReport(report *parser.AggregateReport) error {
	if m.ShouldError {
		return &testError{"mock SMTP error"}
	}
	m.SentReports = append(m.SentReports, report)
	return nil
}

func (m *MockSMTPSender) SendForensicReport(report *parser.ForensicReport) error {
	if m.ShouldError {
		return &testError{"mock SMTP error"}
	}
	m.SentReports = append(m.SentReports, report)
	return nil
}

func (m *MockSMTPSender) SendSMTPTLSReport(report *parser.SMTPTLSReport) error {
	if m.ShouldError {
		return &testError{"mock SMTP error"}
	}
	m.SentReports = append(m.SentReports, report)
	return nil
}

// MockKafkaSender implements KafkaSender for testing
type MockKafkaSender struct {
	SentReports []interface{}
	ShouldError bool
}

func (m *MockKafkaSender) SendAggregateReport(report *parser.AggregateReport) error {
	if m.ShouldError {
		return &testError{"mock Kafka error"}
	}
	m.SentReports = append(m.SentReports, report)
	return nil
}

func (m *MockKafkaSender) SendForensicReport(report *parser.ForensicReport) error {
	if m.ShouldError {
		return &testError{"mock Kafka error"}
	}
	m.SentReports = append(m.SentReports, report)
	return nil
}

func (m *MockKafkaSender) SendSMTPTLSReport(report *parser.SMTPTLSReport) error {
	if m.ShouldError {
		return &testError{"mock Kafka error"}
	}
	m.SentReports = append(m.SentReports, report)
	return nil
}

// testError implements error interface
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

func TestJSONWriter_WithSMTPAndKafka(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create mock senders
	mockSMTP := &MockSMTPSender{}
	mockKafka := &MockKafkaSender{}

	// Create output buffer
	var buf bytes.Buffer

	// Create JSON writer with mock senders
	writer := &JSONWriter{
		writer:      &buf,
		closer:      nil,
		smtpSender:  mockSMTP,
		kafkaSender: mockKafka,
		logger:      logger,
	}

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
		},
		Records: []parser.Record{
			{
				Source: parser.Source{
					IPAddress: "192.168.1.1",
					Country:   "US",
				},
				Count: 1,
			},
		},
	}

	// Write the report
	err := writer.WriteAggregateReport(report)
	if err != nil {
		t.Fatalf("WriteAggregateReport failed: %v", err)
	}

	// Verify JSON was written to buffer
	output := buf.String()
	if !strings.Contains(output, "Test Org") {
		t.Error("JSON output doesn't contain expected data")
	}

	if !strings.Contains(output, "test-123") {
		t.Error("JSON output doesn't contain report ID")
	}

	// Verify SMTP sender was called
	if len(mockSMTP.SentReports) != 1 {
		t.Errorf("Expected 1 SMTP report, got %d", len(mockSMTP.SentReports))
	}

	// Verify Kafka sender was called
	if len(mockKafka.SentReports) != 1 {
		t.Errorf("Expected 1 Kafka report, got %d", len(mockKafka.SentReports))
	}
}

func TestJSONWriter_WithSMTPError(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create mock senders - SMTP will error
	mockSMTP := &MockSMTPSender{ShouldError: true}
	mockKafka := &MockKafkaSender{}

	var buf bytes.Buffer

	writer := &JSONWriter{
		writer:      &buf,
		closer:      nil,
		smtpSender:  mockSMTP,
		kafkaSender: mockKafka,
		logger:      logger,
	}

	report := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:  "Test Org",
			ReportID: "test-123",
		},
	}

	// Write should succeed even if SMTP fails
	err := writer.WriteAggregateReport(report)
	if err != nil {
		t.Fatalf("WriteAggregateReport should not fail when SMTP errors: %v", err)
	}

	// JSON should still be written
	if buf.Len() == 0 {
		t.Error("JSON output should be written even if SMTP fails")
	}

	// Kafka should still be called
	if len(mockKafka.SentReports) != 1 {
		t.Errorf("Expected 1 Kafka report, got %d", len(mockKafka.SentReports))
	}
}

func TestJSONWriter_WithKafkaError(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create mock senders - Kafka will error
	mockSMTP := &MockSMTPSender{}
	mockKafka := &MockKafkaSender{ShouldError: true}

	var buf bytes.Buffer

	writer := &JSONWriter{
		writer:      &buf,
		closer:      nil,
		smtpSender:  mockSMTP,
		kafkaSender: mockKafka,
		logger:      logger,
	}

	report := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:  "Test Org",
			ReportID: "test-123",
		},
	}

	// Write should succeed even if Kafka fails
	err := writer.WriteAggregateReport(report)
	if err != nil {
		t.Fatalf("WriteAggregateReport should not fail when Kafka errors: %v", err)
	}

	// JSON should still be written
	if buf.Len() == 0 {
		t.Error("JSON output should be written even if Kafka fails")
	}

	// SMTP should still be called
	if len(mockSMTP.SentReports) != 1 {
		t.Errorf("Expected 1 SMTP report, got %d", len(mockSMTP.SentReports))
	}
}

func TestJSONWriter_NoSenders(t *testing.T) {
	logger := zaptest.NewLogger(t)

	var buf bytes.Buffer

	// Create writer without senders
	writer := &JSONWriter{
		writer:      &buf,
		closer:      nil,
		smtpSender:  nil,
		kafkaSender: nil,
		logger:      logger,
	}

	report := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:  "Test Org",
			ReportID: "test-123",
		},
	}

	// Should work fine without senders
	err := writer.WriteAggregateReport(report)
	if err != nil {
		t.Fatalf("WriteAggregateReport should work without senders: %v", err)
	}

	// JSON should be written
	if buf.Len() == 0 {
		t.Error("JSON output should be written")
	}

	output := buf.String()
	if !strings.Contains(output, "Test Org") {
		t.Error("JSON output doesn't contain expected data")
	}
}

func TestCSVWriter_WithSenders(t *testing.T) {
	logger := zaptest.NewLogger(t)

	mockSMTP := &MockSMTPSender{}
	mockKafka := &MockKafkaSender{}

	var buf bytes.Buffer

	writer := &CSVWriter{
		writer:         &buf,
		closer:         nil,
		csvWriter:      csv.NewWriter(&buf),
		smtpSender:     mockSMTP,
		kafkaSender:    mockKafka,
		logger:         logger,
		headersWritten: make(map[string]bool),
	}

	report := &parser.ForensicReport{
		FeedbackType:   "auth-failure",
		ArrivalDate:    time.Now(),
		Subject:        "Test Report",
		MessageID:      "<test@example.com>",
		ReportedDomain: "example.com",
		Source: parser.Source{
			IPAddress: "192.168.1.1",
		},
		AuthFailure: []string{"dmarc"},
	}

	// Write forensic report
	err := writer.WriteForensicReport(report)
	if err != nil {
		t.Fatalf("WriteForensicReport failed: %v", err)
	}

	// Verify CSV was written
	output := buf.String()
	if !strings.Contains(output, "feedback_type") {
		t.Error("CSV output doesn't contain headers")
	}

	if !strings.Contains(output, "auth-failure") {
		t.Error("CSV output doesn't contain data")
	}

	// Verify senders were called
	if len(mockSMTP.SentReports) != 1 {
		t.Errorf("Expected 1 SMTP report, got %d", len(mockSMTP.SentReports))
	}

	if len(mockKafka.SentReports) != 1 {
		t.Errorf("Expected 1 Kafka report, got %d", len(mockKafka.SentReports))
	}
}

func TestNewWriter_WithSenders(t *testing.T) {
	logger := zaptest.NewLogger(t)

	mockSMTP := &MockSMTPSender{}
	mockKafka := &MockKafkaSender{}

	config := Config{
		Format:      FormatJSON,
		File:        "", // Use stdout
		SMTPSender:  mockSMTP,
		KafkaSender: mockKafka,
		Logger:      logger,
	}

	writer, err := NewWriter(config)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	// Verify it's a JSON writer with the correct senders
	jsonWriter, ok := writer.(*JSONWriter)
	if !ok {
		t.Fatal("Expected JSONWriter")
	}

	if jsonWriter.smtpSender != mockSMTP {
		t.Error("SMTP sender not set correctly")
	}

	if jsonWriter.kafkaSender != mockKafka {
		t.Error("Kafka sender not set correctly")
	}
}
