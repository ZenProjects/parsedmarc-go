package parser

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/metrics"
	"parsedmarc-go/internal/utils"
)

// Parser handles DMARC report parsing
type Parser struct {
	config  config.ParserConfig
	storage Storage
	logger  *zap.Logger
	metrics *metrics.ParserMetrics
}

// New creates a new parser instance
func New(config config.ParserConfig, storage Storage, logger *zap.Logger) *Parser {
	return &Parser{
		config:  config,
		storage: storage,
		logger:  logger,
		metrics: metrics.NewParserMetrics(),
	}
}

// ParseFile parses a single file or directory of DMARC reports
func (p *Parser) ParseFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return p.parseDirectory(path)
	}

	return p.parseSingleFile(path)
}

// ParseData parses DMARC report data from byte slice
func (p *Parser) ParseData(data []byte) error {
	return p.parseDataWithSource(data, "http")
}

// parseDataWithSource parses DMARC report data with source tracking
func (p *Parser) parseDataWithSource(data []byte, source string) error {
	start := time.Now()
	size := len(data)

	p.logger.Debug("Parsing data", zap.Int("size", size), zap.String("source", source))

	// Extract content if compressed
	extractedData, err := p.extractReportData(data)
	if err != nil {
		duration := time.Since(start).Seconds()
		if p.metrics != nil {
			p.metrics.RecordParseFailure("unknown", source, "extraction_failed", duration, size)
		}
		return fmt.Errorf("failed to extract report data: %w", err)
	}

	// Try to parse as different report types and collect errors
	var parseErrors []string

	if err := p.parseAsAggregateReportWithMetrics(extractedData, source, start, size); err == nil {
		return nil
	} else {
		parseErrors = append(parseErrors, fmt.Sprintf("aggregate: %v", err))
	}

	if err := p.parseAsForensicReportWithMetrics(extractedData, source, start, size); err == nil {
		return nil
	} else {
		parseErrors = append(parseErrors, fmt.Sprintf("forensic: %v", err))
	}

	if err := p.parseAsSMTPTLSReportWithMetrics(extractedData, source, start, size); err == nil {
		return nil
	} else {
		parseErrors = append(parseErrors, fmt.Sprintf("smtp_tls: %v", err))
	}

	duration := time.Since(start).Seconds()
	if p.metrics != nil {
		p.metrics.RecordParseFailure("unknown", source, "unknown_format", duration, size)
	}

	// Log detailed parsing errors
	p.logger.Debug("Detailed parsing errors",
		zap.Strings("errors", parseErrors),
		zap.String("source", source),
	)

	return fmt.Errorf("unable to parse data as any known DMARC report type. Details: %s",
		strings.Join(parseErrors, "; "))
}

// parseDirectory recursively parses all files in a directory
func (p *Parser) parseDirectory(dirPath string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			if err := p.parseSingleFile(path); err != nil {
				p.logger.Error("Failed to parse file",
					zap.String("file", path),
					zap.Error(err),
				)
			}
		}
		return nil
	})
}

// parseSingleFile parses a single DMARC report file
func (p *Parser) parseSingleFile(filePath string) error {
	p.logger.Info("Parsing file", zap.String("file", filePath))

	data, err := p.extractReport(filePath)
	if err != nil {
		return fmt.Errorf("failed to extract report: %w", err)
	}

	// Try to parse as different report types
	if err := p.parseAsAggregateReport(data); err == nil {
		return nil
	}

	if err := p.parseAsForensicReport(data); err == nil {
		return nil
	}

	if err := p.parseAsSMTPTLSReport(data); err == nil {
		return nil
	}

	return fmt.Errorf("unable to parse file as any known DMARC report type")
}

// extractReport extracts content from zip, gzip, or plain text files
func (p *Parser) extractReport(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read first few bytes to detect file type
	header := make([]byte, 10)
	n, err := file.Read(header)
	if err != nil && err != io.EOF {
		return nil, err
	}
	header = header[:n]

	// Reset file position
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset file position: %w", err)
	}

	// Check for ZIP file magic
	if len(header) >= 4 && string(header[:4]) == "PK\x03\x04" {
		return p.extractFromZip(file)
	}

	// Check for GZIP file magic
	if len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		return p.extractFromGzip(file)
	}

	// Check for XML or JSON
	if strings.HasPrefix(string(header), "<?xml") ||
		strings.HasPrefix(string(header), "{") {
		return io.ReadAll(file)
	}

	return io.ReadAll(file)
}

// extractReportData extracts content from compressed data
func (p *Parser) extractReportData(data []byte) ([]byte, error) {
	// Check file type by magic bytes
	if len(data) < 4 {
		return data, nil
	}

	// Check for ZIP file magic
	if string(data[:4]) == "PK\x03\x04" {
		return p.extractFromZipData(data)
	}

	// Check for GZIP file magic
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		return p.extractFromGzipData(data)
	}

	// Return as-is if not compressed
	return data, nil
}

// extractFromZipData extracts from ZIP data
func (p *Parser) extractFromZipData(data []byte) ([]byte, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	if len(zipReader.File) == 0 {
		return nil, fmt.Errorf("zip contains no files")
	}

	// Extract first file
	file := zipReader.File[0]
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// extractFromGzipData extracts from GZIP data
func (p *Parser) extractFromGzipData(data []byte) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	return io.ReadAll(gzReader)
}

