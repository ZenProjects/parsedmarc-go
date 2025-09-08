package output

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"parsedmarc-go/internal/parser"
)

// Format represents the output format type
type Format string

const (
	FormatJSON Format = "json"
	FormatCSV  Format = "csv"
)

// Writer interface for output writers
type Writer interface {
	WriteAggregateReport(report *parser.AggregateReport) error
	WriteForensicReport(report *parser.ForensicReport) error
	WriteSMTPTLSReport(report *parser.SMTPTLSReport) error
	Close() error
}

// SMTPSender interface for sending reports via SMTP
type SMTPSender interface {
	SendAggregateReport(report *parser.AggregateReport) error
	SendForensicReport(report *parser.ForensicReport) error
	SendSMTPTLSReport(report *parser.SMTPTLSReport) error
}

// KafkaSender interface for sending reports via Kafka
type KafkaSender interface {
	SendAggregateReport(report *parser.AggregateReport) error
	SendForensicReport(report *parser.ForensicReport) error
	SendSMTPTLSReport(report *parser.SMTPTLSReport) error
}

// Config holds output configuration
type Config struct {
	Format      Format
	File        string // empty string means stdout, directory path for per-report files
	SMTPSender  SMTPSender
	KafkaSender KafkaSender
	Logger      *zap.Logger
}

// NewWriter creates a new output writer based on configuration
func NewWriter(cfg Config) (Writer, error) {
	// Check if cfg.File is a directory
	if cfg.File != "" {
		stat, err := os.Stat(cfg.File)
		if err == nil && stat.IsDir() {
			// Directory mode - create individual files per report
			switch cfg.Format {
			case FormatJSON:
				return &DirectoryJSONWriter{
					outputDir:   cfg.File,
					smtpSender:  cfg.SMTPSender,
					kafkaSender: cfg.KafkaSender,
					logger:      cfg.Logger,
				}, nil
			case FormatCSV:
				return &DirectoryCSVWriter{
					outputDir:   cfg.File,
					smtpSender:  cfg.SMTPSender,
					kafkaSender: cfg.KafkaSender,
					logger:      cfg.Logger,
				}, nil
			default:
				return nil, fmt.Errorf("unsupported output format: %s", cfg.Format)
			}
		} else if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to stat output path %s: %w", cfg.File, err)
		}
	}

	// Original file/stdout mode
	var w io.Writer
	var closer io.Closer

	if cfg.File == "" {
		w = os.Stdout
	} else {
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open output file %s: %w", cfg.File, err)
		}
		w = file
		closer = file
	}

	switch cfg.Format {
	case FormatJSON:
		return &JSONWriter{
			writer:      w,
			closer:      closer,
			smtpSender:  cfg.SMTPSender,
			kafkaSender: cfg.KafkaSender,
			logger:      cfg.Logger,
		}, nil
	case FormatCSV:
		return &CSVWriter{
			writer:      w,
			closer:      closer,
			csvWriter:   csv.NewWriter(w),
			smtpSender:  cfg.SMTPSender,
			kafkaSender: cfg.KafkaSender,
			logger:      cfg.Logger,
		}, nil
	default:
		if closer != nil {
			closer.Close()
		}
		return nil, fmt.Errorf("unsupported output format: %s", cfg.Format)
	}
}

// JSONWriter writes output in JSON format
type JSONWriter struct {
	writer      io.Writer
	closer      io.Closer
	smtpSender  SMTPSender
	kafkaSender KafkaSender
	logger      *zap.Logger
}

