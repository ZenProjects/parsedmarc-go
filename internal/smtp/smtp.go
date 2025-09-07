package smtp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"go.uber.org/zap"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/parser"
)

// Client represents an SMTP client for sending email reports
type Client struct {
	config *config.SMTPConfig
	logger *zap.Logger
}

// New creates a new SMTP client
func New(cfg *config.SMTPConfig, logger *zap.Logger) *Client {
	return &Client{
		config: cfg,
		logger: logger,
	}
}

// SendAggregateReport sends an aggregate DMARC report via email
func (c *Client) SendAggregateReport(report *parser.AggregateReport) error {
	if !c.config.Enabled {
		return nil
	}

	// Marshal report to JSON
	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	subject := c.config.Subject
	if subject == "" {
		subject = fmt.Sprintf("DMARC Aggregate Report - %s", report.PolicyPublished.Domain)
	}

	body := c.config.Message
	if body == "" {
		body = fmt.Sprintf("DMARC Aggregate Report for domain %s\n\nReport ID: %s\nOrganization: %s\nDate Range: %s to %s\n\nReport data attached as JSON.",
			report.PolicyPublished.Domain,
			report.ReportMetadata.ReportID,
			report.ReportMetadata.OrgName,
			report.ReportMetadata.BeginDate.Format("2006-01-02"),
			report.ReportMetadata.EndDate.Format("2006-01-02"),
		)
	}

	return c.sendEmail(subject, body, reportData, "dmarc-aggregate.json")
}

// SendForensicReport sends a forensic DMARC report via email
func (c *Client) SendForensicReport(report *parser.ForensicReport) error {
	if !c.config.Enabled {
		return nil
	}

	// Marshal report to JSON
	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	subject := c.config.Subject
	if subject == "" {
		subject = fmt.Sprintf("DMARC Forensic Report - %s", report.ReportedDomain)
	}

	body := c.config.Message
	if body == "" {
		body = fmt.Sprintf("DMARC Forensic Report for domain %s\n\nSubject: %s\nMessage ID: %s\nSource IP: %s\nAuth Failure: %s\n\nReport data attached as JSON.",
			report.ReportedDomain,
			report.Subject,
			report.MessageID,
			report.Source.IPAddress,
			strings.Join(report.AuthFailure, ", "),
		)
	}

	return c.sendEmail(subject, body, reportData, "dmarc-forensic.json")
}

// SendSMTPTLSReport sends an SMTP TLS report via email
func (c *Client) SendSMTPTLSReport(report *parser.SMTPTLSReport) error {
	if !c.config.Enabled {
		return nil
	}

	// Marshal report to JSON
	reportData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	subject := c.config.Subject
	if subject == "" {
		subject = fmt.Sprintf("SMTP TLS Report - %s", report.OrganizationName)
	}

	body := c.config.Message
	if body == "" {
		body = fmt.Sprintf("SMTP TLS Report from %s\n\nReport ID: %s\nDate Range: %s to %s\n\nReport data attached as JSON.",
			report.OrganizationName,
			report.ReportID,
			report.BeginDate.Format("2006-01-02"),
			report.EndDate.Format("2006-01-02"),
		)
	}

	return c.sendEmail(subject, body, reportData, "smtp-tls.json")
}

// sendEmail sends an email with the specified subject, body, and attachment
func (c *Client) sendEmail(subject, body string, attachment []byte, filename string) error {
	if len(c.config.To) == 0 {
		return fmt.Errorf("no recipients configured")
	}

	// Create the email message
	var msg bytes.Buffer

	// Headers
	msg.WriteString(fmt.Sprintf("From: %s\r\n", c.config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(c.config.To, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")

	// Multipart boundary
	boundary := fmt.Sprintf("boundary-%d", time.Now().Unix())
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary))
	msg.WriteString("\r\n")

	// Text part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	msg.WriteString("\r\n\r\n")

	// Attachment part
	if len(attachment) > 0 && filename != "" {
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: application/json\r\n")
		msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=%s\r\n", filename))
		msg.WriteString("Content-Transfer-Encoding: base64\r\n")
		msg.WriteString("\r\n")

		// Base64 encode the attachment
		encoded := encodeBase64(attachment)
		msg.WriteString(encoded)
		msg.WriteString("\r\n")
	}

	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	// Send the email
	var auth smtp.Auth
	if c.config.Username != "" && c.config.Password != "" {
		auth = smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)
	}

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	c.logger.Debug("Sending email via SMTP",
		zap.String("host", c.config.Host),
		zap.Int("port", c.config.Port),
		zap.String("from", c.config.From),
		zap.Strings("to", c.config.To),
		zap.String("subject", subject),
	)

	return smtp.SendMail(addr, auth, c.config.From, c.config.To, msg.Bytes())
}

// encodeBase64 encodes data in base64 with line breaks
func encodeBase64(data []byte) string {
	const lineLength = 76
	encoded := make([]byte, 0, len(data)*4/3+4)

	// Base64 character set
	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	// Process data in 3-byte chunks
	for i := 0; i < len(data); i += 3 {
		chunk := make([]byte, 3)
		chunkLen := copy(chunk, data[i:])

		// Convert 3 bytes to 4 base64 characters
		b1 := chunk[0]
		b2, b3 := byte(0), byte(0)
		if chunkLen > 1 {
			b2 = chunk[1]
		}
		if chunkLen > 2 {
			b3 = chunk[2]
		}

		c1 := charset[b1>>2]
		c2 := charset[((b1&0x03)<<4)|((b2&0xf0)>>4)]
		c3 := charset[((b2&0x0f)<<2)|((b3&0xc0)>>6)]
		c4 := charset[b3&0x3f]

		// Add padding if necessary
		if chunkLen == 1 {
			c3, c4 = '=', '='
		} else if chunkLen == 2 {
			c4 = '='
		}

		encoded = append(encoded, c1, c2, c3, c4)

		// Add line breaks
		if len(encoded)%lineLength == 0 {
			encoded = append(encoded, '\r', '\n')
		}
	}

	// Add final line break if needed
	if len(encoded) > 0 && encoded[len(encoded)-1] != '\n' {
		encoded = append(encoded, '\r', '\n')
	}

	return string(encoded)
}