// extractFromZip extracts content from ZIP file
func (p *Parser) extractFromZip(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	if len(zipReader.File) == 0 {
		return nil, fmt.Errorf("zip file contains no files")
	}

	// Extract first file
	file := zipReader.File[0]
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// extractFromGzip extracts content from GZIP file
func (p *Parser) extractFromGzip(reader io.Reader) ([]byte, error) {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	return io.ReadAll(gzReader)
}

// parseAsAggregateReport tries to parse data as aggregate DMARC report
func (p *Parser) parseAsAggregateReport(data []byte) error {
	report, err := p.parseAggregateXML(data)
	if err != nil {
		return err
	}

	if p.storage != nil {
		if err := p.storage.StoreAggregateReport(report); err != nil {
			return fmt.Errorf("failed to store aggregate report: %w", err)
		}
	}

	p.logger.Info("Successfully parsed aggregate report",
		zap.String("org", report.ReportMetadata.OrgName),
		zap.String("report_id", report.ReportMetadata.ReportID),
		zap.Int("records", len(report.Records)),
	)

	return nil
}

// parseAsForensicReport tries to parse data as forensic DMARC report
func (p *Parser) parseAsForensicReport(data []byte) error {
	report, err := p.parseForensicEmail(data)
	if err != nil {
		return err
	}

	if p.storage != nil {
		if err := p.storage.StoreForensicReport(report); err != nil {
			return fmt.Errorf("failed to store forensic report: %w", err)
		}
	}

	p.logger.Info("Successfully parsed forensic report",
		zap.String("subject", report.Subject),
		zap.String("source_ip", report.Source.IPAddress),
		zap.String("reported_domain", report.ReportedDomain),
	)

	return nil
}

// parseAsSMTPTLSReport tries to parse data as SMTP TLS report
func (p *Parser) parseAsSMTPTLSReport(data []byte) error {
	// First try to parse as direct JSON
	var report SMTPTLSReport
	if err := p.parseJSONWithLineInfo(data, &report); err == nil {
		// Direct JSON parsing succeeded
		return p.processSMTPTLSReport(&report)
	}

	// Try to parse as email containing SMTP TLS report
	if reportFromEmail, err := p.parseSMTPTLSEmail(data); err == nil {
		return p.processSMTPTLSReport(reportFromEmail)
	}

	return fmt.Errorf("failed to parse SMTP TLS report")
}

// processSMTPTLSReport handles storage and logging for SMTP TLS reports
func (p *Parser) processSMTPTLSReport(report *SMTPTLSReport) error {
	if p.storage != nil {
		if err := p.storage.StoreSMTPTLSReport(report); err != nil {
			return fmt.Errorf("failed to store SMTP TLS report: %w", err)
		}
	}

	p.logger.Info("Successfully parsed SMTP TLS report",
		zap.String("org", report.OrganizationName),
		zap.String("report_id", report.ReportID),
		zap.Int("policies", len(report.Policies)),
	)

	return nil
}

// parseSMTPTLSEmail parses an SMTP TLS report from email data
func (p *Parser) parseSMTPTLSEmail(emailData []byte) (*SMTPTLSReport, error) {
	// Parse the email message
	emailStr := string(emailData)

	// Split email into headers and body parts
	parts := strings.Split(emailStr, "\r\n\r\n")
	if len(parts) < 2 {
		parts = strings.Split(emailStr, "\n\n")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid email format")
		}
	}

	// Extract SMTP TLS report from MIME parts
	jsonContent := p.extractSMTPTLSFromMIME(emailStr)
	if jsonContent == "" {
		return nil, fmt.Errorf("no SMTP TLS report found")
	}

	// Parse the JSON content
	var report SMTPTLSReport
	if err := p.parseJSONWithLineInfo([]byte(jsonContent), &report); err != nil {
		return nil, fmt.Errorf("failed to parse SMTP TLS JSON: %w", err)
	}

	return &report, nil
}

// extractSMTPTLSFromMIME extracts SMTP TLS JSON from MIME multipart message
func (p *Parser) extractSMTPTLSFromMIME(body string) string {
	// First try to parse as multipart MIME message
	content := p.extractSMTPTLSFromMIMEParts(body)
	if content != "" {
		return content
	}

	// Fall back to looking for direct JSON in the body (for non-MIME messages)
	if strings.Contains(body, `"organization-name"`) || strings.Contains(body, `"report-id"`) {
		// Extract JSON from body (skip headers)
		lines := strings.Split(body, "\n")
		jsonStart := -1
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" && jsonStart == -1 {
				// Found end of headers, next non-empty line should be JSON
				continue
			}
			if line != "" && (strings.HasPrefix(line, "{") || strings.Contains(line, `"organization-name"`)) {
				jsonStart = i
				break
			}
		}
		if jsonStart >= 0 {
			return strings.Join(lines[jsonStart:], "\n")
		}
	}

	return ""
}

