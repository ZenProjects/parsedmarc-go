package parser

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zaptest"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/metrics"
)

// createTestParser creates a parser for testing without reinitializing metrics
func createTestParser(t *testing.T) *Parser {
	logger := zaptest.NewLogger(t)
	cfg := config.ParserConfig{
		Offline: true, // Use offline mode for tests
	}

	// Create parser with nil metrics to avoid Prometheus registration conflicts
	parser := &Parser{
		config:  cfg,
		storage: nil,
		logger:  logger,
		metrics: nil, // Use nil metrics for tests
	}

	return parser
}

func TestParser_ParseAggregateReports(t *testing.T) {
	parser := createTestParser(t)

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "Basic aggregate report",
			filename: "!example.com!1538204542!1538463818.xml",
			wantErr:  false,
		},
		{
			name:     "Large aggregate report",
			filename: "!large-example.com!1711897200!1711983600.xml",
			wantErr:  false,
		},
		{
			name:     "Fastmail GZIP report",
			filename: "fastmail.com!example.com!1516060800!1516147199!102675056.xml.gz",
			wantErr:  false,
		},
		{
			name:     "ZIP compressed report",
			filename: "estadocuenta1.infonacot.gob.mx!example.com!1536853302!1536939702!2940.xml.zip",
			wantErr:  false,
		},
		{
			name:     "Empty reason report",
			filename: "empty_reason.xml",
			wantErr:  false,
		},
		{
			name:     "Invalid XML",
			filename: "invalid_xml.xml",
			wantErr:  true,
		},
		{
			name:     "Invalid UTF-8",
			filename: "invalid_utf_8.xml",
			wantErr:  true,
		},
		{
			name:     "Empty file",
			filename: "../empty.xml",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samplePath := filepath.Join("../../samples/aggregate", tt.filename)

			data, err := os.ReadFile(samplePath)
			if err != nil {
				t.Fatalf("Failed to read sample file %s: %v", samplePath, err)
			}

			err = parser.ParseData(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parser.ParseData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParser_ParseForensicReports(t *testing.T) {
	parser := createTestParser(t)

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "LinkedIn forensic report",
			filename: "dmarc_ruf_report_linkedin.eml",
			wantErr:  false,
		},
		{
			name:     "LinkedIn CRLF forensic report",
			filename: "dmarc_ruf_report_linkedin.crlf.eml",
			wantErr:  false,
		},
		{
			name:     "Domain.de forensic report",
			filename: "DMARC Failure Report for domain.de (mail-from=sharepoint@domain.de, ip=10.10.10.10).eml",
			wantErr:  false,
		},
		{
			name:     "Netease forensic report",
			filename: "[Netease DMARC Failure Report] Rent Reminder.eml",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samplePath := filepath.Join("../../samples/forensic", tt.filename)

			data, err := os.ReadFile(samplePath)
			if err != nil {
				t.Fatalf("Failed to read sample file %s: %v", samplePath, err)
			}

			err = parser.ParseData(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parser.ParseData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParser_ParseSMTPTLSReports(t *testing.T) {
	parser := createTestParser(t)

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "RFC 8460 JSON report",
			filename: "rfc8460.json",
			wantErr:  false,
		},
		{
			name:     "Mail.ru JSON report",
			filename: "mail.ru.json",
			wantErr:  false,
		},
		{
			name:     "SMTP TLS JSON report",
			filename: "smtp_tls.json",
			wantErr:  false,
		},
		{
			name:     "Google SMTP TLS email report",
			filename: "google.com_smtp_tls_report.eml",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samplePath := filepath.Join("../../samples/smtp_tls", tt.filename)

			data, err := os.ReadFile(samplePath)
			if err != nil {
				t.Fatalf("Failed to read sample file %s: %v", samplePath, err)
			}

			err = parser.ParseData(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parser.ParseData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParser_ParseInvalidReports(t *testing.T) {
	parser := createTestParser(t)

	tests := []struct {
		name     string
		path     string
		filename string
		wantErr  bool
	}{
		{
			name:     "Invalid aggregate report",
			path:     "../../samples/aggregate_invalid",
			filename: "report_with_upper_cased_pass.xml",
			wantErr:  true,
		},
		{
			name:     "Empty XML",
			path:     "../../samples",
			filename: "empty.xml",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samplePath := filepath.Join(tt.path, tt.filename)

			data, err := os.ReadFile(samplePath)
			if err != nil {
				// If file doesn't exist, create empty data for empty.xml test
				if tt.filename == "empty.xml" {
					data = []byte("")
				} else {
					t.Fatalf("Failed to read sample file %s: %v", samplePath, err)
				}
			}

			err = parser.ParseData(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parser.ParseData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParser_ParseCompressedFiles(t *testing.T) {
	parser := createTestParser(t)

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "GZIP compressed XML",
			filename: "fastmail.com!example.com!1516060800!1516147199!102675056.xml.gz",
			wantErr:  false,
		},
		{
			name:     "ZIP compressed XML",
			filename: "estadocuenta1.infonacot.gob.mx!example.com!1536853302!1536939702!2940.xml.zip",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			samplePath := filepath.Join("../../samples/aggregate", tt.filename)

			data, err := os.ReadFile(samplePath)
			if err != nil {
				t.Fatalf("Failed to read sample file %s: %v", samplePath, err)
			}

			err = parser.ParseData(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parser.ParseData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParser_ParseAggregateFromBytes(t *testing.T) {
	parser := createTestParser(t)

	// Test with a simple aggregate report XML
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<feedback>
  <version>1.0</version>
  <report_metadata>
    <org_name>Example Corp</org_name>
    <org_email>postmaster@example.com</org_email>
    <report_id>test123</report_id>
    <date_range>
      <begin>1538204542</begin>
      <end>1538290942</end>
    </date_range>
  </report_metadata>
  <policy_published>
    <domain>example.com</domain>
    <adkim>r</adkim>
    <aspf>r</aspf>
    <p>none</p>
    <sp>none</sp>
    <pct>100</pct>
  </policy_published>
  <record>
    <row>
      <source_ip>192.168.1.1</source_ip>
      <count>1</count>
      <policy_evaluated>
        <disposition>none</disposition>
        <dkim>pass</dkim>
        <spf>pass</spf>
      </policy_evaluated>
    </row>
    <identifiers>
      <header_from>example.com</header_from>
    </identifiers>
    <auth_results>
      <spf>
        <domain>example.com</domain>
        <result>pass</result>
      </spf>
    </auth_results>
  </record>
</feedback>`

	report, err := parser.ParseAggregateFromBytes([]byte(xmlData))
	if err != nil {
		t.Fatalf("ParseAggregateFromBytes() error = %v", err)
	}

	if report == nil {
		t.Fatal("ParseAggregateFromBytes() returned nil report")
	}

	// Verify basic report fields
	if report.ReportMetadata.OrgName != "Example Corp" {
		t.Errorf("Expected org_name 'Example Corp', got '%s'", report.ReportMetadata.OrgName)
	}

	if report.ReportMetadata.ReportID != "test123" {
		t.Errorf("Expected report_id 'test123', got '%s'", report.ReportMetadata.ReportID)
	}

	if report.PolicyPublished.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", report.PolicyPublished.Domain)
	}

	if len(report.Records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(report.Records))
	}

	if len(report.Records) > 0 {
		record := report.Records[0]
		if record.Source.IPAddress != "192.168.1.1" {
			t.Errorf("Expected source IP '192.168.1.1', got '%s'", record.Source.IPAddress)
		}
		if record.Count != 1 {
			t.Errorf("Expected count 1, got %d", record.Count)
		}
	}
}

// Benchmark tests
func BenchmarkParser_ParseAggregateReport(b *testing.B) {
	logger := zaptest.NewLogger(b)
	cfg := config.ParserConfig{
		Offline: true,
	}

	// Create parser with empty metrics to avoid registration conflicts
	parser := &Parser{
		config:  cfg,
		storage: nil,
		logger:  logger,
		metrics: &metrics.ParserMetrics{},
	}

	samplePath := filepath.Join("../../samples/aggregate", "!example.com!1538204542!1538463818.xml")
	data, err := os.ReadFile(samplePath)
	if err != nil {
		b.Fatalf("Failed to read sample file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := parser.ParseData(data)
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

func BenchmarkParser_ParseLargeAggregateReport(b *testing.B) {
	logger := zaptest.NewLogger(b)
	cfg := config.ParserConfig{
		Offline: true,
	}

	// Create parser with empty metrics to avoid registration conflicts
	parser := &Parser{
		config:  cfg,
		storage: nil,
		logger:  logger,
		metrics: &metrics.ParserMetrics{},
	}

	samplePath := filepath.Join("../../samples/aggregate", "!large-example.com!1711897200!1711983600.xml")
	data, err := os.ReadFile(samplePath)
	if err != nil {
		b.Skipf("Large sample file not found: %v", err)
		return
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := parser.ParseData(data)
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}
