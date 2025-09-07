package imap

import (
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/domainaware/parsedmarc-go/internal/config"
	"github.com/domainaware/parsedmarc-go/internal/parser"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"go.uber.org/zap"
)

// Client represents an IMAP client for fetching DMARC reports
type Client struct {
	config config.IMAPConfig
	parser *parser.Parser
	logger *zap.Logger
	client *client.Client
}

// New creates a new IMAP client
func New(cfg config.IMAPConfig, p *parser.Parser, logger *zap.Logger) *Client {
	return &Client{
		config: cfg,
		parser: p,
		logger: logger,
	}
}

// Connect establishes connection to IMAP server
func (c *Client) Connect() error {
	var err error

	address := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	if c.config.TLS {
		tlsConfig := &tls.Config{
			ServerName:         c.config.Host,
			InsecureSkipVerify: c.config.SkipVerify,
		}
		c.client, err = client.DialTLS(address, tlsConfig)
	} else {
		c.client, err = client.Dial(address)
		if err != nil {
			return fmt.Errorf("failed to dial IMAP server: %w", err)
		}

		// Try STARTTLS if available
		if caps, err := c.client.Capability(); err == nil {
			if caps["STARTTLS"] {
				tlsConfig := &tls.Config{
					ServerName:         c.config.Host,
					InsecureSkipVerify: c.config.SkipVerify,
				}
				if err := c.client.StartTLS(tlsConfig); err != nil {
					c.logger.Warn("Failed to start TLS", zap.Error(err))
				}
			}
		}
	}

	if err != nil {
		return fmt.Errorf("failed to connect to IMAP server: %w", err)
	}

	// Login
	if err := c.client.Login(c.config.Username, c.config.Password); err != nil {
		return fmt.Errorf("failed to login to IMAP server: %w", err)
	}

	c.logger.Info("Connected to IMAP server",
		zap.String("host", c.config.Host),
		zap.Int("port", c.config.Port),
		zap.String("username", c.config.Username),
	)

	return nil
}

// Disconnect closes the IMAP connection
func (c *Client) Disconnect() error {
	if c.client != nil {
		if err := c.client.Logout(); err != nil {
			c.logger.Warn("Failed to logout from IMAP server", zap.Error(err))
		}
		return c.client.Close()
	}
	return nil
}

// ProcessMessages processes DMARC reports from mailbox
func (c *Client) ProcessMessages() error {
	// Select mailbox
	status, err := c.client.Select(c.config.Mailbox, false)
	if err != nil {
		return fmt.Errorf("failed to select mailbox %s: %w", c.config.Mailbox, err)
	}

	if status.Messages == 0 {
		c.logger.Info("No messages in mailbox", zap.String("mailbox", c.config.Mailbox))
		return nil
	}

	c.logger.Info("Processing messages",
		zap.String("mailbox", c.config.Mailbox),
		zap.Uint32("count", status.Messages),
	)

	// Search for all messages
	seqSet := new(imap.SeqSet)
	seqSet.AddRange(1, status.Messages)

	// Fetch message headers first to identify DMARC reports
	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- c.client.Fetch(seqSet, []imap.FetchItem{
			imap.FetchEnvelope,
			imap.FetchBodyStructure,
			imap.FetchUid,
		}, messages)
	}()

	var dmarcMessages []uint32

	for msg := range messages {
		if c.isDMARCReport(msg) {
			dmarcMessages = append(dmarcMessages, msg.SeqNum)
			c.logger.Debug("Found DMARC report",
				zap.Uint32("seq", msg.SeqNum),
				zap.String("subject", msg.Envelope.Subject),
			)
		}
	}

	if err := <-done; err != nil {
		return fmt.Errorf("failed to fetch messages: %w", err)
	}

	if len(dmarcMessages) == 0 {
		c.logger.Info("No DMARC reports found")
		return nil
	}

	// Process each DMARC report
	processed := 0
	for _, seqNum := range dmarcMessages {
		if err := c.processMessage(seqNum); err != nil {
			c.logger.Error("Failed to process message",
				zap.Uint32("seq", seqNum),
				zap.Error(err),
			)
		} else {
			processed++
		}
	}

	c.logger.Info("Processed DMARC reports",
		zap.Int("processed", processed),
		zap.Int("total", len(dmarcMessages)),
	)

	return nil
}

// isDMARCReport checks if message is a DMARC report based on subject and structure
func (c *Client) isDMARCReport(msg *imap.Message) bool {
	if msg.Envelope == nil {
		return false
	}

	subject := strings.ToLower(msg.Envelope.Subject)

	// Check for DMARC report keywords in subject
	dmarcKeywords := []string{
		"dmarc",
		"report domain",
		"aggregate report",
		"forensic report",
		"tlsrpt",
	}

	for _, keyword := range dmarcKeywords {
		if strings.Contains(subject, keyword) {
			return true
		}
	}

	// Check body structure for attachments that might contain reports
	if msg.BodyStructure != nil {
		return c.hasReportAttachment(msg.BodyStructure)
	}

	return false
}