// extractSMTPTLSFromMIMEParts extracts SMTP TLS content from MIME multipart message
func (p *Parser) extractSMTPTLSFromMIMEParts(body string) string {
	// Look for Content-Type header with boundary
	lines := strings.Split(body, "\n")
	var contentType string
	bodyStartIdx := 0

	// Find Content-Type header and body start, handling multiline headers
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "content-type:") {
			// Start building content type, may span multiple lines
			contentType = line
			// Look ahead for continuation lines (start with whitespace)
			for j := i + 1; j < len(lines); j++ {
				nextLine := lines[j]
				if strings.HasPrefix(nextLine, " ") || strings.HasPrefix(nextLine, "\t") {
					contentType += " " + strings.TrimSpace(nextLine)
				} else if strings.TrimSpace(nextLine) == "" {
					// Empty line after headers marks start of body
					bodyStartIdx = j + 1
					break
				} else {
					// Non-continuation line, this header is complete
					break
				}
			}
			break
		} else if line == "" {
			// Empty line after headers marks start of body
			bodyStartIdx = i + 1
			break
		}
	}

	// Extract boundary from content type
	var boundary string
	if strings.Contains(strings.ToLower(contentType), "boundary=") {
		parts := strings.Split(contentType, "boundary=")
		if len(parts) >= 2 {
			boundaryPart := strings.Trim(parts[1], `"`)
			// Remove any trailing content after the boundary value
			if idx := strings.Index(boundaryPart, ";"); idx > 0 {
				boundaryPart = boundaryPart[:idx]
			}
			if idx := strings.Index(boundaryPart, " "); idx > 0 {
				boundaryPart = boundaryPart[:idx]
			}
			boundary = strings.Trim(boundaryPart, `"`)
		}
	}

	if boundary == "" || !strings.Contains(strings.ToLower(contentType), "multipart") {
		return ""
	}

	// Reconstruct the body from bodyStartIdx
	if bodyStartIdx >= len(lines) {
		return ""
	}
	bodyLines := lines[bodyStartIdx:]
	mimeBody := strings.Join(bodyLines, "\n")

	// Extract media type value from header (remove "Content-type: " prefix)
	mediaTypeValue := contentType
	if colonIdx := strings.Index(strings.ToLower(contentType), "content-type:"); colonIdx >= 0 {
		mediaTypeValue = strings.TrimSpace(contentType[colonIdx+13:]) // "content-type:" is 13 chars
	}

	// Parse MIME multipart
	mediaType, params, err := mime.ParseMediaType(mediaTypeValue)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return ""
	}

	mr := multipart.NewReader(strings.NewReader(mimeBody), params["boundary"])
	if mr == nil {
		return ""
	}

	// Process each MIME part
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Read part content
		content, err := io.ReadAll(part)
		if err != nil {
			part.Close()
			continue
		}
		part.Close()

		contentStr := string(content)
		partContentType := part.Header.Get("Content-Type")
		contentTransferEncoding := part.Header.Get("Content-Transfer-Encoding")

		// Handle base64 encoded content
		if strings.ToLower(contentTransferEncoding) == "base64" {
			// Clean base64 string (remove whitespace and newlines)
			cleanB64 := strings.ReplaceAll(strings.ReplaceAll(contentStr, "\n", ""), "\r", "")
			cleanB64 = strings.ReplaceAll(cleanB64, " ", "")

			decoded, err := base64.StdEncoding.DecodeString(cleanB64)
			if err == nil {
				contentStr = string(decoded)
			}
		}

		// Handle gzip compressed content
		if strings.Contains(strings.ToLower(partContentType), "gzip") && len(contentStr) > 0 {
			if reader, err := gzip.NewReader(bytes.NewReader([]byte(contentStr))); err == nil {
				if decompressed, err := io.ReadAll(reader); err == nil {
					contentStr = string(decompressed)
				}
				reader.Close()
			}
		}

		// Look for SMTP TLS report content
		if strings.Contains(strings.ToLower(partContentType), "application/tlsrpt") ||
			strings.Contains(strings.ToLower(partContentType), "tlsrpt") ||
			strings.Contains(contentStr, `"organization-name"`) ||
			strings.Contains(contentStr, `"report-id"`) {
			return contentStr
		}
	}

	return ""
}

// parseAsAggregateReportWithMetrics parses aggregate report with metrics
func (p *Parser) parseAsAggregateReportWithMetrics(data []byte, source string, start time.Time, size int) error {
	report, err := p.parseAggregateXML(data)
	if err != nil {
		duration := time.Since(start).Seconds()
		if p.metrics != nil {
			p.metrics.RecordParseFailure("aggregate", source, "parse_failed", duration, size)
		}
		return err
	}

	if p.storage != nil {
		if err := p.storage.StoreAggregateReport(report); err != nil {
			duration := time.Since(start).Seconds()
			if p.metrics != nil {
				p.metrics.RecordParseFailure("aggregate", source, "storage_failed", duration, size)
			}
			return fmt.Errorf("failed to store aggregate report: %w", err)
		}
	}

	duration := time.Since(start).Seconds()
	if p.metrics != nil {
		p.metrics.RecordParseSuccess("aggregate", source, duration, size)
	}

	p.logger.Info("Successfully parsed aggregate report",
		zap.String("org", report.ReportMetadata.OrgName),
		zap.String("report_id", report.ReportMetadata.ReportID),
		zap.Int("records", len(report.Records)),
		zap.String("source", source),
	)

	return nil
}

