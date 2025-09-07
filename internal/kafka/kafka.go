package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	//	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"go.uber.org/zap"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/parser"
)

// Client represents a Kafka client for sending reports
type Client struct {
	config *config.KafkaConfig
	logger *zap.Logger
}

// New creates a new Kafka client
func New(cfg *config.KafkaConfig, logger *zap.Logger) *Client {
	return &Client{
		config: cfg,
		logger: logger,
	}
}

// SendAggregateReport sends an aggregate DMARC report to Kafka
func (c *Client) SendAggregateReport(report *parser.AggregateReport) error {
	if !c.config.Enabled || c.config.AggregateTopic == "" {
		return nil
	}

	// Marshal report to JSON
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal aggregate report: %w", err)
	}

	// Create message
	msg := kafka.Message{
		Key:   []byte(report.ReportMetadata.ReportID),
		Value: data,
		Time:  time.Now(),
		Headers: []kafka.Header{
			{Key: "type", Value: []byte("aggregate")},
			{Key: "domain", Value: []byte(report.PolicyPublished.Domain)},
			{Key: "org", Value: []byte(report.ReportMetadata.OrgName)},
		},
	}

	c.logger.Debug("Sending aggregate report to Kafka",
		zap.String("topic", c.config.AggregateTopic),
		zap.String("report_id", report.ReportMetadata.ReportID),
		zap.String("domain", report.PolicyPublished.Domain),
	)

	return c.sendMessage(c.config.AggregateTopic, msg)
}

// SendForensicReport sends a forensic DMARC report to Kafka
func (c *Client) SendForensicReport(report *parser.ForensicReport) error {
	if !c.config.Enabled || c.config.ForensicTopic == "" {
		return nil
	}

	// Marshal report to JSON
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal forensic report: %w", err)
	}

	// Create message key from message ID and timestamp
	key := fmt.Sprintf("%s-%d", report.MessageID, report.ArrivalDate.Unix())

	// Create message
	msg := kafka.Message{
		Key:   []byte(key),
		Value: data,
		Time:  time.Now(),
		Headers: []kafka.Header{
			{Key: "type", Value: []byte("forensic")},
			{Key: "domain", Value: []byte(report.ReportedDomain)},
			{Key: "source_ip", Value: []byte(report.Source.IPAddress)},
		},
	}

	c.logger.Debug("Sending forensic report to Kafka",
		zap.String("topic", c.config.ForensicTopic),
		zap.String("key", key),
		zap.String("domain", report.ReportedDomain),
	)

	return c.sendMessage(c.config.ForensicTopic, msg)
}

// SendSMTPTLSReport sends an SMTP TLS report to Kafka
func (c *Client) SendSMTPTLSReport(report *parser.SMTPTLSReport) error {
	if !c.config.Enabled || c.config.SMTPTLSTopic == "" {
		return nil
	}

	// Marshal report to JSON
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal SMTP TLS report: %w", err)
	}

	// Create message
	msg := kafka.Message{
		Key:   []byte(report.ReportID),
		Value: data,
		Time:  time.Now(),
		Headers: []kafka.Header{
			{Key: "type", Value: []byte("smtp_tls")},
			{Key: "org", Value: []byte(report.OrganizationName)},
		},
	}

	c.logger.Debug("Sending SMTP TLS report to Kafka",
		zap.String("topic", c.config.SMTPTLSTopic),
		zap.String("report_id", report.ReportID),
		zap.String("org", report.OrganizationName),
	)

	return c.sendMessage(c.config.SMTPTLSTopic, msg)
}

// sendMessage sends a message to the specified Kafka topic
func (c *Client) sendMessage(topic string, msg kafka.Message) error {
	// Validate that we have hosts configured
	if len(c.config.Hosts) == 0 {
		return fmt.Errorf("no Kafka brokers configured")
	}

	// Create writer configuration
	writerConfig := kafka.WriterConfig{
		Brokers:  c.config.Hosts,
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}

	// Configure TLS if enabled
	if c.config.SSL {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: c.config.SkipVerify,
		}
		writerConfig.Dialer = &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			TLS:       tlsConfig,
		}
	}

	// Configure SASL authentication if credentials are provided
	if c.config.Username != "" && c.config.Password != "" {
		mechanism := plain.Mechanism{
			Username: c.config.Username,
			Password: c.config.Password,
		}

		if writerConfig.Dialer == nil {
			writerConfig.Dialer = &kafka.Dialer{
				Timeout:   10 * time.Second,
				DualStack: true,
			}
		}
		writerConfig.Dialer.SASLMechanism = mechanism
	}

	// Create writer
	writer := kafka.NewWriter(writerConfig)
	defer writer.Close()

	// Send message with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := writer.WriteMessages(ctx, msg)
	if err != nil {
		c.logger.Error("Failed to send message to Kafka",
			zap.String("topic", topic),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send message to Kafka topic %s: %w", topic, err)
	}

	c.logger.Debug("Successfully sent message to Kafka",
		zap.String("topic", topic),
		zap.String("key", string(msg.Key)),
	)

	return nil
}

// TestConnection tests the connection to Kafka brokers
func (c *Client) TestConnection() error {
	if !c.config.Enabled || len(c.config.Hosts) == 0 {
		return fmt.Errorf("Kafka not enabled or no hosts configured")
	}

	// Create a simple connection test using a reader
	readerConfig := kafka.ReaderConfig{
		Brokers: c.config.Hosts,
		Topic:   "test-connection",
		GroupID: "parsedmarc-connection-test",
	}

	// Configure TLS if enabled
	if c.config.SSL {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: c.config.SkipVerify,
		}
		readerConfig.Dialer = &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			TLS:       tlsConfig,
		}
	}

	// Configure SASL authentication if credentials are provided
	if c.config.Username != "" && c.config.Password != "" {
		mechanism := plain.Mechanism{
			Username: c.config.Username,
			Password: c.config.Password,
		}

		if readerConfig.Dialer == nil {
			readerConfig.Dialer = &kafka.Dialer{
				Timeout:   10 * time.Second,
				DualStack: true,
			}
		}
		readerConfig.Dialer.SASLMechanism = mechanism
	}

	reader := kafka.NewReader(readerConfig)
	defer reader.Close()

	// Try to connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Just try to fetch metadata to test connection
	_, err := reader.FetchMessage(ctx)
	if err != nil {
		// We expect this to fail since we're using a test topic
		// But if we get a connection-related error, return it
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("connection timeout to Kafka brokers")
		}
		// Other errors might be expected (like topic not found), so we consider the connection working
		c.logger.Debug("Kafka connection test completed", zap.Error(err))
	}

	return nil
}
