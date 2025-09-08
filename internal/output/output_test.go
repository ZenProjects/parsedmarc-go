package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"parsedmarc-go/internal/parser"
)

func TestJSONWriter(t *testing.T) {
	var buf bytes.Buffer

	writer := &JSONWriter{
		writer: &buf,
		closer: nil,
	}

	// Test aggregate report
	aggregateReport := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:   "test.com",
			OrgEmail:  "test@example.com",
			ReportID:  "test-123",
			BeginDate: time.Now().Add(-24 * time.Hour),
			EndDate:   time.Now(),
		},
		PolicyPublished: parser.PolicyPublished{
			Domain: "example.com",
			P:      "none",
		},
		Records: []parser.Record{
			{
				Source: parser.Source{
					IPAddress: "192.0.2.1",
					Country:   "US",
				},
				Count: 1,
				Alignment: parser.Alignment{
					DMARC: true,
				},
			},
		},
	}

	err := writer.WriteAggregateReport(aggregateReport)
	if err != nil {
		t.Fatalf("WriteAggregateReport failed: %v", err)
	}

	// Verify JSON output
	var parsed parser.AggregateReport
	if err := json.Unmarshal(buf.Bytes()[:len(buf.Bytes())-1], &parsed); err != nil { // Remove trailing newline
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if parsed.ReportMetadata.OrgName != "test.com" {
		t.Errorf("Expected org_name 'test.com', got '%s'", parsed.ReportMetadata.OrgName)
	}

	if len(parsed.Records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(parsed.Records))
	}
}