// parseAsForensicReportWithMetrics parses forensic report with metrics
func (p *Parser) parseAsForensicReportWithMetrics(data []byte, source string, start time.Time, size int) error {
	report, err := p.parseForensicEmail(data)
	if err != nil {
		duration := time.Since(start).Seconds()
		if p.metrics != nil {
			p.metrics.RecordParseFailure("forensic", source, "parse_failed", duration, size)
		}
		return err
	}

	if p.storage != nil {
		if err := p.storage.StoreForensicReport(report); err != nil {
			duration := time.Since(start).Seconds()
			if p.metrics != nil {
				p.metrics.RecordParseFailure("forensic", source, "storage_failed", duration, size)
			}
			return fmt.Errorf("failed to store forensic report: %w", err)
		}
	}

	duration := time.Since(start).Seconds()
	if p.metrics != nil {
		p.metrics.RecordParseSuccess("forensic", source, duration, size)
	}

	p.logger.Info("Successfully parsed forensic report",
		zap.String("subject", report.Subject),
		zap.String("source_ip", report.Source.IPAddress),
		zap.String("reported_domain", report.ReportedDomain),
		zap.String("source", source),
	)

	return nil
}

// parseAsSMTPTLSReportWithMetrics parses SMTP TLS report with metrics
func (p *Parser) parseAsSMTPTLSReportWithMetrics(data []byte, source string, start time.Time, size int) error {
	// First try to parse as direct JSON
	var report SMTPTLSReport
	var parseErr error
	if err := p.parseJSONWithLineInfo(data, &report); err == nil {
		// Direct JSON parsing succeeded
		return p.processSMTPTLSReportWithMetrics(&report, source, start, size)
	} else {
		parseErr = err
	}

	// Try to parse as email containing SMTP TLS report
	if reportFromEmail, err := p.parseSMTPTLSEmail(data); err == nil {
		return p.processSMTPTLSReportWithMetrics(reportFromEmail, source, start, size)
	}

	// Both parsing attempts failed
	duration := time.Since(start).Seconds()
	if p.metrics != nil {
		p.metrics.RecordParseFailure("smtp_tls", source, "parse_failed", duration, size)
	}
	return fmt.Errorf("failed to parse SMTP TLS report: %w", parseErr)
}

// processSMTPTLSReportWithMetrics handles storage, metrics and logging for SMTP TLS reports
func (p *Parser) processSMTPTLSReportWithMetrics(report *SMTPTLSReport, source string, start time.Time, size int) error {
	if p.storage != nil {
		if err := p.storage.StoreSMTPTLSReport(report); err != nil {
			duration := time.Since(start).Seconds()
			if p.metrics != nil {
				p.metrics.RecordParseFailure("smtp_tls", source, "storage_failed", duration, size)
			}
			return fmt.Errorf("failed to store SMTP TLS report: %w", err)
		}
	}

	duration := time.Since(start).Seconds()
	if p.metrics != nil {
		p.metrics.RecordParseSuccess("smtp_tls", source, duration, size)
	}

	p.logger.Info("Successfully parsed SMTP TLS report",
		zap.String("org", report.OrganizationName),
		zap.String("report_id", report.ReportID),
		zap.Int("policies", len(report.Policies)),
		zap.String("source", source),
	)

	return nil
}

// parseXMLWithLineInfo wraps XML parsing to provide line number information on errors
func (p *Parser) parseXMLWithLineInfo(data []byte, v interface{}) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(v)
	if err != nil {
		// Try to get line information from XML syntax errors
		if syntaxErr, ok := err.(*xml.SyntaxError); ok {
			return fmt.Errorf("XML syntax error at line %d: %w", syntaxErr.Line, err)
		}
		// For other XML errors, try to extract line info if available
		errStr := err.Error()
		if strings.Contains(errStr, "line ") {
			return fmt.Errorf("XML parsing error: %w", err)
		}
		return fmt.Errorf("XML parsing error (unable to determine line): %w", err)
	}
	return nil
}

// parseJSONWithLineInfo wraps JSON parsing to provide line number information on errors
func (p *Parser) parseJSONWithLineInfo(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	if err != nil {
		// Try to get line information from JSON syntax errors
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			// Calculate line number by counting newlines up to the error offset
			lines := bytes.Count(data[:syntaxErr.Offset], []byte{'\n'}) + 1
			return fmt.Errorf("JSON syntax error at line %d: %w", lines, err)
		}
		// For other JSON errors, try to extract line info if available
		errStr := err.Error()
		if strings.Contains(errStr, "line ") {
			return fmt.Errorf("JSON parsing error: %w", err)
		}
		return fmt.Errorf("JSON parsing error (unable to determine line): %w", err)
	}
	return nil
}

