package validation

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Validator handles validation of DMARC reports and related data
type Validator struct {
	logger *zap.Logger
}

// New creates a new validator instance
func New(logger *zap.Logger) *Validator {
	return &Validator{
		logger: logger,
	}
}

// ValidationResult contains the result of validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateXMLReport validates a DMARC aggregate XML report
func (v *Validator) ValidateXMLReport(data []byte) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Check if data is valid XML
	if !v.isValidXML(data) {
		result.Valid = false
		result.Errors = append(result.Errors, "Invalid XML format")
		return result
	}

	// Parse and validate structure
	var feedback struct {
		XMLName        xml.Name `xml:"feedback"`
		Version        string   `xml:"version,omitempty"`
		ReportMetadata struct {
			OrgName   string `xml:"org_name"`
			Email     string `xml:"email"`
			ReportID  string `xml:"report_id"`
			DateRange struct {
				Begin string `xml:"begin"`
				End   string `xml:"end"`
			} `xml:"date_range"`
		} `xml:"report_metadata"`
		PolicyPublished struct {
			Domain string `xml:"domain"`
			P      string `xml:"p"`
		} `xml:"policy_published"`
		Record []struct {
			Row struct {
				SourceIP string `xml:"source_ip"`
				Count    int    `xml:"count"`
			} `xml:"row"`
			Identifiers struct {
				HeaderFrom string `xml:"header_from"`
			} `xml:"identifiers"`
		} `xml:"record"`
	}

	if err := xml.Unmarshal(data, &feedback); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to parse XML: %v", err))
		return result
	}

	// Validate required fields
	if feedback.ReportMetadata.OrgName == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "Missing organization name")
	}

	if feedback.ReportMetadata.ReportID == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "Missing report ID")
	}

	// Validate email format
	if feedback.ReportMetadata.Email != "" && !v.isValidEmail(feedback.ReportMetadata.Email) {
		result.Warnings = append(result.Warnings, "Invalid email format in report metadata")
	}

	// Validate domain
	if feedback.PolicyPublished.Domain == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "Missing domain in policy published")
	} else if !v.isValidDomain(feedback.PolicyPublished.Domain) {
		result.Valid = false
		result.Errors = append(result.Errors, "Invalid domain format in policy published")
	}

	// Validate DMARC policy
	if !v.isValidDMARCPolicy(feedback.PolicyPublished.P) {
		result.Valid = false
		result.Errors = append(result.Errors, "Invalid DMARC policy value")
	}

	// Validate date range
	if err := v.validateDateRange(feedback.ReportMetadata.DateRange.Begin, feedback.ReportMetadata.DateRange.End); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid date range: %v", err))
	}

	// Validate records
	if len(feedback.Record) == 0 {
		result.Warnings = append(result.Warnings, "No records found in report")
	} else {
		for i, record := range feedback.Record {
			if record.Row.Count <= 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Record %d has invalid count: %d", i+1, record.Row.Count))
			}

			if !v.isValidIP(record.Row.SourceIP) {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("Record %d has invalid source IP: %s", i+1, record.Row.SourceIP))
			}

			if record.Identifiers.HeaderFrom == "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Record %d missing header_from", i+1))
			} else if !v.isValidDomain(record.Identifiers.HeaderFrom) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Record %d has invalid header_from domain: %s", i+1, record.Identifiers.HeaderFrom))
			}
		}
	}

	return result
}

// ValidateJSONReport validates a DMARC JSON report (like SMTP TLS)
func (v *Validator) ValidateJSONReport(data []byte) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if !v.isValidJSON(data) {
		result.Valid = false
		result.Errors = append(result.Errors, "Invalid JSON format")
		return result
	}

	// Additional JSON-specific validation can be added here
	return result
}

// ValidateBase64Content validates base64 encoded content
func (v *Validator) ValidateBase64Content(content string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if _, err := base64.StdEncoding.DecodeString(content); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, "Invalid base64 encoding")
	}

	return result
}

// ValidateReportSize checks if report size is within acceptable limits
func (v *Validator) ValidateReportSize(size int64, maxSize int64) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if size <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "Empty report content")
	}

	if maxSize > 0 && size > maxSize {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Report size (%d bytes) exceeds maximum allowed size (%d bytes)", size, maxSize))
	}

	// Warning for very large reports
	if size > 10*1024*1024 { // 10MB
		result.Warnings = append(result.Warnings, "Report size is very large, consider using compression")
	}

	return result
}