func TestCSVWriter(t *testing.T) {
	var buf bytes.Buffer

	writer := &CSVWriter{
		writer:         &buf,
		closer:         nil,
		csvWriter:      csv.NewWriter(&buf),
		headersWritten: make(map[string]bool),
	}

	// Test aggregate report
	aggregateReport := &parser.AggregateReport{
		ReportMetadata: parser.ReportMetadata{
			OrgName:   "test.com",
			OrgEmail:  "test@example.com",
			ReportID:  "test-123",
			BeginDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		},
		PolicyPublished: parser.PolicyPublished{
			Domain: "example.com",
			P:      "none",
		},
		Records: []parser.Record{
			{
				Source: parser.Source{
					IPAddress: "192.0.2.1",
					Country:   "US",
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
				Alignment: parser.Alignment{
					DMARC: true,
				},
			},
		},
	}

	err := writer.WriteAggregateReport(aggregateReport)
	if err != nil {
		t.Fatalf("WriteAggregateReport failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header + 1 data row
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines (header + data), got %d", len(lines))
	}

	// Check header
	header := lines[0]
	if !strings.Contains(header, "report_id") || !strings.Contains(header, "org_name") {
		t.Errorf("Header missing expected fields: %s", header)
	}

	// Check data row
	dataRow := lines[1]
	if !strings.Contains(dataRow, "test-123") || !strings.Contains(dataRow, "test.com") {
		t.Errorf("Data row missing expected values: %s", dataRow)
	}
}

func TestForensicReportCSV(t *testing.T) {
	var buf bytes.Buffer

	writer := &CSVWriter{
		writer:         &buf,
		closer:         nil,
		csvWriter:      csv.NewWriter(&buf),
		headersWritten: make(map[string]bool),
	}

	forensicReport := &parser.ForensicReport{
		FeedbackType:          "auth-failure",
		ArrivalDate:           time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Subject:               "Test Report",
		MessageID:             "test@example.com",
		AuthenticationResults: "dmarc=fail",
		Source: parser.Source{
			IPAddress: "192.0.2.100",
			Country:   "US",
		},
		DeliveryResult: "delivered",
		AuthFailure:    []string{"dmarc"},
		ReportedDomain: "example.com",
	}

	err := writer.WriteForensicReport(forensicReport)
	if err != nil {
		t.Fatalf("WriteForensicReport failed: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header + 1 data row
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}

	// Check that forensic-specific fields are present
	header := lines[0]
	if !strings.Contains(header, "feedback_type") || !strings.Contains(header, "auth_failure") {
		t.Errorf("Header missing forensic fields: %s", header)
	}
}

func TestNewWriter(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "JSON to stdout",
			config: Config{
				Format: FormatJSON,
				File:   "",
			},
			expectError: false,
		},
		{
			name: "CSV to stdout",
			config: Config{
				Format: FormatCSV,
				File:   "",
			},
			expectError: false,
		},
		{
			name: "Invalid format",
			config: Config{
				Format: "invalid",
				File:   "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, err := NewWriter(tt.config)
			if (err != nil) != tt.expectError {
				t.Errorf("NewWriter() error = %v, expectError %v", err, tt.expectError)
			}

			if writer != nil {
				writer.Close()
			}
		})
	}
}

func TestDirectoryJSONWriter(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create a test logger
	logger := zap.NewNop()

	writer := &DirectoryJSONWriter{
		outputDir: tempDir,
		logger:    logger,
	}

	// Test aggregate report
	aggregateReport := &parser.AggregateReport{
		XMLSchema: "1.0",
		ReportMetadata: parser.ReportMetadata{
			OrgName:   "test.com",
			OrgEmail:  "test@example.com",
			ReportID:  "test-123",
			BeginDate: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC),
		},
		PolicyPublished: parser.PolicyPublished{
			Domain: "example.com",
			P:      "none",
		},
		Records: []parser.Record{
			{
				Source: parser.Source{
					IPAddress: "192.0.2.1",
					Country:   "US",
				},
				Count: 1,
				Alignment: parser.Alignment{
					DMARC: true,
				},
			},
		},
	}

	err := writer.WriteAggregateReport(aggregateReport)
	if err != nil {
		t.Fatalf("WriteAggregateReport failed: %v", err)
	}

	// Check that file was created
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	filename := files[0].Name()
	if !strings.HasPrefix(filename, "aggregate_") || !strings.HasSuffix(filename, ".json") {
		t.Errorf("Unexpected filename format: %s", filename)
	}

	// Verify file content
	filePath := filepath.Join(tempDir, filename)
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var parsed parser.AggregateReport
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("Failed to parse JSON content: %v", err)
	}

	if parsed.ReportMetadata.OrgName != "test.com" {
		t.Errorf("Expected org_name 'test.com', got '%s'", parsed.ReportMetadata.OrgName)
	}
}

func TestDirectoryCSVWriter(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create a test logger
	logger := zap.NewNop()

	writer := &DirectoryCSVWriter{
		outputDir: tempDir,
		logger:    logger,
	}

	// Test forensic report
	forensicReport := &parser.ForensicReport{
		FeedbackType:          "auth-failure",
		ArrivalDate:           time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Subject:               "Test Report",
		MessageID:             "test@example.com",
		AuthenticationResults: "dmarc=fail",
		Source: parser.Source{
			IPAddress: "192.0.2.100",
			Country:   "US",
		},
		DeliveryResult: "delivered",
		AuthFailure:    []string{"dmarc"},
		ReportedDomain: "example.com",
	}

	err := writer.WriteForensicReport(forensicReport)
	if err != nil {
		t.Fatalf("WriteForensicReport failed: %v", err)
	}

	// Check that file was created
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	filename := files[0].Name()
	if !strings.HasPrefix(filename, "forensic_") || !strings.HasSuffix(filename, ".csv") {
		t.Errorf("Unexpected filename format: %s", filename)
	}

	// Verify file content
	filePath := filepath.Join(tempDir, filename)
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines (header + data), got %d", len(lines))
	}

	// Check header
	header := lines[0]
	if !strings.Contains(header, "feedback_type") {
		t.Errorf("Header missing expected fields: %s", header)
	}

	// Check data row contains expected values
	dataRow := lines[1]
	if !strings.Contains(dataRow, "auth-failure") {
		t.Errorf("Data row missing expected values: %s", dataRow)
	}
}