// parseAggregateXML parses XML aggregate DMARC report
func (p *Parser) parseAggregateXML(data []byte) (*AggregateReport, error) {
	var feedback struct {
		XMLName        xml.Name `xml:"feedback"`
		Version        string   `xml:"version,omitempty"`
		ReportMetadata struct {
			OrgName          string `xml:"org_name"`
			Email            string `xml:"email"`
			ExtraContactInfo string `xml:"extra_contact_info,omitempty"`
			ReportID         string `xml:"report_id"`
			DateRange        struct {
				Begin string `xml:"begin"`
				End   string `xml:"end"`
			} `xml:"date_range"`
			Error []string `xml:"error,omitempty"`
		} `xml:"report_metadata"`
		PolicyPublished struct {
			Domain string `xml:"domain"`
			ADKIM  string `xml:"adkim,omitempty"`
			ASPF   string `xml:"aspf,omitempty"`
			P      string `xml:"p"`
			SP     string `xml:"sp,omitempty"`
			PCT    string `xml:"pct,omitempty"`
			FO     string `xml:"fo,omitempty"`
		} `xml:"policy_published"`
		Record []struct {
			Row struct {
				SourceIP        string `xml:"source_ip"`
				Count           int    `xml:"count"`
				PolicyEvaluated struct {
					Disposition string `xml:"disposition"`
					DKIM        string `xml:"dkim"`
					SPF         string `xml:"spf"`
					Reason      []struct {
						Type    string `xml:"type"`
						Comment string `xml:"comment,omitempty"`
					} `xml:"reason,omitempty"`
				} `xml:"policy_evaluated"`
			} `xml:"row"`
			Identifiers struct {
				HeaderFrom   string `xml:"header_from"`
				EnvelopeFrom string `xml:"envelope_from,omitempty"`
				EnvelopeTo   string `xml:"envelope_to,omitempty"`
			} `xml:"identifiers"`
			AuthResults struct {
				DKIM []struct {
					Domain   string `xml:"domain"`
					Selector string `xml:"selector,omitempty"`
					Result   string `xml:"result"`
				} `xml:"dkim"`
				SPF []struct {
					Domain string `xml:"domain"`
					Scope  string `xml:"scope,omitempty"`
					Result string `xml:"result"`
				} `xml:"spf"`
			} `xml:"auth_results"`
		} `xml:"record"`
	}

	if err := p.parseXMLWithLineInfo(data, &feedback); err != nil {
		return nil, fmt.Errorf("failed to parse aggregate report XML: %w", err)
	}

	// Convert to internal format
	report := &AggregateReport{
		XMLSchema: feedback.Version,
		ReportMetadata: ReportMetadata{
			OrgName:  feedback.ReportMetadata.OrgName,
			OrgEmail: feedback.ReportMetadata.Email,
			ReportID: feedback.ReportMetadata.ReportID,
			Errors:   feedback.ReportMetadata.Error,
		},
		PolicyPublished: PolicyPublished{
			Domain: feedback.PolicyPublished.Domain,
			ADKIM:  utils.DefaultString(feedback.PolicyPublished.ADKIM, "r"),
			ASPF:   utils.DefaultString(feedback.PolicyPublished.ASPF, "r"),
			P:      feedback.PolicyPublished.P,
			SP:     utils.DefaultString(feedback.PolicyPublished.SP, feedback.PolicyPublished.P),
			PCT:    utils.DefaultString(feedback.PolicyPublished.PCT, "100"),
			FO:     utils.DefaultString(feedback.PolicyPublished.FO, "0"),
		},
	}

	if feedback.ReportMetadata.ExtraContactInfo != "" {
		report.ReportMetadata.OrgExtraContactInfo = &feedback.ReportMetadata.ExtraContactInfo
	}

	// Parse dates
	beginDate, err := utils.ParseTimestamp(feedback.ReportMetadata.DateRange.Begin)
	if err != nil {
		return nil, fmt.Errorf("failed to parse begin date: %w", err)
	}
	report.ReportMetadata.BeginDate = beginDate

	endDate, err := utils.ParseTimestamp(feedback.ReportMetadata.DateRange.End)
	if err != nil {
		return nil, fmt.Errorf("failed to parse end date: %w", err)
	}
	report.ReportMetadata.EndDate = endDate

	// Validate date range (max 24 hours per RFC 7489)
	if endDate.Sub(beginDate) > 48*time.Hour {
		return nil, fmt.Errorf("time span > 24 hours - RFC 7489 section 7.2")
	}

	// Parse records
	for _, xmlRecord := range feedback.Record {
		record := Record{
			Count: xmlRecord.Row.Count,
			Identifiers: Identifiers{
				HeaderFrom: strings.ToLower(xmlRecord.Identifiers.HeaderFrom),
			},
		}

		// Handle envelope from
		if xmlRecord.Identifiers.EnvelopeFrom != "" {
			envelopeFrom := strings.ToLower(xmlRecord.Identifiers.EnvelopeFrom)
			record.Identifiers.EnvelopeFrom = &envelopeFrom
		}

		// Handle envelope to
		if xmlRecord.Identifiers.EnvelopeTo != "" {
			envelopeTo := strings.ToLower(xmlRecord.Identifiers.EnvelopeTo)
			record.Identifiers.EnvelopeTo = &envelopeTo
		}

		// Parse source IP information
		source, err := p.parseSourceIP(xmlRecord.Row.SourceIP)
		if err != nil {
			p.logger.Warn("Failed to parse source IP",
				zap.String("ip", xmlRecord.Row.SourceIP),
				zap.Error(err),
			)
			// Create basic source info
			source = &Source{
				IPAddress: xmlRecord.Row.SourceIP,
				Country:   "Unknown",
				Type:      "Unknown",
			}
		}
		record.Source = *source

		// Parse policy evaluation
		record.PolicyEvaluated = PolicyEvaluated{
			Disposition: xmlRecord.Row.PolicyEvaluated.Disposition,
			DKIM:        utils.DefaultString(xmlRecord.Row.PolicyEvaluated.DKIM, "fail"),
			SPF:         utils.DefaultString(xmlRecord.Row.PolicyEvaluated.SPF, "fail"),
		}

		// Parse policy override reasons
		for _, reason := range xmlRecord.Row.PolicyEvaluated.Reason {
			por := PolicyOverrideReason{}
			if reason.Type != "" {
				por.Type = &reason.Type
			}
			if reason.Comment != "" {
				por.Comment = &reason.Comment
			}
			record.PolicyEvaluated.PolicyOverrideReasons = append(
				record.PolicyEvaluated.PolicyOverrideReasons, por)
		}

		// Parse alignment
		spfAligned := strings.ToLower(record.PolicyEvaluated.SPF) == "pass"
		dkimAligned := strings.ToLower(record.PolicyEvaluated.DKIM) == "pass"
		record.Alignment = Alignment{
			SPF:   spfAligned,
			DKIM:  dkimAligned,
			DMARC: spfAligned || dkimAligned,
		}

		// Parse auth results
		for _, dkimResult := range xmlRecord.AuthResults.DKIM {
			if dkimResult.Domain != "" {
				record.AuthResults.DKIM = append(record.AuthResults.DKIM, DKIMResult{
					Domain:   dkimResult.Domain,
					Selector: utils.DefaultString(dkimResult.Selector, "none"),
					Result:   utils.DefaultString(dkimResult.Result, "none"),
				})
			}
		}

		for _, spfResult := range xmlRecord.AuthResults.SPF {
			if spfResult.Domain != "" {
				record.AuthResults.SPF = append(record.AuthResults.SPF, SPFResult{
					Domain: spfResult.Domain,
					Scope:  utils.DefaultString(spfResult.Scope, "mfrom"),
					Result: utils.DefaultString(spfResult.Result, "none"),
				})
			}
		}

		report.Records = append(report.Records, record)
	}

	return report, nil
}

