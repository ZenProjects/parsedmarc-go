package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.uber.org/zap"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/parser"
)

// Storage implements ClickHouse storage for DMARC reports
type Storage struct {
	conn   driver.Conn
	logger *zap.Logger
}

// New creates a new ClickHouse storage instance
func New(cfg config.ClickHouseConfig, logger *zap.Logger) (*Storage, error) {
	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout:      30 * time.Second,
		MaxOpenConns:     10,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Hour,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	}

	if cfg.TLS {
		options.TLS = &tls.Config{
			InsecureSkipVerify: cfg.SkipVerify,
		}
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	storage := &Storage{
		conn:   conn,
		logger: logger,
	}

	// Create tables if they don't exist
	if err := storage.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return storage, nil
}

// Close closes the ClickHouse connection
func (s *Storage) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// createTables creates the necessary tables for storing DMARC reports
func (s *Storage) createTables() error {
	ctx := context.Background()

	// Create aggregate reports table
	aggregateTableSQL := `
	CREATE TABLE IF NOT EXISTS dmarc_aggregate_reports (
		id UUID DEFAULT generateUUIDv4(),
		xml_schema String,
		org_name String,
		org_email String,
		org_extra_contact_info Nullable(String),
		report_id String,
		begin_date DateTime,
		end_date DateTime,
		errors Array(String),
		domain String,
		adkim String,
		aspf String,
		p String,
		sp String,
		pct String,
		fo String,
		created_at DateTime DEFAULT now()
	) ENGINE = MergeTree()
	ORDER BY (org_name, report_id, begin_date)
	PARTITION BY toYYYYMM(begin_date)`

	if err := s.conn.Exec(ctx, aggregateTableSQL); err != nil {
		return fmt.Errorf("failed to create aggregate reports table: %w", err)
	}

	// Create records table
	recordsTableSQL := `
	CREATE TABLE IF NOT EXISTS dmarc_aggregate_records (
		id UUID DEFAULT generateUUIDv4(),
		report_id String,
		org_name String,
		source_ip_address String,
		source_country String,
		source_reverse_dns String,
		source_base_domain String,
		source_name String,
		source_type String,
		count UInt32,
		spf_aligned UInt8,
		dkim_aligned UInt8,
		dmarc_aligned UInt8,
		disposition String,
		policy_override_reasons Array(String),
		policy_override_comments Array(String),
		envelope_from Nullable(String),
		header_from String,
		envelope_to Nullable(String),
		dkim_domains Array(String),
		dkim_selectors Array(String),
		dkim_results Array(String),
		spf_domains Array(String),
		spf_scopes Array(String),
		spf_results Array(String),
		begin_date DateTime,
		created_at DateTime DEFAULT now()
	) ENGINE = MergeTree()
	ORDER BY (org_name, report_id, source_ip_address, begin_date)
	PARTITION BY toYYYYMM(begin_date)`

	if err := s.conn.Exec(ctx, recordsTableSQL); err != nil {
		return fmt.Errorf("failed to create records table: %w", err)
	}

	// Create forensic reports table
	forensicTableSQL := `
	CREATE TABLE IF NOT EXISTS dmarc_forensic_reports (
		id UUID DEFAULT generateUUIDv4(),
		feedback_type String,
		user_agent Nullable(String),
		version Nullable(String),
		original_envelope_id Nullable(String),
		original_mail_from Nullable(String),
		original_rcpt_to Nullable(String),
		arrival_date DateTime,
		arrival_date_utc DateTime,
		subject String,
		message_id String,
		authentication_results String,
		dkim_domain Nullable(String),
		source_ip_address String,
		source_country String,
		source_reverse_dns String,
		source_base_domain String,
		source_name String,
		source_type String,
		delivery_result String,
		auth_failure Array(String),
		reported_domain String,
		authentication_mechanisms Array(String),
		sample_headers_only UInt8,
		sample String,
		parsed_sample String,
		created_at DateTime DEFAULT now()
	) ENGINE = MergeTree()
	ORDER BY (arrival_date, source_ip_address)
	PARTITION BY toYYYYMM(arrival_date)`

	if err := s.conn.Exec(ctx, forensicTableSQL); err != nil {
		return fmt.Errorf("failed to create forensic reports table: %w", err)
	}

	// Create SMTP TLS reports table
	smtpTLSTableSQL := `
	CREATE TABLE IF NOT EXISTS dmarc_smtp_tls_reports (
		id UUID DEFAULT generateUUIDv4(),
		organization_name String,
		begin_date DateTime,
		end_date DateTime,
		contact_info String,
		report_id String,
		policy_domain String,
		policy_type String,
		policy_strings Array(String),
		mx_host_patterns Array(String),
		successful_session_count UInt64,
		failed_session_count UInt64,
		created_at DateTime DEFAULT now(),
		INDEX idx_report_id report_id TYPE bloom_filter GRANULARITY 1,
		INDEX idx_org_name organization_name TYPE bloom_filter GRANULARITY 1,
		INDEX idx_policy_domain policy_domain TYPE bloom_filter GRANULARITY 1
	) ENGINE = MergeTree()
	ORDER BY (begin_date, organization_name)
	PARTITION BY toYYYYMM(begin_date)`

	if err := s.conn.Exec(ctx, smtpTLSTableSQL); err != nil {
		return fmt.Errorf("failed to create SMTP TLS reports table: %w", err)
	}

	// Create SMTP TLS failure details table
	smtpTLSFailuresTableSQL := `
	CREATE TABLE IF NOT EXISTS dmarc_smtp_tls_failures (
		id UUID DEFAULT generateUUIDv4(),
		report_id String,
		policy_domain String,
		result_type String,
		failed_session_count UInt64,
		sending_mta_ip Nullable(String),
		receiving_ip Nullable(String),
		receiving_mx_hostname Nullable(String),
		receiving_mx_helo Nullable(String),
		additional_info_uri Nullable(String),
		failure_reason_code Nullable(String),
		created_at DateTime DEFAULT now(),
		INDEX idx_report_id report_id TYPE bloom_filter GRANULARITY 1,
		INDEX idx_policy_domain policy_domain TYPE bloom_filter GRANULARITY 1
	) ENGINE = MergeTree()
	ORDER BY (report_id, result_type)
	PARTITION BY toYYYYMM(created_at)`

	if err := s.conn.Exec(ctx, smtpTLSFailuresTableSQL); err != nil {
		return fmt.Errorf("failed to create SMTP TLS failures table: %w", err)
	}

	s.logger.Info("ClickHouse tables created successfully")
	return nil
}

