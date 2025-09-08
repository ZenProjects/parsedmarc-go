package parser

import (
	"encoding/json"
	"errors"

	"parsedmarc-go/internal/output"
)

var ErrUnsupportedFormat = errors.New("unsupported report format")

// ParseWithOutput parses data and writes to output writer instead of storage
func (p *Parser) ParseWithOutput(data []byte, outputWriter output.Writer) error {
	// Try to detect and parse the report type
	reportType := p.detectReportType(data)

	switch reportType {
	case "aggregate":
		return p.parseAggregateWithOutput(data, outputWriter)
	case "forensic":
		return p.parseForensicWithOutput(data, outputWriter)
	case "smtp_tls":
		return p.parseSMTPTLSWithOutput(data, outputWriter)
	default:
		// Try all parsers
		if err := p.parseAggregateWithOutput(data, outputWriter); err == nil {
			return nil
		}
		if err := p.parseForensicWithOutput(data, outputWriter); err == nil {
			return nil
		}
		if err := p.parseSMTPTLSWithOutput(data, outputWriter); err == nil {
			return nil
		}
		return ErrUnsupportedFormat
	}
}

func (p *Parser) parseAggregateWithOutput(data []byte, outputWriter output.Writer) error {
	// Parse aggregate report
	report, err := p.parseAggregateXML(data)
	if err != nil {
		return err
	}

	// Write to output instead of storage
	return outputWriter.WriteAggregateReport(report)
}

func (p *Parser) parseForensicWithOutput(data []byte, outputWriter output.Writer) error {
	// Parse forensic report
	report, err := p.parseForensicEmail(data)
	if err != nil {
		return err
	}

	// Write to output instead of storage
	return outputWriter.WriteForensicReport(report)
}

func (p *Parser) parseSMTPTLSWithOutput(data []byte, outputWriter output.Writer) error {
	// Parse SMTP TLS report
	var report SMTPTLSReport
	if err := p.parseSMTPTLSJSON(data, &report); err != nil {
		return err
	}

	// Write to output instead of storage
	return outputWriter.WriteSMTPTLSReport(&report)
}

func (p *Parser) detectReportType(data []byte) string {
	dataStr := string(data)

	// Check for forensic report markers
	if containsAny(dataStr, []string{"feedback-type:", "auth-failure:"}) {
		return "forensic"
	}

	// Check for SMTP TLS report markers
	if containsAny(dataStr, []string{"organization-name", "\"organization_name\""}) {
		return "smtp_tls"
	}

	// Check for aggregate report markers
	if containsAny(dataStr, []string{"<feedback", "<report_metadata", "org_name"}) {
		return "aggregate"
	}

	return "unknown"
}

func (p *Parser) parseSMTPTLSJSON(data []byte, report *SMTPTLSReport) error {
	return json.Unmarshal(data, report)
}

func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
