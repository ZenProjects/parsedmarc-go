package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/http"
	"parsedmarc-go/internal/imap"
	"parsedmarc-go/internal/kafka"
	"parsedmarc-go/internal/parser"
	"parsedmarc-go/internal/smtp"
	"parsedmarc-go/internal/storage/clickhouse"
)

// TestConfig holds configuration for integration tests
type TestConfig struct {
	ClickHouse config.ClickHouseConfig
	Kafka      config.KafkaConfig
	IMAP       config.IMAPConfig
	SMTP       config.SMTPConfig
	HTTP       config.HTTPConfig
}

// NewTestConfig creates test configuration with default values
func NewTestConfig() *TestConfig {
	return &TestConfig{
		ClickHouse: config.ClickHouseConfig{
			Enabled:  true,
			Host:     "localhost",
			Port:     9000,
			Database: "parsedmarc_test",
			Username: "parsedmarc",
			Password: "test123",
		},
		Kafka: config.KafkaConfig{
			Enabled: true,
			Brokers: []string{"localhost:9092"},
			Topic:   "parsedmarc-reports",
		},
		IMAP: config.IMAPConfig{
			Enabled:       true,
			Host:          "localhost",
			Port:          143,
			Username:      "testuser@test.local",
			Password:      "testpass",
			Folder:        "INBOX",
			CheckInterval: 30,
		},
		SMTP: config.SMTPConfig{
			Enabled:  true,
			Host:     "localhost",
			Port:     1025,
			Username: "",
			Password: "",
			From:     "parsedmarc@test.local",
			To:       []string{"admin@test.local"},
		},
		HTTP: config.HTTPConfig{
			Enabled: true,
			Port:    8080,
		},
	}
}

// TestIntegrationSuite tests all services integration
func TestIntegrationSuite(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Check if services are available
	if !servicesAvailable(t) {
		t.Skip("Required services not available - run 'docker-compose -f docker-compose.test.yml up -d' first")
	}

	logger := zaptest.NewLogger(t)
	testConfig := NewTestConfig()

	t.Run("ClickHouse", func(t *testing.T) {
		testClickHouseIntegration(t, testConfig.ClickHouse, logger)
	})

	t.Run("Kafka", func(t *testing.T) {
		testKafkaIntegration(t, testConfig.Kafka, logger)
	})

	t.Run("IMAP", func(t *testing.T) {
		testIMAPIntegration(t, testConfig.IMAP, logger)
	})

	t.Run("SMTP", func(t *testing.T) {
		testSMTPIntegration(t, testConfig.SMTP, logger)
	})

	t.Run("HTTP", func(t *testing.T) {
		testHTTPIntegration(t, testConfig.HTTP, logger)
	})

	t.Run("EndToEnd", func(t *testing.T) {
		testEndToEndIntegration(t, testConfig, logger)
	})
}

// servicesAvailable checks if all required services are running
func servicesAvailable(t *testing.T) bool {
	// Simple check - could be more sophisticated
	return true // For now, assume services are running
}

// testClickHouseIntegration tests ClickHouse integration
func testClickHouseIntegration(t *testing.T, cfg config.ClickHouseConfig, logger *zap.Logger) {
	// Wait for ClickHouse to be ready
	time.Sleep(5 * time.Second)

	storage, err := clickhouse.New(cfg, logger)
	require.NoError(t, err, "Failed to create ClickHouse storage")
	defer storage.Close()

	// Test storing an aggregate report
	report := createTestAggregateReport()
	err = storage.StoreAggregateReport(report)
	assert.NoError(t, err, "Failed to store aggregate report")

	// Test storing a forensic report
	forensicReport := createTestForensicReport()
	err = storage.StoreForensicReport(forensicReport)
	assert.NoError(t, err, "Failed to store forensic report")
}

// testKafkaIntegration tests Kafka integration
func testKafkaIntegration(t *testing.T, cfg config.KafkaConfig, logger *zap.Logger) {
	kafkaClient := kafka.New(&cfg, logger)

	// Test sending a message
	testMessage := `{"test": "message", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`
	err := kafkaClient.SendMessage([]byte(testMessage))
	assert.NoError(t, err, "Failed to send Kafka message")
}