// hasReportAttachment recursively checks for report attachments
func (c *Client) hasReportAttachment(bs *imap.BodyStructure) bool {
	if bs == nil {
		return false
	}

	// Check current part
	if bs.MIMEType == "application" {
		switch bs.MIMESubType {
		case "xml", "zip", "gzip", "octet-stream":
			return true
		case "tlsrpt+json", "tlsrpt+gzip":
			return true
		}
	}

	if bs.MIMEType == "text" && bs.MIMESubType == "xml" {
		return true
	}

	// Check child parts
	for _, part := range bs.Parts {
		if c.hasReportAttachment(part) {
			return true
		}
	}

	return false
}

// processMessage fetches and processes a single message
func (c *Client) processMessage(seqNum uint32) error {
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(seqNum)

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)

	go func() {
		done <- c.client.Fetch(seqSet, []imap.FetchItem{
			imap.FetchRFC822,
			imap.FetchUid,
		}, messages)
	}()

	msg := <-messages
	if err := <-done; err != nil {
		return fmt.Errorf("failed to fetch message body: %w", err)
	}

	if msg == nil {
		return fmt.Errorf("message not found")
	}

	// Parse the email
	reader := msg.GetBody(&imap.BodySectionName{})
	if reader == nil {
		return fmt.Errorf("failed to get message body")
	}

	mailReader, err := mail.CreateReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create mail reader: %w", err)
	}

	// Process email parts
	processed := false
	for {
		part, err := mailReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read email part: %w", err)
		}

		if err := c.processEmailPart(part); err != nil {
			c.logger.Warn("Failed to process email part", zap.Error(err))
		} else {
			processed = true
		}
	}

	if processed {
		// Move message to archive or delete if configured
		if err := c.archiveMessage(seqNum, msg.Uid); err != nil {
			c.logger.Warn("Failed to archive message", zap.Error(err))
		}
	}

	return nil
}

// processEmailPart processes an individual email part
func (c *Client) processEmailPart(part *mail.Part) error {
	contentType := part.Header.Get("ContentType")

	if contentType != "" {
		return fmt.Errorf("unsupported content type: %s", contentType)
	}

	// Check if this part contains a DMARC report
	if !c.isReportPart(contentType, params) {
		return nil
	}

	// Read the part content
	data, err := io.ReadAll(part.Body)
	if err != nil {
		return fmt.Errorf("failed to read part body: %w", err)
	}

	// Parse the report using our parser
	return c.parser.ParseData(data)
}

// isReportPart checks if email part contains a DMARC report
func (c *Client) isReportPart(contentType string, params map[string]string) bool {
	switch contentType {
	case "application/xml":
		return true
	case "application/zip":
		return true
	case "application/gzip":
		return true
	case "application/octet-stream":
		// Check filename
		if filename, ok := params["name"]; ok {
			return c.isReportFilename(filename)
		}
		return false
	case "application/tlsrpt+json":
		return true
	case "application/tlsrpt+gzip":
		return true
	case "text/xml":
		return true
	default:
		return false
	}
}

// isReportFilename checks if filename suggests a DMARC report
func (c *Client) isReportFilename(filename string) bool {
	filename = strings.ToLower(filename)
	extensions := []string{".xml", ".zip", ".gz", ".json"}

	for _, ext := range extensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}

	return false
}

// archiveMessage moves message to archive folder or deletes it
func (c *Client) archiveMessage(seqNum, uid uint32) error {
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(seqNum)

	if c.config.DeleteProcessed {
		// Mark for deletion
		flags := []interface{}{imap.DeletedFlag}
		if err := c.client.Store(seqSet, imap.FormatFlagsOp(imap.AddFlags, false), flags, nil); err != nil {
			return fmt.Errorf("failed to mark message for deletion: %w", err)
		}

		// Expunge to actually delete
		if err := c.client.Expunge(nil); err != nil {
			return fmt.Errorf("failed to expunge deleted messages: %w", err)
		}

		c.logger.Debug("Deleted processed message", zap.Uint32("seq", seqNum))
	} else if c.config.ArchiveMailbox != "" && c.config.ArchiveMailbox != c.config.Mailbox {
		// Move to archive folder
		if err := c.client.Move(seqSet, c.config.ArchiveMailbox); err != nil {
			return fmt.Errorf("failed to move message to archive: %w", err)
		}

		c.logger.Debug("Archived processed message",
			zap.Uint32("seq", seqNum),
			zap.String("archive", c.config.ArchiveMailbox),
		)
	}

	return nil
}

// Watch continuously monitors the mailbox for new DMARC reports
func (c *Client) Watch() error {
	for {
		if err := c.ProcessMessages(); err != nil {
			c.logger.Error("Failed to process messages", zap.Error(err))
		}

		c.logger.Debug("Waiting for next check",
			zap.Int("interval", c.config.CheckInterval),
		)

		time.Sleep(time.Duration(c.config.CheckInterval) * time.Second)
	}
}