// parseSourceIP parses source IP information including geolocation
func (p *Parser) parseSourceIP(ipAddress string) (*Source, error) {
	source := &Source{
		IPAddress: ipAddress,
		Country:   "Unknown",
		Type:      "Unknown",
	}

	if !p.config.Offline {
		// Get geolocation info
		if p.config.IPDBPath != "" {
			geo, err := utils.GetGeoLocation(ipAddress, p.config.IPDBPath)
			if err == nil {
				source.Country = geo.Country
			}
		}

		// Get reverse DNS
		if len(p.config.Nameservers) > 0 {
			reverseDNS, err := utils.GetReverseDNS(ipAddress, p.config.Nameservers, p.config.DNSTimeout)
			if err == nil {
				source.ReverseDNS = reverseDNS
				source.BaseDomain = utils.GetBaseDomain(reverseDNS)
				source.Name = reverseDNS
			}
		}
	}

	return source, nil
}

// parseForensicEmail parses a forensic DMARC report from email data
func (p *Parser) parseForensicEmail(emailData []byte) (*ForensicReport, error) {
	// Parse the email message
	emailStr := string(emailData)

	// Split email into headers and body parts
	parts := strings.Split(emailStr, "\r\n\r\n")
	if len(parts) < 2 {
		parts = strings.Split(emailStr, "\n\n")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid email format")
		}
	}

	headers := parts[0]

	// Parse headers
	subject, messageID, arrivalDate := p.parseEmailHeaders(headers)

	// Look for feedback report and sample in the complete email
	feedbackReport, sample := p.extractForensicParts(emailStr)
	if feedbackReport == "" {
		return nil, fmt.Errorf("no feedback report found")
	}

	// Parse the feedback report section
	report, err := p.parseFeedbackReport(feedbackReport, sample, arrivalDate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feedback report: %w", err)
	}

	// Set additional fields from email
	report.Subject = subject
	report.MessageID = messageID

	return report, nil
}

// parseEmailHeaders extracts relevant headers from email
func (p *Parser) parseEmailHeaders(headers string) (subject, messageID string, arrivalDate time.Time) {
	arrivalDate = time.Now().UTC() // default

	lines := strings.Split(headers, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(strings.ToLower(line), "subject:") {
			subject = strings.TrimSpace(line[8:])
		} else if strings.HasPrefix(strings.ToLower(line), "message-id:") {
			messageID = strings.TrimSpace(line[11:])
		} else if strings.HasPrefix(strings.ToLower(line), "date:") {
			dateStr := strings.TrimSpace(line[5:])
			if parsed, err := time.Parse(time.RFC1123Z, dateStr); err == nil {
				arrivalDate = parsed.UTC()
			} else if parsed, err := time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", dateStr); err == nil {
				arrivalDate = parsed.UTC()
			}
		}
	}

	return
}

