package parser

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zaptest"
	"parsedmarc-go/internal/config"
)

func TestParser_ParseAggregateReports(t *testing.T) {
	// Initialize parser with test logger
	logger := zaptest.NewLogger(t)
	cfg := config.ParserConfig{
		Offline: true, // Use offline mode for tests
	}
	parser := New(cfg, nil, logger)

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
	logger := zaptest.NewLogger(t)
	cfg := config.ParserConfig{
		Offline: true,
	}
	parser := New(cfg, nil, logger)

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
	logger := zaptest.NewLogger(t)
	cfg := config.ParserConfig{
		Offline: true,
	}
	parser := New(cfg, nil, logger)

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
	logger := zaptest.NewLogger(t)
	cfg := config.ParserConfig{
		Offline: true,
	}
	parser := New(cfg, nil, logger)

	tests := []struct {
		name     string
		path     string
		filename string
		wantErr  bool
	}{
		{
			name:     "Invalid aggregate report",
			path:     "../../samples/aggregate_invalid",
			filename: "invalid_xml.xml",
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
	logger := zaptest.NewLogger(t)
	cfg := config.ParserConfig{
		Offline: true,
	}
	parser := New(cfg, nil, logger)

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

// Benchmark tests
func BenchmarkParser_ParseAggregateReport(b *testing.B) {
	logger := zaptest.NewLogger(b)
	cfg := config.ParserConfig{
		Offline: true,
	}
	parser := New(cfg, nil, logger)

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
	parser := New(cfg, nil, logger)

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