func (j *JSONWriter) WriteAggregateReport(report *parser.AggregateReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal aggregate report to JSON: %w", err)
	}

	_, err = j.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	// Add newline for better formatting
	_, err = j.writer.Write([]byte("\n"))
	if err != nil {
		return err
	}

	// Send via SMTP if configured
	if j.smtpSender != nil {
		if err := j.smtpSender.SendAggregateReport(report); err != nil {
			j.logger.Error("Failed to send aggregate report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if j.kafkaSender != nil {
		if err := j.kafkaSender.SendAggregateReport(report); err != nil {
			j.logger.Error("Failed to send aggregate report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (j *JSONWriter) WriteForensicReport(report *parser.ForensicReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal forensic report to JSON: %w", err)
	}

	_, err = j.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	// Add newline for better formatting
	_, err = j.writer.Write([]byte("\n"))
	if err != nil {
		return err
	}

	// Send via SMTP if configured
	if j.smtpSender != nil {
		if err := j.smtpSender.SendForensicReport(report); err != nil {
			j.logger.Error("Failed to send forensic report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if j.kafkaSender != nil {
		if err := j.kafkaSender.SendForensicReport(report); err != nil {
			j.logger.Error("Failed to send forensic report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (j *JSONWriter) WriteSMTPTLSReport(report *parser.SMTPTLSReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SMTP TLS report to JSON: %w", err)
	}

	_, err = j.writer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	// Add newline for better formatting
	_, err = j.writer.Write([]byte("\n"))
	if err != nil {
		return err
	}

	// Send via SMTP if configured
	if j.smtpSender != nil {
		if err := j.smtpSender.SendSMTPTLSReport(report); err != nil {
			j.logger.Error("Failed to send SMTP TLS report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if j.kafkaSender != nil {
		if err := j.kafkaSender.SendSMTPTLSReport(report); err != nil {
			j.logger.Error("Failed to send SMTP TLS report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (j *JSONWriter) Close() error {
	if j.closer != nil {
		return j.closer.Close()
	}
	return nil
}

// CSVWriter writes output in CSV format
type CSVWriter struct {
	writer         io.Writer
	closer         io.Closer
	csvWriter      *csv.Writer
	headersWritten map[string]bool
	smtpSender     SMTPSender
	kafkaSender    KafkaSender
	logger         *zap.Logger
}

func (c *CSVWriter) WriteAggregateReport(report *parser.AggregateReport) error {
	if c.headersWritten == nil {
		c.headersWritten = make(map[string]bool)
	}

	// Write headers if not written yet
	if !c.headersWritten["aggregate"] {
		headers := []string{
			"report_id", "org_name", "org_email", "begin_date", "end_date",
			"domain", "policy_adkim", "policy_aspf", "policy_p", "policy_sp", "policy_pct",
			"source_ip", "source_country", "source_reverse_dns", "count",
			"disposition", "dkim_result", "spf_result", "dmarc_aligned",
			"header_from", "envelope_from", "dkim_domain", "dkim_selector", "spf_domain",
		}
		if err := c.csvWriter.Write(headers); err != nil {
			return fmt.Errorf("failed to write CSV headers: %w", err)
		}
		c.headersWritten["aggregate"] = true
	}

	// Write each record as a row
	for _, record := range report.Records {
		row := []string{
			report.ReportMetadata.ReportID,
			report.ReportMetadata.OrgName,
			report.ReportMetadata.OrgEmail,
			report.ReportMetadata.BeginDate.Format(time.RFC3339),
			report.ReportMetadata.EndDate.Format(time.RFC3339),
			report.PolicyPublished.Domain,
			report.PolicyPublished.ADKIM,
			report.PolicyPublished.ASPF,
			report.PolicyPublished.P,
			report.PolicyPublished.SP,
			report.PolicyPublished.PCT,
			record.Source.IPAddress,
			record.Source.Country,
			record.Source.ReverseDNS,
			strconv.Itoa(record.Count),
			record.PolicyEvaluated.Disposition,
			record.PolicyEvaluated.DKIM,
			record.PolicyEvaluated.SPF,
			strconv.FormatBool(record.Alignment.DMARC),
			record.Identifiers.HeaderFrom,
			stringPtrToString(record.Identifiers.EnvelopeFrom),
			getDKIMDomain(record.AuthResults.DKIM),
			getDKIMSelector(record.AuthResults.DKIM),
			getSPFDomain(record.AuthResults.SPF),
		}

		if err := c.csvWriter.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	c.csvWriter.Flush()
	if err := c.csvWriter.Error(); err != nil {
		return err
	}

	// Send via SMTP if configured
	if c.smtpSender != nil {
		if err := c.smtpSender.SendAggregateReport(report); err != nil {
			c.logger.Error("Failed to send aggregate report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if c.kafkaSender != nil {
		if err := c.kafkaSender.SendAggregateReport(report); err != nil {
			c.logger.Error("Failed to send aggregate report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (c *CSVWriter) WriteForensicReport(report *parser.ForensicReport) error {
	if c.headersWritten == nil {
		c.headersWritten = make(map[string]bool)
	}

	// Write headers if not written yet
	if !c.headersWritten["forensic"] {
		headers := []string{
			"feedback_type", "user_agent", "version", "original_envelope_id",
			"original_mail_from", "original_rcpt_to", "arrival_date", "subject",
			"message_id", "authentication_results", "dkim_domain", "source_ip",
			"source_country", "delivery_result", "auth_failure", "reported_domain",
		}
		if err := c.csvWriter.Write(headers); err != nil {
			return fmt.Errorf("failed to write CSV headers: %w", err)
		}
		c.headersWritten["forensic"] = true
	}

	row := []string{
		report.FeedbackType,
		stringPtrToString(report.UserAgent),
		stringPtrToString(report.Version),
		stringPtrToString(report.OriginalEnvelopeID),
		stringPtrToString(report.OriginalMailFrom),
		stringPtrToString(report.OriginalRcptTo),
		report.ArrivalDate.Format(time.RFC3339),
		report.Subject,
		report.MessageID,
		report.AuthenticationResults,
		stringPtrToString(report.DKIMDomain),
		report.Source.IPAddress,
		report.Source.Country,
		report.DeliveryResult,
		strings.Join(report.AuthFailure, ";"),
		report.ReportedDomain,
	}

	if err := c.csvWriter.Write(row); err != nil {
		return fmt.Errorf("failed to write CSV row: %w", err)
	}

	c.csvWriter.Flush()
	err := c.csvWriter.Error()
	if err != nil {
		return err
	}

	// Send via SMTP if configured
	if c.smtpSender != nil {
		if err := c.smtpSender.SendForensicReport(report); err != nil {
			c.logger.Error("Failed to send forensic report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if c.kafkaSender != nil {
		if err := c.kafkaSender.SendForensicReport(report); err != nil {
			c.logger.Error("Failed to send forensic report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (c *CSVWriter) WriteSMTPTLSReport(report *parser.SMTPTLSReport) error {
	if c.headersWritten == nil {
		c.headersWritten = make(map[string]bool)
	}

	// Write headers if not written yet
	if !c.headersWritten["smtp_tls"] {
		headers := []string{
			"organization_name", "begin_date", "end_date", "contact_info", "report_id",
			"policy_domain", "policy_type", "successful_session_count", "failed_session_count",
			"failure_result_type", "failure_sending_mta_ip", "failure_receiving_ip",
		}
		if err := c.csvWriter.Write(headers); err != nil {
			return fmt.Errorf("failed to write CSV headers: %w", err)
		}
		c.headersWritten["smtp_tls"] = true
	}

	// Write each policy as rows
	for _, policy := range report.Policies {
		// Base row for policy
		baseRow := []string{
			report.OrganizationName,
			report.BeginDate.Format(time.RFC3339),
			report.EndDate.Format(time.RFC3339),
			report.ContactInfo,
			report.ReportID,
			policy.PolicyDomain,
			policy.PolicyType,
			strconv.Itoa(policy.SuccessfulSessionCount),
			strconv.Itoa(policy.FailedSessionCount),
			"", // failure_result_type (filled below)
			"", // failure_sending_mta_ip (filled below)
			"", // failure_receiving_ip (filled below)
		}

		if len(policy.FailureDetails) == 0 {
			// Write row without failure details
			if err := c.csvWriter.Write(baseRow); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
		} else {
			// Write one row per failure detail
			for _, failure := range policy.FailureDetails {
				row := make([]string, len(baseRow))
				copy(row, baseRow)
				row[9] = failure.ResultType                       // failure_result_type
				row[10] = stringPtrToString(failure.SendingMTAIP) // failure_sending_mta_ip
				row[11] = stringPtrToString(failure.ReceivingIP)  // failure_receiving_ip

				if err := c.csvWriter.Write(row); err != nil {
					return fmt.Errorf("failed to write CSV row: %w", err)
				}
			}
		}
	}

	c.csvWriter.Flush()
	err := c.csvWriter.Error()
	if err != nil {
		return err
	}

	// Send via SMTP if configured
	if c.smtpSender != nil {
		if err := c.smtpSender.SendSMTPTLSReport(report); err != nil {
			c.logger.Error("Failed to send SMTP TLS report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if c.kafkaSender != nil {
		if err := c.kafkaSender.SendSMTPTLSReport(report); err != nil {
			c.logger.Error("Failed to send SMTP TLS report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (c *CSVWriter) Close() error {
	if c.csvWriter != nil {
		c.csvWriter.Flush()
	}
	if c.closer != nil {
		return c.closer.Close()
	}
	return nil
}

// Helper functions
func stringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func getDKIMDomain(dkimResults []parser.DKIMResult) string {
	if len(dkimResults) == 0 {
		return ""
	}
	return dkimResults[0].Domain
}

func getDKIMSelector(dkimResults []parser.DKIMResult) string {
	if len(dkimResults) == 0 {
		return ""
	}
	return dkimResults[0].Selector
}

func getSPFDomain(spfResults []parser.SPFResult) string {
	if len(spfResults) == 0 {
		return ""
	}
	return spfResults[0].Domain
}

// DirectoryJSONWriter writes each report as a separate JSON file in a directory
type DirectoryJSONWriter struct {
	outputDir   string
	smtpSender  SMTPSender
	kafkaSender KafkaSender
	logger      *zap.Logger
}

func (d *DirectoryJSONWriter) WriteAggregateReport(report *parser.AggregateReport) error {
	filename := d.generateAggregateFilename(report, "json")
	filePath := filepath.Join(d.outputDir, filename)

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal aggregate report to JSON: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", filePath, err)
	}

	d.logger.Info("Wrote aggregate report", zap.String("file", filePath))

	// Send via SMTP if configured
	if d.smtpSender != nil {
		if err := d.smtpSender.SendAggregateReport(report); err != nil {
			d.logger.Error("Failed to send aggregate report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if d.kafkaSender != nil {
		if err := d.kafkaSender.SendAggregateReport(report); err != nil {
			d.logger.Error("Failed to send aggregate report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (d *DirectoryJSONWriter) WriteForensicReport(report *parser.ForensicReport) error {
	filename := d.generateForensicFilename(report, "json")
	filePath := filepath.Join(d.outputDir, filename)

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal forensic report to JSON: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", filePath, err)
	}

	d.logger.Info("Wrote forensic report", zap.String("file", filePath))

	// Send via SMTP if configured
	if d.smtpSender != nil {
		if err := d.smtpSender.SendForensicReport(report); err != nil {
			d.logger.Error("Failed to send forensic report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if d.kafkaSender != nil {
		if err := d.kafkaSender.SendForensicReport(report); err != nil {
			d.logger.Error("Failed to send forensic report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (d *DirectoryJSONWriter) WriteSMTPTLSReport(report *parser.SMTPTLSReport) error {
	filename := d.generateSMTPTLSFilename(report, "json")
	filePath := filepath.Join(d.outputDir, filename)

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SMTP TLS report to JSON: %w", err)
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write JSON file %s: %w", filePath, err)
	}

	d.logger.Info("Wrote SMTP TLS report", zap.String("file", filePath))

	// Send via SMTP if configured
	if d.smtpSender != nil {
		if err := d.smtpSender.SendSMTPTLSReport(report); err != nil {
			d.logger.Error("Failed to send SMTP TLS report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if d.kafkaSender != nil {
		if err := d.kafkaSender.SendSMTPTLSReport(report); err != nil {
			d.logger.Error("Failed to send SMTP TLS report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (d *DirectoryJSONWriter) Close() error {
	return nil
}

// DirectoryCSVWriter writes each report as a separate CSV file in a directory
type DirectoryCSVWriter struct {
	outputDir   string
	smtpSender  SMTPSender
	kafkaSender KafkaSender
	logger      *zap.Logger
}

func (d *DirectoryCSVWriter) WriteAggregateReport(report *parser.AggregateReport) error {
	filename := d.generateAggregateFilename(report, "csv")
	filePath := filepath.Join(d.outputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file %s: %w", filePath, err)
	}
	defer file.Close()

	csvWriter := csv.NewWriter(file)
	defer csvWriter.Flush()

	// Write headers
	headers := []string{
		"report_id", "org_name", "org_email", "begin_date", "end_date",
		"domain", "policy_adkim", "policy_aspf", "policy_p", "policy_sp", "policy_pct",
		"source_ip", "source_country", "source_reverse_dns", "count",
		"disposition", "dkim_result", "spf_result", "dmarc_aligned",
		"header_from", "envelope_from", "dkim_domain", "dkim_selector", "spf_domain",
	}
	if err := csvWriter.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write each record as a row
	for _, record := range report.Records {
		row := []string{
			report.ReportMetadata.ReportID,
			report.ReportMetadata.OrgName,
			report.ReportMetadata.OrgEmail,
			report.ReportMetadata.BeginDate.Format(time.RFC3339),
			report.ReportMetadata.EndDate.Format(time.RFC3339),
			report.PolicyPublished.Domain,
			report.PolicyPublished.ADKIM,
			report.PolicyPublished.ASPF,
			report.PolicyPublished.P,
			report.PolicyPublished.SP,
			report.PolicyPublished.PCT,
			record.Source.IPAddress,
			record.Source.Country,
			record.Source.ReverseDNS,
			strconv.Itoa(record.Count),
			record.PolicyEvaluated.Disposition,
			record.PolicyEvaluated.DKIM,
			record.PolicyEvaluated.SPF,
			strconv.FormatBool(record.Alignment.DMARC),
			record.Identifiers.HeaderFrom,
			stringPtrToString(record.Identifiers.EnvelopeFrom),
			getDKIMDomain(record.AuthResults.DKIM),
			getDKIMSelector(record.AuthResults.DKIM),
			getSPFDomain(record.AuthResults.SPF),
		}

		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	d.logger.Info("Wrote aggregate report", zap.String("file", filePath))

	// Send via SMTP if configured
	if d.smtpSender != nil {
		if err := d.smtpSender.SendAggregateReport(report); err != nil {
			d.logger.Error("Failed to send aggregate report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if d.kafkaSender != nil {
		if err := d.kafkaSender.SendAggregateReport(report); err != nil {
			d.logger.Error("Failed to send aggregate report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (d *DirectoryCSVWriter) WriteForensicReport(report *parser.ForensicReport) error {
	filename := d.generateForensicFilename(report, "csv")
	filePath := filepath.Join(d.outputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file %s: %w", filePath, err)
	}
	defer file.Close()

	csvWriter := csv.NewWriter(file)
	defer csvWriter.Flush()

	// Write headers
	headers := []string{
		"feedback_type", "user_agent", "version", "original_envelope_id",
		"original_mail_from", "original_rcpt_to", "arrival_date", "subject",
		"message_id", "authentication_results", "dkim_domain", "source_ip",
		"source_country", "delivery_result", "auth_failure", "reported_domain",
	}
	if err := csvWriter.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV headers: %w", err)
	}

	row := []string{
		report.FeedbackType,
		stringPtrToString(report.UserAgent),
		stringPtrToString(report.Version),
		stringPtrToString(report.OriginalEnvelopeID),
		stringPtrToString(report.OriginalMailFrom),
		stringPtrToString(report.OriginalRcptTo),
		report.ArrivalDate.Format(time.RFC3339),
		report.Subject,
		report.MessageID,
		report.AuthenticationResults,
		stringPtrToString(report.DKIMDomain),
		report.Source.IPAddress,
		report.Source.Country,
		report.DeliveryResult,
		strings.Join(report.AuthFailure, ";"),
		report.ReportedDomain,
	}

	if err := csvWriter.Write(row); err != nil {
		return fmt.Errorf("failed to write CSV row: %w", err)
	}

	d.logger.Info("Wrote forensic report", zap.String("file", filePath))

	// Send via SMTP if configured
	if d.smtpSender != nil {
		if err := d.smtpSender.SendForensicReport(report); err != nil {
			d.logger.Error("Failed to send forensic report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if d.kafkaSender != nil {
		if err := d.kafkaSender.SendForensicReport(report); err != nil {
			d.logger.Error("Failed to send forensic report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (d *DirectoryCSVWriter) WriteSMTPTLSReport(report *parser.SMTPTLSReport) error {
	filename := d.generateSMTPTLSFilename(report, "csv")
	filePath := filepath.Join(d.outputDir, filename)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file %s: %w", filePath, err)
	}
	defer file.Close()

	csvWriter := csv.NewWriter(file)
	defer csvWriter.Flush()

	// Write headers
	headers := []string{
		"organization_name", "begin_date", "end_date", "contact_info", "report_id",
		"policy_domain", "policy_type", "successful_session_count", "failed_session_count",
		"failure_result_type", "failure_sending_mta_ip", "failure_receiving_ip",
	}
	if err := csvWriter.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write each policy as rows
	for _, policy := range report.Policies {
		// Base row for policy
		baseRow := []string{
			report.OrganizationName,
			report.BeginDate.Format(time.RFC3339),
			report.EndDate.Format(time.RFC3339),
			report.ContactInfo,
			report.ReportID,
			policy.PolicyDomain,
			policy.PolicyType,
			strconv.Itoa(policy.SuccessfulSessionCount),
			strconv.Itoa(policy.FailedSessionCount),
			"", // failure_result_type (filled below)
			"", // failure_sending_mta_ip (filled below)
			"", // failure_receiving_ip (filled below)
		}

		if len(policy.FailureDetails) == 0 {
			// Write row without failure details
			if err := csvWriter.Write(baseRow); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
		} else {
			// Write one row per failure detail
			for _, failure := range policy.FailureDetails {
				row := make([]string, len(baseRow))
				copy(row, baseRow)
				row[9] = failure.ResultType                       // failure_result_type
				row[10] = stringPtrToString(failure.SendingMTAIP) // failure_sending_mta_ip
				row[11] = stringPtrToString(failure.ReceivingIP)  // failure_receiving_ip

				if err := csvWriter.Write(row); err != nil {
					return fmt.Errorf("failed to write CSV row: %w", err)
				}
			}
		}
	}

	d.logger.Info("Wrote SMTP TLS report", zap.String("file", filePath))

	// Send via SMTP if configured
	if d.smtpSender != nil {
		if err := d.smtpSender.SendSMTPTLSReport(report); err != nil {
			d.logger.Error("Failed to send SMTP TLS report via SMTP", zap.Error(err))
		}
	}

	// Send via Kafka if configured
	if d.kafkaSender != nil {
		if err := d.kafkaSender.SendSMTPTLSReport(report); err != nil {
			d.logger.Error("Failed to send SMTP TLS report via Kafka", zap.Error(err))
		}
	}

	return nil
}

func (d *DirectoryCSVWriter) Close() error {
	return nil
}

// Filename generation methods for DirectoryJSONWriter
func (d *DirectoryJSONWriter) generateAggregateFilename(report *parser.AggregateReport, ext string) string {
	return d.generateFilename("aggregate", report.ReportMetadata.ReportID, report.ReportMetadata.BeginDate, ext)
}

func (d *DirectoryJSONWriter) generateForensicFilename(report *parser.ForensicReport, ext string) string {
	// Use message ID hash for unique filename
	hash := sha256.Sum256([]byte(report.MessageID))
	id := fmt.Sprintf("%x", hash[:8])
	return d.generateFilename("forensic", id, report.ArrivalDate, ext)
}

func (d *DirectoryJSONWriter) generateSMTPTLSFilename(report *parser.SMTPTLSReport, ext string) string {
	return d.generateFilename("smtp_tls", report.ReportID, report.BeginDate, ext)
}

func (d *DirectoryJSONWriter) generateFilename(reportType, id string, timestamp time.Time, ext string) string {
	return fmt.Sprintf("%s_%s_%s.%s", reportType, timestamp.Format("20060102_150405"), id, ext)
}

// Filename generation methods for DirectoryCSVWriter
func (d *DirectoryCSVWriter) generateAggregateFilename(report *parser.AggregateReport, ext string) string {
	return d.generateFilename("aggregate", report.ReportMetadata.ReportID, report.ReportMetadata.BeginDate, ext)
}

func (d *DirectoryCSVWriter) generateForensicFilename(report *parser.ForensicReport, ext string) string {
	// Use message ID hash for unique filename
	hash := sha256.Sum256([]byte(report.MessageID))
	id := fmt.Sprintf("%x", hash[:8])
	return d.generateFilename("forensic", id, report.ArrivalDate, ext)
}

func (d *DirectoryCSVWriter) generateSMTPTLSFilename(report *parser.SMTPTLSReport, ext string) string {
	return d.generateFilename("smtp_tls", report.ReportID, report.BeginDate, ext)
}

func (d *DirectoryCSVWriter) generateFilename(reportType, id string, timestamp time.Time, ext string) string {
	return fmt.Sprintf("%s_%s_%s.%s", reportType, timestamp.Format("20060102_150405"), id, ext)
}