// extractForensicParts extracts feedback report and sample from email body
func (p *Parser) extractForensicParts(body string) (feedbackReport, sample string) {
	// First try to parse as multipart MIME message
	feedbackReport, sample = p.extractFromMIME(body)
	if feedbackReport != "" {
		return feedbackReport, sample
	}

	// Fall back to simple text patterns for non-MIME messages
	if strings.Contains(body, "Feedback-Type:") {
		// Find feedback report section
		lines := strings.Split(body, "\n")
		inFeedback := false
		var feedbackLines []string
		var sampleLines []string
		inSample := false

		for _, line := range lines {
			line = strings.TrimSpace(line)

			// Check for feedback section start
			if strings.HasPrefix(line, "Feedback-Type:") {
				inFeedback = true
				inSample = false
				feedbackLines = append(feedbackLines, line)
				continue
			}

			// Check for sample section (headers or full message)
			if strings.Contains(line, "The original message headers were:") ||
				strings.Contains(line, "Received:") ||
				strings.Contains(line, "Return-Path:") {
				inSample = true
				inFeedback = false
				if !strings.Contains(line, "original message headers") {
					sampleLines = append(sampleLines, line)
				}
				continue
			}

			// Empty line might separate sections
			if line == "" {
				if inFeedback && len(feedbackLines) > 0 {
					// End of feedback section
					inFeedback = false
				}
				continue
			}

			if inFeedback {
				feedbackLines = append(feedbackLines, line)
			} else if inSample {
				sampleLines = append(sampleLines, line)
			}
		}

		feedbackReport = strings.Join(feedbackLines, "\n")
		sample = strings.Join(sampleLines, "\n")
	}

	return
}

// extractFromMIME extracts forensic parts from MIME multipart message
func (p *Parser) extractFromMIME(body string) (feedbackReport, sample string) {
	// Look for Content-Type header with boundary
	lines := strings.Split(body, "\n")
	var contentType string
	bodyStartIdx := 0

	// Find Content-Type header and body start, handling multiline headers
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "content-type:") {
			// Start building content type, may span multiple lines
			contentType = line
			// Look ahead for continuation lines (start with whitespace)
			for j := i + 1; j < len(lines); j++ {
				nextLine := lines[j]
				if strings.HasPrefix(nextLine, " ") || strings.HasPrefix(nextLine, "\t") {
					contentType += " " + strings.TrimSpace(nextLine)
				} else if strings.TrimSpace(nextLine) == "" {
					// Empty line after headers marks start of body
					bodyStartIdx = j + 1
					break
				} else {
					// Non-continuation line, this header is complete
					break
				}
			}
			break
		} else if line == "" {
			// Empty line after headers marks start of body
			bodyStartIdx = i + 1
			break
		}
	}

	// Extract boundary from content type
	var boundary string
	if strings.Contains(strings.ToLower(contentType), "boundary=") {
		parts := strings.Split(contentType, "boundary=")
		if len(parts) >= 2 {
			boundaryPart := strings.Trim(parts[1], `"`)
			// Remove any trailing content after the boundary value
			if idx := strings.Index(boundaryPart, ";"); idx > 0 {
				boundaryPart = boundaryPart[:idx]
			}
			if idx := strings.Index(boundaryPart, " "); idx > 0 {
				boundaryPart = boundaryPart[:idx]
			}
			boundary = strings.Trim(boundaryPart, `"`)
		}
	}

	if boundary == "" || !strings.Contains(strings.ToLower(contentType), "multipart") {
		return "", ""
	}

	// Reconstruct the body from bodyStartIdx
	if bodyStartIdx >= len(lines) {
		return "", ""
	}
	bodyLines := lines[bodyStartIdx:]
	mimeBody := strings.Join(bodyLines, "\n")

	// Extract media type value from header (remove "Content-type: " prefix)
	mediaTypeValue := contentType
	if colonIdx := strings.Index(strings.ToLower(contentType), "content-type:"); colonIdx >= 0 {
		mediaTypeValue = strings.TrimSpace(contentType[colonIdx+13:]) // "content-type:" is 13 chars
	}

	// Parse MIME multipart
	mediaType, params, err := mime.ParseMediaType(mediaTypeValue)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return "", ""
	}

	mr := multipart.NewReader(strings.NewReader(mimeBody), params["boundary"])
	if mr == nil {
		return "", ""
	}

	// Process each MIME part
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		// Read part content
		content, err := io.ReadAll(part)
		if err != nil {
			part.Close()
			continue
		}
		part.Close()

		contentStr := string(content)

		// Check Content-Type of this part
		partContentType := part.Header.Get("Content-Type")
		contentTransferEncoding := part.Header.Get("Content-Transfer-Encoding")

		// Handle base64 encoded content
		if strings.ToLower(contentTransferEncoding) == "base64" {
			// Clean base64 string (remove whitespace and newlines)
			cleanB64 := strings.ReplaceAll(strings.ReplaceAll(contentStr, "\n", ""), "\r", "")
			cleanB64 = strings.ReplaceAll(cleanB64, " ", "")

			decoded, err := base64.StdEncoding.DecodeString(cleanB64)
			if err != nil {
				// Try StdEncoding without padding
				decoded, err = base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(cleanB64)
			}
			if err == nil {
				contentStr = string(decoded)
			}
		}

		// Look for feedback report content type or content with Feedback-Type
		if strings.Contains(strings.ToLower(partContentType), "message/feedback-report") ||
			strings.Contains(contentStr, "Feedback-Type:") {
			feedbackReport = contentStr
		} else if strings.Contains(strings.ToLower(partContentType), "message/rfc822") ||
			strings.Contains(contentStr, "Received:") ||
			strings.Contains(contentStr, "Return-Path:") {
			sample = contentStr
		}
	}

	return feedbackReport, sample
}