func TestNewWriterDirectoryMode(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		format     Format
		expectType string
	}{
		{
			name:       "Directory JSON writer",
			format:     FormatJSON,
			expectType: "*output.DirectoryJSONWriter",
		},
		{
			name:       "Directory CSV writer",
			format:     FormatCSV,
			expectType: "*output.DirectoryCSVWriter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Format: tt.format,
				File:   tempDir,
				Logger: zap.NewNop(),
			}

			writer, err := NewWriter(config)
			if err != nil {
				t.Fatalf("NewWriter() error = %v", err)
			}

			if writer == nil {
				t.Fatal("Expected non-nil writer")
			}

			// Verify correct writer type was created
			switch tt.format {
			case FormatJSON:
				if _, ok := writer.(*DirectoryJSONWriter); !ok {
					t.Errorf("Expected DirectoryJSONWriter, got %T", writer)
				}
			case FormatCSV:
				if _, ok := writer.(*DirectoryCSVWriter); !ok {
					t.Errorf("Expected DirectoryCSVWriter, got %T", writer)
				}
			}

			writer.Close()
		})
	}
}

func TestFileConcatenation(t *testing.T) {
	// Create temporary file
	tempFile := filepath.Join(t.TempDir(), "test_output.json")

	// Create first writer and write first report
	config1 := Config{
		Format: FormatJSON,
		File:   tempFile,
		Logger: zap.NewNop(),
	}

	writer1, err := NewWriter(config1)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}

	report1 := &parser.AggregateReport{
		ReportMetadata: parser.ReportMetadata{
			ReportID: "report-1",
			OrgName:  "org1.com",
		},
	}

	err = writer1.WriteAggregateReport(report1)
	if err != nil {
		t.Fatalf("WriteAggregateReport failed: %v", err)
	}
	writer1.Close()

	// Create second writer for the same file (should append)
	config2 := Config{
		Format: FormatJSON,
		File:   tempFile,
		Logger: zap.NewNop(),
	}

	writer2, err := NewWriter(config2)
	if err != nil {
		t.Fatalf("NewWriter() error = %v", err)
	}

	report2 := &parser.AggregateReport{
		ReportMetadata: parser.ReportMetadata{
			ReportID: "report-2",
			OrgName:  "org2.com",
		},
	}

	err = writer2.WriteAggregateReport(report2)
	if err != nil {
		t.Fatalf("WriteAggregateReport failed: %v", err)
	}
	writer2.Close()

	// Read file content and verify both reports are present
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)

	// Should contain both report IDs
	if !strings.Contains(contentStr, "report-1") {
		t.Error("First report not found in concatenated output")
	}
	if !strings.Contains(contentStr, "report-2") {
		t.Error("Second report not found in concatenated output")
	}

	// Split by newlines and count complete JSON objects
	// Each JSON object should be on its own lines ending with }
	parts := strings.Split(contentStr, "\n}\n")
	// We expect 2 complete objects, so split should give us at least 2 parts
	// (the last part might be empty if file ends with \n}\n)
	if len(parts) < 2 {
		t.Errorf("Expected at least 2 JSON objects in concatenated output, found %d parts", len(parts))
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test stringPtrToString
	str := "test"
	if stringPtrToString(&str) != "test" {
		t.Error("stringPtrToString failed for non-nil pointer")
	}

	if stringPtrToString(nil) != "" {
		t.Error("stringPtrToString failed for nil pointer")
	}

	// Test getDKIMDomain
	dkimResults := []parser.DKIMResult{
		{Domain: "example.com", Selector: "selector1"},
	}
	if getDKIMDomain(dkimResults) != "example.com" {
		t.Error("getDKIMDomain failed")
	}

	if getDKIMDomain([]parser.DKIMResult{}) != "" {
		t.Error("getDKIMDomain should return empty string for empty slice")
	}
}
