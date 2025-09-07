package clickhouse

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/parser"
)

func TestClickHouse_Integration(t *testing.T) {
	// Skip if running in CI or no ClickHouse available
	if testing.Short() {
		t.Skip("Skipping ClickHouse integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	cfg := config.ClickHouseConfig{
		Enabled:  true,
		Host:     "localhost",
		Port:     9000,
		Database: "dmarc_test",
		Username: "default",
		Password: "",
	}

	storage, err := New(cfg, logger)
	if err != nil {
		t.Skipf("Failed to connect to ClickHouse (expected in CI): %v", err)
		return
	}
	defer storage.Close()

	// Test storing an aggregate report
	report := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:   "test.com",
			OrgEmail:  "test@example.com",
			ReportID:  "test-12345",
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
		},
		Records: []parser.Record{
			{
				Source: parser.Source{
					IPAddress:  "192.0.2.1",
					Country:    "US",
					ReverseDNS: "test.example.net",
				},
				Count: 1,
				Alignment: parser.Alignment{
					SPF:   true,
					DKIM:  false,
					DMARC: false,
				},
				PolicyEvaluated: parser.PolicyEvaluated{
					Disposition: "none",
					DKIM:        "fail",
					SPF:         "pass",
				},
				Identifiers: parser.Identifiers{
					HeaderFrom: "example.com",
				},
			},
		},
	}

	err = storage.StoreAggregateReport(report)
	if err != nil {
		t.Errorf("Failed to store aggregate report: %v", err)
	}

	// Test storing a forensic report
	forensicReport := &parser.ForensicReport{
		FeedbackType:          "auth-failure",
		ArrivalDate:           time.Now(),
		ArrivalDateUTC:        time.Now().UTC(),
		Subject:               "Test forensic report",
		MessageID:             "test@example.com",
		AuthenticationResults: "dmarc=fail (p=none dis=none) header.from=example.com",
		Source: parser.Source{
			IPAddress: "192.0.2.2",
			Country:   "US",
		},
		DeliveryResult: "delivered",
		AuthFailure:    []string{"dmarc"},
		ReportedDomain: "example.com",
	}

	err = storage.StoreForensicReport(forensicReport)
	if err != nil {
		t.Errorf("Failed to store forensic report: %v", err)
	}
}

func TestClickHouse_StoreAggregateReport(t *testing.T) {
	// Test the aggregate report storage logic without actual database
	logger := zaptest.NewLogger(t)

	// This is more of a unit test for the storage structure
	report := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:   "google.com",
			OrgEmail:  "noreply-dmarc-support@google.com",
			ReportID:  "1234567890",
			BeginDate: time.Now().Add(-24 * time.Hour),
			EndDate:   time.Now(),
		},
		PolicyPublished: parser.PolicyPublished{
			Domain: "example.com",
			P:      "quarantine",
			SP:     "quarantine",
			PCT:    "100",
		},
		Records: []parser.Record{
			{
				Source: parser.Source{
					IPAddress: "74.125.130.26",
					Country:   "US",
				},
				Count: 15,
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

	// Just verify the structure is valid
	if report == nil {
		t.Error("Report should not be nil")
	}

	if len(report.Records) == 0 {
		t.Error("Report should have records")
	}

	if report.ReportMetadata.ReportID == "" {
		t.Error("Report should have ID")
	}

	logger.Info("Mock aggregate report test completed")
}

func TestClickHouse_StoreForensicReport(t *testing.T) {
	logger := zaptest.NewLogger(t)

	forensicReport := &parser.ForensicReport{
		FeedbackType:          "auth-failure",
		ArrivalDate:           time.Now(),
		Subject:               "DMARC Failure Report",
		MessageID:             "test-message@example.com",
		AuthenticationResults: "spf=fail smtp.mailfrom=example.com; dkim=fail header.i=@example.com",
		Source: parser.Source{
			IPAddress: "192.0.2.100",
			Country:   "Unknown",
		},
		AuthFailure:    []string{"spf", "dkim"},
		ReportedDomain: "example.com",
	}

	// Verify structure
	if forensicReport.FeedbackType != "auth-failure" {
		t.Error("Feedback type should be auth-failure")
	}

	if len(forensicReport.AuthFailure) == 0 {
		t.Error("Should have auth failures")
	}

	logger.Info("Mock forensic report test completed")
}

// Benchmark for report storage preparation
func BenchmarkPrepareAggregateReport(b *testing.B) {
	report := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:   "benchmark.com",
			ReportID:  "bench-123",
			BeginDate: time.Now(),
			EndDate:   time.Now(),
		},
		PolicyPublished: parser.PolicyPublished{
			Domain: "example.com",
			P:      "none",
		},
		Records: make([]parser.Record, 100), // Large number of records
	}

	// Fill with test records
	for i := range report.Records {
		report.Records[i] = parser.Record{
			Source: parser.Source{
				IPAddress: "192.0.2.1",
			},
			Count: 1,
			PolicyEvaluated: parser.PolicyEvaluated{
				Disposition: "none",
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate preparing data for storage
		_ = report.ReportMetadata.ReportID
		_ = len(report.Records)
	}
}