// parseFeedbackReport parses the feedback report section
func (p *Parser) parseFeedbackReport(feedbackReport, sample string, arrivalDate time.Time) (*ForensicReport, error) {
	report := &ForensicReport{
		ArrivalDate:    arrivalDate,
		ArrivalDateUTC: arrivalDate,
		Sample:         sample,
	}

	// Parse feedback report fields
	lines := strings.Split(feedbackReport, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split on first colon
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		field := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch field {
		case "feedback-type":
			report.FeedbackType = value
		case "user-agent":
			report.UserAgent = &value
		case "version":
			report.Version = &value
		case "original-envelope-id":
			report.OriginalEnvelopeID = &value
		case "original-mail-from":
			report.OriginalMailFrom = &value
		case "original-rcpt-to":
			report.OriginalRcptTo = &value
		case "arrival-date":
			if parsed, err := time.Parse(time.RFC3339, value); err == nil {
				report.ArrivalDate = parsed
				report.ArrivalDateUTC = parsed.UTC()
			}
		case "source-ip":
			// Parse source IP and get geo info
			sourceIP := strings.Fields(value)[0] // Take first IP if multiple
			source, err := p.parseSourceIP(sourceIP)
			if err != nil {
				p.logger.Warn("Failed to parse source IP",
					zap.String("ip", sourceIP),
					zap.Error(err),
				)
				// Create basic source info
				source = &Source{
					IPAddress: sourceIP,
					Country:   "Unknown",
					Type:      "Unknown",
				}
			}
			report.Source = *source
		case "authentication-results":
			report.AuthenticationResults = value
		case "dkim-domain":
			report.DKIMDomain = &value
		case "reported-domain":
			report.ReportedDomain = value
		case "delivery-result":
			report.DeliveryResult = value
		case "auth-failure":
			// Split comma-separated auth failures
			failures := strings.Split(value, ",")
			for i, failure := range failures {
				failures[i] = strings.TrimSpace(failure)
			}
			report.AuthFailure = failures
		case "identity-alignment":
			// Parse authentication mechanisms
			if value != "none" {
				mechanisms := strings.Split(value, ",")
				for i, mech := range mechanisms {
					mechanisms[i] = strings.TrimSpace(mech)
				}
				report.AuthenticationMechanisms = mechanisms
			}
		}
	}

	// Set defaults
	if report.FeedbackType == "" {
		report.FeedbackType = "auth-failure"
	}

	if report.DeliveryResult == "" {
		report.DeliveryResult = "other"
	} else {
		// Normalize delivery result
		deliveryResults := []string{"delivered", "spam", "policy", "reject", "other"}
		normalized := "other"
		for _, dr := range deliveryResults {
			if strings.Contains(strings.ToLower(report.DeliveryResult), dr) {
				normalized = dr
				break
			}
		}
		report.DeliveryResult = normalized
	}

	if len(report.AuthFailure) == 0 {
		report.AuthFailure = []string{"dmarc"}
	}

	if report.ReportedDomain == "" && report.Source.IPAddress != "" {
		// Try to extract domain from sample headers if available
		report.ReportedDomain = p.extractDomainFromSample(sample)
	}

	// Determine if sample contains only headers
	report.SampleHeadersOnly = !strings.Contains(sample, "\n\n") &&
		(strings.Contains(sample, "Received:") || strings.Contains(sample, "From:"))

	// Parse sample as JSON (simplified)
	parsedSample := map[string]interface{}{
		"headers_only": report.SampleHeadersOnly,
		"raw_sample":   sample,
	}

	if sampleJSON, err := json.Marshal(parsedSample); err == nil {
		report.ParsedSample = sampleJSON
	}

	return report, nil
}

// extractDomainFromSample tries to extract domain from email sample
func (p *Parser) extractDomainFromSample(sample string) string {
	lines := strings.Split(sample, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "from:") {
			// Extract domain from From header
			fromValue := strings.TrimSpace(line[5:])
			// Look for email in angle brackets or just the email
			if idx := strings.LastIndex(fromValue, "@"); idx != -1 {
				domain := fromValue[idx+1:]
				if idx := strings.Index(domain, ">"); idx != -1 {
					domain = domain[:idx]
				}
				if idx := strings.Index(domain, " "); idx != -1 {
					domain = domain[:idx]
				}
				return strings.TrimSpace(domain)
			}
		}
	}
	return ""
}

// ParseAggregateFromBytes parses aggregate report from byte data
func (p *Parser) ParseAggregateFromBytes(data []byte) (*AggregateReport, error) {
	// Extract content if compressed
	extractedData, err := p.extractReportData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to extract report data: %w", err)
	}

	// Parse as aggregate report
	return p.parseAggregateXML(extractedData)
}

// ParseForensicFromBytes parses forensic report from byte data
func (p *Parser) ParseForensicFromBytes(data []byte) (*ForensicReport, error) {
	// Extract content if compressed
	extractedData, err := p.extractReportData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to extract report data: %w", err)
	}

	// Parse as forensic report
	return p.parseForensicEmail(extractedData)
}

// ParseSMTPTLSFromBytes parses SMTP TLS report from byte data
func (p *Parser) ParseSMTPTLSFromBytes(data []byte) (*SMTPTLSReport, error) {
	// Extract content if compressed
	extractedData, err := p.extractReportData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to extract report data: %w", err)
	}

	// Parse as SMTP TLS report (JSON)
	var report SMTPTLSReport
	err = p.parseJSONWithLineInfo(extractedData, &report)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SMTP TLS report: %w", err)
	}

	return &report, nil
}