// StoreAggregateReport stores an aggregate DMARC report in ClickHouse
func (s *Storage) StoreAggregateReport(report *parser.AggregateReport) error {
	ctx := context.Background()

	// Store the main report record
	reportSQL := `
	INSERT INTO dmarc_aggregate_reports (
		xml_schema, org_name, org_email, org_extra_contact_info, report_id,
		begin_date, end_date, errors, domain, adkim, aspf, p, sp, pct, fo
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	err := s.conn.Exec(ctx, reportSQL,
		report.XMLSchema,
		report.ReportMetadata.OrgName,
		report.ReportMetadata.OrgEmail,
		report.ReportMetadata.OrgExtraContactInfo,
		report.ReportMetadata.ReportID,
		report.ReportMetadata.BeginDate,
		report.ReportMetadata.EndDate,
		report.ReportMetadata.Errors,
		report.PolicyPublished.Domain,
		report.PolicyPublished.ADKIM,
		report.PolicyPublished.ASPF,
		report.PolicyPublished.P,
		report.PolicyPublished.SP,
		report.PolicyPublished.PCT,
		report.PolicyPublished.FO,
	)
	if err != nil {
		return fmt.Errorf("failed to insert aggregate report: %w", err)
	}

	// Store individual records
	if len(report.Records) > 0 {
		batch, err := s.conn.PrepareBatch(ctx, `
		INSERT INTO dmarc_aggregate_records (
			report_id, org_name, source_ip_address, source_country, source_reverse_dns,
			source_base_domain, source_name, source_type, count, spf_aligned,
			dkim_aligned, dmarc_aligned, disposition, policy_override_reasons,
			policy_override_comments, envelope_from, header_from, envelope_to,
			dkim_domains, dkim_selectors, dkim_results, spf_domains, spf_scopes,
			spf_results, begin_date
		)`)
		if err != nil {
			return fmt.Errorf("failed to prepare batch: %w", err)
		}

		for _, record := range report.Records {
			// Convert policy override reasons
			var reasons, comments []string
			for _, reason := range record.PolicyEvaluated.PolicyOverrideReasons {
				if reason.Type != nil {
					reasons = append(reasons, *reason.Type)
				} else {
					reasons = append(reasons, "none")
				}
				if reason.Comment != nil {
					comments = append(comments, *reason.Comment)
				} else {
					comments = append(comments, "none")
				}
			}

			// Convert auth results
			var dkimDomains, dkimSelectors, dkimResults []string
			for _, dkim := range record.AuthResults.DKIM {
				dkimDomains = append(dkimDomains, dkim.Domain)
				dkimSelectors = append(dkimSelectors, dkim.Selector)
				dkimResults = append(dkimResults, dkim.Result)
			}

			var spfDomains, spfScopes, spfResults []string
			for _, spf := range record.AuthResults.SPF {
				spfDomains = append(spfDomains, spf.Domain)
				spfScopes = append(spfScopes, spf.Scope)
				spfResults = append(spfResults, spf.Result)
			}

			err := batch.Append(
				report.ReportMetadata.ReportID,
				report.ReportMetadata.OrgName,
				record.Source.IPAddress,
				record.Source.Country,
				record.Source.ReverseDNS,
				record.Source.BaseDomain,
				record.Source.Name,
				record.Source.Type,
				record.Count,
				boolToUint8(record.Alignment.SPF),
				boolToUint8(record.Alignment.DKIM),
				boolToUint8(record.Alignment.DMARC),
				record.PolicyEvaluated.Disposition,
				reasons,
				comments,
				record.Identifiers.EnvelopeFrom,
				record.Identifiers.HeaderFrom,
				record.Identifiers.EnvelopeTo,
				dkimDomains,
				dkimSelectors,
				dkimResults,
				spfDomains,
				spfScopes,
				spfResults,
				report.ReportMetadata.BeginDate,
			)
			if err != nil {
				return fmt.Errorf("failed to append record to batch: %w", err)
			}
		}

		if err := batch.Send(); err != nil {
			return fmt.Errorf("failed to send batch: %w", err)
		}
	}

	s.logger.Info("Stored aggregate report in ClickHouse",
		zap.String("org", report.ReportMetadata.OrgName),
		zap.String("report_id", report.ReportMetadata.ReportID),
		zap.Int("records", len(report.Records)),
	)

	return nil
}

// StoreForensicReport stores a forensic DMARC report in ClickHouse
func (s *Storage) StoreForensicReport(report *parser.ForensicReport) error {
	ctx := context.Background()

	reportSQL := `
	INSERT INTO dmarc_forensic_reports (
		feedback_type, user_agent, version, original_envelope_id, original_mail_from,
		original_rcpt_to, arrival_date, arrival_date_utc, subject, message_id,
		authentication_results, dkim_domain, source_ip_address, source_country,
		source_reverse_dns, source_base_domain, source_name, source_type,
		delivery_result, auth_failure, reported_domain, authentication_mechanisms,
		sample_headers_only, sample, parsed_sample
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	err := s.conn.Exec(ctx, reportSQL,
		report.FeedbackType,
		report.UserAgent,
		report.Version,
		report.OriginalEnvelopeID,
		report.OriginalMailFrom,
		report.OriginalRcptTo,
		report.ArrivalDate,
		report.ArrivalDateUTC,
		report.Subject,
		report.MessageID,
		report.AuthenticationResults,
		report.DKIMDomain,
		report.Source.IPAddress,
		report.Source.Country,
		report.Source.ReverseDNS,
		report.Source.BaseDomain,
		report.Source.Name,
		report.Source.Type,
		report.DeliveryResult,
		report.AuthFailure,
		report.ReportedDomain,
		report.AuthenticationMechanisms,
		boolToUint8(report.SampleHeadersOnly),
		report.Sample,
		string(report.ParsedSample),
	)
	if err != nil {
		return fmt.Errorf("failed to insert forensic report: %w", err)
	}

	s.logger.Info("Stored forensic report in ClickHouse",
		zap.String("subject", report.Subject),
		zap.String("source_ip", report.Source.IPAddress),
	)

	return nil
}

// StoreSMTPTLSReport stores an SMTP TLS report in ClickHouse
func (s *Storage) StoreSMTPTLSReport(report *parser.SMTPTLSReport) error {
	ctx := context.Background()

	// Insert main report
	reportSQL := `
	INSERT INTO dmarc_smtp_tls_reports (
		organization_name, begin_date, end_date, contact_info, report_id,
		policy_domain, policy_type, policy_strings, mx_host_patterns,
		successful_session_count, failed_session_count
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// For simplicity, we'll store the first policy's data in the main table
	// In a production system, you might want separate tables for policies
	var policyDomain, policyType string
	var policyStrings, mxHostPatterns []string
	var successfulCount, failedCount int

	if len(report.Policies) > 0 {
		policy := report.Policies[0]
		policyDomain = policy.PolicyDomain
		policyType = policy.PolicyType
		policyStrings = policy.PolicyStrings
		mxHostPatterns = policy.MXHostPatterns
		successfulCount = policy.SuccessfulSessionCount
		failedCount = policy.FailedSessionCount
	}

	err := s.conn.Exec(ctx, reportSQL,
		report.OrganizationName,
		report.BeginDate,
		report.EndDate,
		report.ContactInfo,
		report.ReportID,
		policyDomain,
		policyType,
		policyStrings,
		mxHostPatterns,
		successfulCount,
		failedCount,
	)
	if err != nil {
		return fmt.Errorf("failed to insert SMTP TLS report: %w", err)
	}

	// Insert failure details for all policies
	if len(report.Policies) > 0 {
		failureSQL := `
		INSERT INTO dmarc_smtp_tls_failures (
			report_id, policy_domain, result_type, failed_session_count,
			sending_mta_ip, receiving_ip, receiving_mx_hostname, receiving_mx_helo,
			additional_info_uri, failure_reason_code
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

		for _, policy := range report.Policies {
			for _, failure := range policy.FailureDetails {
				err := s.conn.Exec(ctx, failureSQL,
					report.ReportID,
					policy.PolicyDomain,
					failure.ResultType,
					failure.FailedSessionCount,
					failure.SendingMTAIP,
					failure.ReceivingIP,
					failure.ReceivingMXHostname,
					failure.ReceivingMXHelo,
					failure.AdditionalInfoURI,
					failure.FailureReasonCode,
				)
				if err != nil {
					return fmt.Errorf("failed to insert SMTP TLS failure detail: %w", err)
				}
			}
		}
	}

	s.logger.Info("Stored SMTP TLS report in ClickHouse",
		zap.String("org", report.OrganizationName),
		zap.String("report_id", report.ReportID),
		zap.Int("policies", len(report.Policies)),
	)

	return nil
}

// boolToUint8 converts boolean to uint8 for ClickHouse
func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
