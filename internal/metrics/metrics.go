package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ParserMetrics contains metrics for the parser
type ParserMetrics struct {
	ParsedReportsTotal   *prometheus.CounterVec
	ParseFailuresTotal   *prometheus.CounterVec
	ParseDurationSeconds *prometheus.HistogramVec
	ReportSizeBytes      prometheus.Histogram
}

// IMAPMetrics contains metrics for IMAP client
type IMAPMetrics struct {
	ConnectionAttemptsTotal *prometheus.CounterVec
	MessagesProcessedTotal  *prometheus.CounterVec
	ConnectionDuration      prometheus.Histogram
	LastCheckTimestamp      prometheus.Gauge
}

// NewParserMetrics creates new parser metrics
func NewParserMetrics() *ParserMetrics {
	metrics := &ParserMetrics{
		ParsedReportsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "parsedmarc_parser_reports_total",
				Help: "Total number of reports parsed",
			},
			[]string{"type", "source"},
		),
		ParseFailuresTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "parsedmarc_parser_failures_total",
				Help: "Total number of parsing failures",
			},
			[]string{"type", "source", "reason"},
		),
		ParseDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "parsedmarc_parser_duration_seconds",
				Help:    "Time spent parsing reports",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0},
			},
			[]string{"type", "source"},
		),
		ReportSizeBytes: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "parsedmarc_parser_report_size_bytes",
				Help:    "Size of parsed reports in bytes",
				Buckets: []float64{1024, 4096, 16384, 65536, 262144, 1048576, 4194304},
			},
		),
	}

	// Only register if not already registered (to avoid test conflicts)
	registry := prometheus.DefaultRegisterer
	if err := registry.Register(metrics.ParsedReportsTotal); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := registry.Register(metrics.ParseFailuresTotal); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := registry.Register(metrics.ParseDurationSeconds); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := registry.Register(metrics.ReportSizeBytes); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}

	return metrics
}

// NewIMAPMetrics creates new IMAP metrics
func NewIMAPMetrics() *IMAPMetrics {
	metrics := &IMAPMetrics{
		ConnectionAttemptsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "parsedmarc_imap_connections_total",
				Help: "Total number of IMAP connection attempts",
			},
			[]string{"status"},
		),
		MessagesProcessedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "parsedmarc_imap_messages_total",
				Help: "Total number of IMAP messages processed",
			},
			[]string{"action", "status"},
		),
		ConnectionDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "parsedmarc_imap_connection_duration_seconds",
				Help:    "Time spent connected to IMAP server",
				Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
			},
		),
		LastCheckTimestamp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "parsedmarc_imap_last_check_timestamp_seconds",
				Help: "Timestamp of last IMAP check",
			},
		),
	}

	// Only register if not already registered (to avoid test conflicts)
	registry := prometheus.DefaultRegisterer
	if err := registry.Register(metrics.ConnectionAttemptsTotal); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := registry.Register(metrics.MessagesProcessedTotal); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := registry.Register(metrics.ConnectionDuration); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := registry.Register(metrics.LastCheckTimestamp); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}

	return metrics
}

// RecordParseSuccess records a successful parse
func (m *ParserMetrics) RecordParseSuccess(reportType, source string, duration float64, size int) {
	m.ParsedReportsTotal.WithLabelValues(reportType, source).Inc()
	m.ParseDurationSeconds.WithLabelValues(reportType, source).Observe(duration)
	m.ReportSizeBytes.Observe(float64(size))
}

// RecordParseFailure records a parse failure
func (m *ParserMetrics) RecordParseFailure(reportType, source, reason string, duration float64, size int) {
	m.ParseFailuresTotal.WithLabelValues(reportType, source, reason).Inc()
	m.ParseDurationSeconds.WithLabelValues(reportType, source).Observe(duration)
	m.ReportSizeBytes.Observe(float64(size))
}

// RecordIMAPConnection records an IMAP connection attempt
func (m *IMAPMetrics) RecordConnection(success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.ConnectionAttemptsTotal.WithLabelValues(status).Inc()
}

// RecordMessageProcessed records a processed IMAP message
func (m *IMAPMetrics) RecordMessageProcessed(action string, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.MessagesProcessedTotal.WithLabelValues(action, status).Inc()
}

// RecordConnectionDuration records IMAP connection duration
func (m *IMAPMetrics) RecordConnectionDuration(duration float64) {
	m.ConnectionDuration.Observe(duration)
}

// UpdateLastCheck updates the last check timestamp
func (m *IMAPMetrics) UpdateLastCheck() {
	m.LastCheckTimestamp.SetToCurrentTime()
}
