package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"

	"go.uber.org/zap"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/parser"
)

// Server represents the HTTP server for receiving DMARC reports
type Server struct {
	config config.HTTPConfig
	parser *parser.Parser
	logger *zap.Logger
	server *http.Server

	// Rate limiting
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex

	// Metrics
	metrics *Metrics
}

// Metrics holds Prometheus metrics
type Metrics struct {
	RequestsTotal         *prometheus.CounterVec
	RequestDuration       *prometheus.HistogramVec
	ReportsProcessedTotal *prometheus.CounterVec
	ReportsFailedTotal    *prometheus.CounterVec
	ActiveConnections     prometheus.Gauge
	ReportSizeBytes       prometheus.Histogram
}

// New creates a new HTTP server instance
func New(cfg config.HTTPConfig, p *parser.Parser, logger *zap.Logger) *Server {
	metrics := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "parsedmarc_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "parsedmarc_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		ReportsProcessedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "parsedmarc_reports_processed_total",
				Help: "Total number of DMARC reports processed successfully",
			},
			[]string{"type"},
		),
		ReportsFailedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "parsedmarc_reports_failed_total",
				Help: "Total number of DMARC reports that failed to process",
			},
			[]string{"type", "reason"},
		),
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "parsedmarc_http_active_connections",
				Help: "Number of active HTTP connections",
			},
		),
		ReportSizeBytes: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "parsedmarc_report_size_bytes",
				Help:    "Size of received DMARC reports in bytes",
				Buckets: []float64{1024, 4096, 16384, 65536, 262144, 1048576, 4194304},
			},
		),
	}

	// Register metrics with error handling
	registry := prometheus.DefaultRegisterer
	metricsToRegister := []prometheus.Collector{
		metrics.RequestsTotal,
		metrics.RequestDuration,
		metrics.ReportsProcessedTotal,
		metrics.ReportsFailedTotal,
		metrics.ActiveConnections,
		metrics.ReportSizeBytes,
	}

	for _, metric := range metricsToRegister {
		if err := registry.Register(metric); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				panic(err)
			}
		}
	}

	return &Server{
		config:   cfg,
		parser:   p,
		logger:   logger,
		limiters: make(map[string]*rate.Limiter),
		metrics:  metrics,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	if !s.config.Enabled {
		s.logger.Info("HTTP server is disabled")
		return nil
	}

	// Set Gin mode based on log level
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(s.loggingMiddleware())
	router.Use(s.recoveryMiddleware())
	router.Use(s.rateLimitMiddleware())
	router.Use(s.maxSizeMiddleware())
	router.Use(s.metricsMiddleware())

	// Simple DMARC endpoint (RFC 7489 compliant)
	router.POST("/dmarc/report", s.handleDMARCReport)
	router.PUT("/dmarc/report", s.handleDMARCReport)

	// Health check
	router.GET("/health", s.handleHealth)

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Root endpoint
	router.GET("/", s.handleRoot)

	address := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	s.server = &http.Server{
		Addr:         address,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.logger.Info("Starting HTTP server",
		zap.String("address", address),
		zap.Bool("tls", s.config.TLS),
	)

	if s.config.TLS {
		if s.config.CertFile == "" || s.config.KeyFile == "" {
			return fmt.Errorf("TLS enabled but cert_file or key_file not specified")
		}
		return s.server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	}

	return s.server.ListenAndServe()
}

// Stop stops the HTTP server gracefully
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("Stopping HTTP server...")
	return s.server.Shutdown(ctx)
}

// Middleware functions

func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		s.logger.Info("HTTP request",
			zap.String("client_ip", clientIP),
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
		)
	}
}

func (s *Server) recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

func (s *Server) rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.config.RateLimit <= 0 {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		limiter := s.getLimiter(clientIP)

		if !limiter.Allow() {
			s.logger.Warn("Rate limit exceeded", zap.String("client_ip", clientIP))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": "60s",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (s *Server) maxSizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.config.MaxUploadSize > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.config.MaxUploadSize)
		}
		c.Next()
	}
}

