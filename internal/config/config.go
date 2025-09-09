package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Logging    LoggingConfig    `mapstructure:"logging"`
	Parser     ParserConfig     `mapstructure:"parser"`
	ClickHouse ClickHouseConfig `mapstructure:"clickhouse"`
	IMAP       IMAPConfig       `mapstructure:"imap"`
	HTTP       HTTPConfig       `mapstructure:"http"`
	SMTP       SMTPConfig       `mapstructure:"smtp"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
}

// ParserConfig contains parser configuration
type ParserConfig struct {
	Offline             bool     `mapstructure:"offline"`
	IPDBPath            string   `mapstructure:"ip_db_path"`
	ReverseDNSMapPath   string   `mapstructure:"reverse_dns_map_path"`
	ReverseDNSMapURL    string   `mapstructure:"reverse_dns_map_url"`
	AlwaysUseLocalFiles bool     `mapstructure:"always_use_local_files"`
	Nameservers         []string `mapstructure:"nameservers"`
	DNSTimeout          int      `mapstructure:"dns_timeout"`
}

// ClickHouseConfig contains ClickHouse configuration
type ClickHouseConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Database   string `mapstructure:"database"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	TLS        bool   `mapstructure:"tls"`
	SkipVerify bool   `mapstructure:"skip_verify"`
}

// IMAPConfig contains IMAP configuration
type IMAPConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	TLS             bool   `mapstructure:"tls"`
	SkipVerify      bool   `mapstructure:"skip_verify"`
	Mailbox         string `mapstructure:"mailbox"`
	ArchiveMailbox  string `mapstructure:"archive_mailbox"`
	DeleteProcessed bool   `mapstructure:"delete_processed"`
	CheckInterval   int    `mapstructure:"check_interval"`
}

// HTTPConfig contains HTTP server configuration
type HTTPConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	TLS           bool   `mapstructure:"tls"`
	CertFile      string `mapstructure:"cert_file"`
	KeyFile       string `mapstructure:"key_file"`
	RateLimit     int    `mapstructure:"rate_limit"`
	RateBurst     int    `mapstructure:"rate_burst"`
	MaxUploadSize int64  `mapstructure:"max_upload_size"`
}

// SMTPConfig contains SMTP configuration for sending email reports
type SMTPConfig struct {
	Enabled    bool     `mapstructure:"enabled"`
	Host       string   `mapstructure:"host"`
	Port       int      `mapstructure:"port"`
	SSL        bool     `mapstructure:"ssl"`
	Username   string   `mapstructure:"username"`
	Password   string   `mapstructure:"password"`
	From       string   `mapstructure:"from"`
	To         []string `mapstructure:"to"`
	Subject    string   `mapstructure:"subject"`
	Attachment string   `mapstructure:"attachment"`
	Message    string   `mapstructure:"message"`
}

// KafkaConfig contains Kafka configuration for sending reports
type KafkaConfig struct {
	Enabled        bool     `mapstructure:"enabled"`
	Hosts          []string `mapstructure:"hosts"`
	Username       string   `mapstructure:"username"`
	Password       string   `mapstructure:"password"`
	SSL            bool     `mapstructure:"ssl"`
	SkipVerify     bool     `mapstructure:"skip_verify"`
	AggregateTopic string   `mapstructure:"aggregate_topic"`
	ForensicTopic  string   `mapstructure:"forensic_topic"`
	SMTPTLSTopic   string   `mapstructure:"smtp_tls_topic"`
}

// Load loads configuration from file, using defaults if file doesn't exist
func Load(configFile string) (*Config, error) {
	v := viper.New()

	// Set defaults first
	setDefaults(v)

	// Enable environment variable reading
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Try to read config file if it exists
	if configFile != "" {
		v.SetConfigFile(configFile)
		v.SetConfigType("yaml")

		// Only return error if file exists but can't be read/parsed
		if err := v.ReadInConfig(); err != nil {
			// Check if it's just a file not found error
			if !isFileNotFoundError(err) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
			// If file doesn't exist, just continue with defaults
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// LoadDefault loads configuration with default values only
func LoadDefault() *Config {
	v := viper.New()
	setDefaults(v)

	// Enable environment variable reading
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		// This should not happen with default configuration, but handle gracefully
		return &Config{
			Logging: LoggingConfig{
				Level:  "info",
				Format: "json",
			},
			Parser: ParserConfig{
				Offline:    false,
				DNSTimeout: 5,
			},
		}
	}
	return &cfg
}

// isFileNotFoundError checks if the error is a file not found error
func isFileNotFoundError(err error) bool {
	errMsg := err.Error()
	return strings.Contains(errMsg, "no such file or directory") ||
		strings.Contains(errMsg, "cannot find the file") ||
		strings.Contains(errMsg, "system cannot find the file")
}

func setDefaults(v *viper.Viper) {
	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output_path", "stdout")

	// Parser defaults
	v.SetDefault("parser.offline", false)
	v.SetDefault("parser.always_use_local_files", false)
	v.SetDefault("parser.nameservers", []string{"1.1.1.1", "1.0.0.1"})
	v.SetDefault("parser.dns_timeout", 2)

	// ClickHouse defaults
	v.SetDefault("clickhouse.enabled", false)
	v.SetDefault("clickhouse.host", "localhost")
	v.SetDefault("clickhouse.port", 9000)
	v.SetDefault("clickhouse.database", "dmarc")
	v.SetDefault("clickhouse.username", "default")
	v.SetDefault("clickhouse.password", "")
	v.SetDefault("clickhouse.tls", false)
	v.SetDefault("clickhouse.skip_verify", false)

	// IMAP defaults
	v.SetDefault("imap.enabled", false)
	v.SetDefault("imap.host", "")
	v.SetDefault("imap.port", 993)
	v.SetDefault("imap.username", "")
	v.SetDefault("imap.password", "")
	v.SetDefault("imap.tls", true)
	v.SetDefault("imap.skip_verify", false)
	v.SetDefault("imap.mailbox", "INBOX")
	v.SetDefault("imap.archive_mailbox", "DMARC-Archive")
	v.SetDefault("imap.delete_processed", false)
	v.SetDefault("imap.check_interval", 300) // 5 minutes

	// HTTP defaults
	v.SetDefault("http.enabled", false)
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.tls", false)
	v.SetDefault("http.cert_file", "")
	v.SetDefault("http.key_file", "")
	v.SetDefault("http.rate_limit", 60)                // requests per minute
	v.SetDefault("http.rate_burst", 10)                // burst capacity
	v.SetDefault("http.max_upload_size", 50*1024*1024) // 50MB

	// SMTP defaults
	v.SetDefault("smtp.enabled", false)
	v.SetDefault("smtp.host", "")
	v.SetDefault("smtp.port", 25)
	v.SetDefault("smtp.ssl", false)
	v.SetDefault("smtp.username", "")
	v.SetDefault("smtp.password", "")
	v.SetDefault("smtp.from", "")
	v.SetDefault("smtp.to", []string{})
	v.SetDefault("smtp.subject", "parsedmarc report")
	v.SetDefault("smtp.attachment", "")
	v.SetDefault("smtp.message", "")

	// Kafka defaults
	v.SetDefault("kafka.enabled", false)
	v.SetDefault("kafka.hosts", []string{})
	v.SetDefault("kafka.username", "")
	v.SetDefault("kafka.password", "")
	v.SetDefault("kafka.ssl", true)
	v.SetDefault("kafka.skip_verify", false)
	v.SetDefault("kafka.aggregate_topic", "")
	v.SetDefault("kafka.forensic_topic", "")
	v.SetDefault("kafka.smtp_tls_topic", "")
}