// testIMAPIntegration tests IMAP integration
func testIMAPIntegration(t *testing.T, cfg config.IMAPConfig, logger *zap.Logger) {
	// Create parser for IMAP client
	parser := parser.New(config.ParserConfig{}, nil, logger)
	imapClient := imap.New(cfg, parser, logger)

	// Test connection
	err := imapClient.Connect()
	if err != nil {
		t.Skipf("IMAP connection failed (expected in test environment): %v", err)
		return
	}
	defer imapClient.Disconnect()

	// Test processing messages (should not fail)
	err = imapClient.ProcessMessages()
	assert.NoError(t, err, "Failed to process IMAP messages")
}

// testSMTPIntegration tests SMTP integration
func testSMTPIntegration(t *testing.T, cfg config.SMTPConfig, logger *zap.Logger) {
	smtpClient := smtp.New(&cfg, logger)

	// Test sending email
	subject := "Test Email - " + time.Now().Format(time.RFC3339)
	body := "This is a test email from integration tests"

	err := smtpClient.SendEmail(subject, body, []byte("attachment data"))
	assert.NoError(t, err, "Failed to send SMTP email")
}

// testHTTPIntegration tests HTTP server integration
func testHTTPIntegration(t *testing.T, cfg config.HTTPConfig, logger *zap.Logger) {
	// Create parser for HTTP server
	parser := parser.New(config.ParserConfig{}, nil, logger)
	httpServer := http.New(cfg, parser, logger)

	// Start server in goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := httpServer.Start()
		if err != nil {
			logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Wait for server to start
	time.Sleep(2 * time.Second)

	// Test HTTP endpoints
	// (Would need HTTP client to test endpoints)

	// Stop server
	err := httpServer.Stop(ctx)
	assert.NoError(t, err, "Failed to stop HTTP server")
}

// testEndToEndIntegration tests full pipeline
func testEndToEndIntegration(t *testing.T, cfg *TestConfig, logger *zap.Logger) {
	// Create storage
	storage, err := clickhouse.New(cfg.ClickHouse, logger)
	require.NoError(t, err)
	defer storage.Close()

	// Create parser with storage
	parser := parser.New(config.ParserConfig{}, storage, logger)

	// Create Kafka client
	kafkaClient := kafka.New(&cfg.Kafka, logger)

	// Test full pipeline: Parse -> Store -> Send to Kafka
	report := createTestAggregateReport()

	// This would be a more complex test simulating the full workflow
	err = storage.StoreAggregateReport(report)
	require.NoError(t, err)

	// Send notification via Kafka
	message := fmt.Sprintf(`{"type": "aggregate_report", "org": "%s", "report_id": "%s"}`,
		report.ReportMetadata.OrgName, report.ReportMetadata.ReportID)
	err = kafkaClient.SendMessage([]byte(message))
	assert.NoError(t, err)
}

// Helper functions to create test data
func createTestAggregateReport() *parser.AggregateReport {
	return &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:   "test.integration",
			OrgEmail:  "test@integration.local",
			ReportID:  fmt.Sprintf("test-integration-%d", time.Now().Unix()),
			BeginDate: time.Now().Add(-24 * time.Hour),
			EndDate:   time.Now(),
		},
		PolicyPublished: parser.PolicyPublished{
			Domain: "integration.test",
			P:      "none",
			SP:     "none",
			PCT:    "100",
		},
		Records: []parser.Record{
			{
				Source: parser.Source{
					IPAddress: "192.0.2.100",
					Country:   "Test",
				},
				Count: 1,
				PolicyEvaluated: parser.PolicyEvaluated{
					Disposition: "none",
					DKIM:        "pass",
					SPF:         "pass",
				},
				Identifiers: parser.Identifiers{
					HeaderFrom: "integration.test",
				},
			},
		},
	}
}

func createTestForensicReport() *parser.ForensicReport {
	return &parser.ForensicReport{
		FeedbackType: "auth-failure",
		ArrivalDate:  time.Now(),
		Subject:      "Integration Test Forensic Report",
		MessageID:    fmt.Sprintf("test-forensic-%d@integration.test", time.Now().Unix()),
		Source: parser.Source{
			IPAddress: "192.0.2.200",
			Country:   "Test",
		},
		AuthFailure:    []string{"spf", "dkim"},
		ReportedDomain: "integration.test",
	}
}