func (s *Server) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		s.metrics.ActiveConnections.Inc()

		defer func() {
			s.metrics.ActiveConnections.Dec()
			duration := time.Since(start).Seconds()

			endpoint := s.getEndpointLabel(c.Request.URL.Path)
			method := c.Request.Method
			status := fmt.Sprintf("%d", c.Writer.Status())

			s.metrics.RequestsTotal.WithLabelValues(method, endpoint, status).Inc()
			s.metrics.RequestDuration.WithLabelValues(method, endpoint).Observe(duration)
		}()

		c.Next()
	}
}

// Rate limiter helper
func (s *Server) getLimiter(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	limiter, exists := s.limiters[ip]
	if !exists {
		// Create new limiter: rate per minute with burst capacity
		limiter = rate.NewLimiter(
			rate.Limit(float64(s.config.RateLimit)/60.0), // per second
			s.config.RateBurst,
		)
		s.limiters[ip] = limiter
	}

	return limiter
}

func (s *Server) getEndpointLabel(path string) string {
	switch {
	case strings.HasPrefix(path, "/dmarc/report"):
		return "dmarc_report"
	case strings.HasPrefix(path, "/health"):
		return "health"
	case strings.HasPrefix(path, "/metrics"):
		return "metrics"
	case path == "/":
		return "root"
	default:
		return "other"
	}
}

// Handler functions

func (s *Server) handleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "parsedmarc-go",
		"version": "1.0.0",
		"endpoints": map[string]string{
			"health":       "/health",
			"dmarc_report": "/dmarc/report",
			"metrics":      "/metrics",
		},
	})
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleDMARCReport(c *gin.Context) {
	// Simple endpoint for DMARC reports (RFC 7489 compliant)
	contentType := c.GetHeader("Content-Type")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		s.logger.Error("Failed to read request body", zap.Error(err))
		s.metrics.ReportsFailedTotal.WithLabelValues("unknown", "read_body_failed").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to read request body",
		})
		return
	}

	if len(body) == 0 {
		s.metrics.ReportsFailedTotal.WithLabelValues("unknown", "empty_body").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Empty request body",
		})
		return
	}

	// Record report size
	s.metrics.ReportSizeBytes.Observe(float64(len(body)))

	// Validate content type
	if !s.isValidDMARCContentType(contentType) {
		s.logger.Warn("Invalid content type", zap.String("content_type", contentType))
		s.metrics.ReportsFailedTotal.WithLabelValues("unknown", "invalid_content_type").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid content type. Expected XML, JSON, or multipart/form-data",
		})
		return
	}

	// Parse the report
	reportType := s.detectReportType(body, contentType)
	if err := s.parser.ParseData(body); err != nil {
		s.logger.Error("Failed to parse DMARC report", zap.Error(err))
		s.metrics.ReportsFailedTotal.WithLabelValues(reportType, "parse_failed").Inc()
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to parse DMARC report",
			"details": err.Error(),
		})
		return
	}

	s.metrics.ReportsProcessedTotal.WithLabelValues(reportType).Inc()

	s.logger.Info("Successfully processed DMARC report",
		zap.String("client_ip", c.ClientIP()),
		zap.String("content_type", contentType),
		zap.String("report_type", reportType),
		zap.Int("size", len(body)),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "DMARC report processed successfully",
	})
}

// Validation helpers

func (s *Server) isValidDMARCContentType(contentType string) bool {
	validTypes := []string{
		"application/xml",
		"text/xml",
		"application/json",
		"application/zip",
		"application/gzip",
		"application/octet-stream",
		"application/tlsrpt+json",
		"application/tlsrpt+gzip",
		"multipart/form-data",
	}

	for _, validType := range validTypes {
		if strings.Contains(strings.ToLower(contentType), validType) {
			return true
		}
	}

	return false
}

func (s *Server) detectReportType(body []byte, contentType string) string {
	contentTypeStr := strings.ToLower(contentType)

	if strings.Contains(contentTypeStr, "tlsrpt") {
		return "smtp_tls"
	}

	bodyStr := strings.ToLower(string(body[:min(len(body), 1024)]))

	if strings.Contains(bodyStr, "feedback-type:") {
		return "forensic"
	}

	if strings.Contains(bodyStr, "<feedback") || strings.Contains(bodyStr, "<report_metadata") {
		return "aggregate"
	}

	if strings.Contains(bodyStr, "organization-name") {
		return "smtp_tls"
	}

	return "unknown"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