// ValidateReportID checks if report ID follows expected format
func (v *Validator) ValidateReportID(reportID string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if reportID == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "Report ID cannot be empty")
		return result
	}

	// Report ID should be reasonable length
	if len(reportID) > 255 {
		result.Valid = false
		result.Errors = append(result.Errors, "Report ID too long (max 255 characters)")
	}

	// Check for potentially dangerous characters
	if v.containsDangerousChars(reportID) {
		result.Valid = false
		result.Errors = append(result.Errors, "Report ID contains potentially dangerous characters")
	}

	return result
}

// Helper validation methods

func (v *Validator) isValidXML(data []byte) bool {
	return xml.Unmarshal(data, &struct{}{}) == nil
}

func (v *Validator) isValidJSON(data []byte) bool {
	// Basic check - could be more sophisticated
	trimmed := strings.TrimSpace(string(data))
	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}

func (v *Validator) isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func (v *Validator) isValidDomain(domain string) bool {
	if domain == "" {
		return false
	}

	// Basic domain validation
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	return domainRegex.MatchString(domain)
}

func (v *Validator) isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func (v *Validator) isValidDMARCPolicy(policy string) bool {
	validPolicies := []string{"none", "quarantine", "reject"}
	for _, validPolicy := range validPolicies {
		if policy == validPolicy {
			return true
		}
	}
	return false
}

func (v *Validator) validateDateRange(beginStr, endStr string) error {
	begin, err := v.parseTimestamp(beginStr)
	if err != nil {
		return fmt.Errorf("invalid begin date: %v", err)
	}

	end, err := v.parseTimestamp(endStr)
	if err != nil {
		return fmt.Errorf("invalid end date: %v", err)
	}

	if end.Before(begin) {
		return fmt.Errorf("end date is before begin date")
	}

	// RFC 7489: reports should cover at most 24 hours
	if end.Sub(begin) > 48*time.Hour {
		return fmt.Errorf("date range exceeds 48 hours (RFC 7489 recommends max 24 hours)")
	}

	// Check if dates are too far in the future
	now := time.Now().UTC()
	if begin.After(now.Add(24*time.Hour)) || end.After(now.Add(24*time.Hour)) {
		return fmt.Errorf("report dates are too far in the future")
	}

	return nil
}

func (v *Validator) parseTimestamp(timestamp string) (time.Time, error) {
	// Try Unix timestamp first
	if len(timestamp) == 10 {
		if unixTime, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
			return time.Unix(unixTime, 0).UTC(), nil
		}
	}

	// Try RFC3339 format
	if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
		return t.UTC(), nil
	}

	// Try other common formats
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timestamp); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestamp)
}

func (v *Validator) containsDangerousChars(input string) bool {
	dangerousChars := []string{
		"<", ">", "&", "\"", "'",
		"\x00", "\r", "\n",
		"../", "..\\",
		"<script", "javascript:",
		"<?php", "<%",
	}

	inputLower := strings.ToLower(input)
	for _, dangerous := range dangerousChars {
		if strings.Contains(inputLower, dangerous) {
			return true
		}
	}

	return false
}

// SanitizeInput sanitizes input string for safe processing
func (v *Validator) SanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Remove other control characters except tab, newline, carriage return
	result := strings.Map(func(r rune) rune {
		if r == 0 || (r > 0 && r < 32 && r != 9 && r != 10 && r != 13) {
			return -1
		}
		return r
	}, input)

	// Trim whitespace
	return strings.TrimSpace(result)
}

// ValidateBatch validates multiple reports in batch
func (v *Validator) ValidateBatch(reports [][]byte, maxReports int) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if len(reports) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "No reports to validate")
		return result
	}

	if maxReports > 0 && len(reports) > maxReports {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Too many reports (%d), maximum allowed is %d", len(reports), maxReports))
		return result
	}

	// Validate each report
	for i, reportData := range reports {
		reportResult := v.ValidateXMLReport(reportData)
		if !reportResult.Valid {
			result.Valid = false
			for _, err := range reportResult.Errors {
				result.Errors = append(result.Errors, fmt.Sprintf("Report %d: %s", i+1, err))
			}
		}

		for _, warning := range reportResult.Warnings {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Report %d: %s", i+1, warning))
		}
	}

	return result
}
