package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/http"
	"parsedmarc-go/internal/imap"
	"parsedmarc-go/internal/kafka"
	"parsedmarc-go/internal/logger"
	"parsedmarc-go/internal/output"
	"parsedmarc-go/internal/parser"
	"parsedmarc-go/internal/smtp"
	"parsedmarc-go/internal/storage/clickhouse"
)

const version = "1.0.0"

func main() {
	var (
		configFile   = flag.String("config", "config.yaml", "Config file path")
		inputFile    = flag.String("input", "", "Input file or directory to parse")
		outputFile   = flag.String("output", "", "Output file (default: stdout)")
		outputFormat = flag.String("format", "json", "Output format: json, csv")
		showVersion  = flag.Bool("version", false, "Show version information")
		daemon       = flag.Bool("daemon", false, "Run as daemon (enables IMAP and HTTP)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("parsedmarc-go version %s\n", version)
		return
	}

	// Initialize configuration
	var cfg *config.Config
	var err error

	// Try to load config file, fallback to defaults if not found
	if *configFile != "" {
		cfg, err = config.Load(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Use default configuration
		cfg = config.LoadDefault()
	}

	// Initialize logger
	log, err := logger.New(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := log.Sync(); err != nil {
			// Ignore sync errors on stdout/stderr as they're common and expected
			if !strings.Contains(err.Error(), "inappropriate ioctl for device") &&
				!strings.Contains(err.Error(), "invalid argument") {
				fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
			}
		}
	}()

	log.Info("Starting parsedmarc-go",
		zap.String("version", version),
		zap.String("config", *configFile),
		zap.Bool("daemon", *daemon),
	)

	// Initialize storage
	var storage parser.Storage
	if cfg.ClickHouse.Enabled {
		storage, err = clickhouse.New(cfg.ClickHouse, log)
		if err != nil {
			log.Fatal("Failed to initialize ClickHouse storage", zap.Error(err))
		}
		defer storage.Close()
	}

	// Initialize parser
	p := parser.New(cfg.Parser, storage, log)

	// Handle single file processing
	if *inputFile != "" && !*daemon {
		// Validate output format
		format := output.Format(strings.ToLower(*outputFormat))
		if format != output.FormatJSON && format != output.FormatCSV {
			log.Fatal("Invalid output format", zap.String("format", *outputFormat))
		}

		// Create SMTP client if configured
		var smtpSender output.SMTPSender
		if cfg.SMTP.Enabled {
			smtpSender = smtp.New(&cfg.SMTP, log)
		}

		// Create Kafka client if configured
		var kafkaSender output.KafkaSender
		if cfg.Kafka.Enabled {
			kafkaSender = kafka.New(&cfg.Kafka, log)
		}

		// Create output writer
		outputWriter, err := output.NewWriter(output.Config{
			Format:      format,
			File:        *outputFile,
			SMTPSender:  smtpSender,
			KafkaSender: kafkaSender,
			Logger:      log,
		})
		if err != nil {
			log.Fatal("Failed to create output writer", zap.Error(err))
		}
		defer outputWriter.Close()

		err = parseFileWithCustomOutput(*inputFile, p, outputWriter, log)
		if err != nil {
			log.Fatal("Failed to parse file",
				zap.String("file", *inputFile),
				zap.Error(err),
			)
		}
		log.Info("Processing completed successfully")
		return
	}

	// Run in daemon mode
	if *daemon || cfg.IMAP.Enabled || cfg.HTTP.Enabled {
		runDaemon(cfg, p, log)
	} else {
		log.Info("No input file specified and daemon mode disabled")
		log.Info("Use -input flag for single file processing or -daemon flag for continuous processing")
	}
}

func runDaemon(cfg *config.Config, p *parser.Parser, log *zap.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Start HTTP server if enabled
	var httpServer *http.Server
	if cfg.HTTP.Enabled {
		httpServer = http.New(cfg.HTTP, p, log)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := httpServer.Start(); err != nil {
				log.Error("HTTP server failed", zap.Error(err))
			}
		}()
		log.Info("HTTP server started")
	}

	// Start IMAP client if enabled
	var imapClient *imap.Client
	if cfg.IMAP.Enabled {
		imapClient = imap.New(cfg.IMAP, p, log)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if err := imapClient.Connect(); err != nil {
						log.Error("Failed to connect to IMAP server", zap.Error(err))
						time.Sleep(30 * time.Second)
						continue
					}

					if err := imapClient.ProcessMessages(); err != nil {
						log.Error("Failed to process IMAP messages", zap.Error(err))
					}

					if err := imapClient.Disconnect(); err != nil {
						log.Error("Failed to disconnect IMAP client during processing", zap.Error(err))
					}

					// Wait before next check
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(cfg.IMAP.CheckInterval) * time.Second):
					}
				}
			}
		}()
		log.Info("IMAP client started")
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	log.Info("Received signal, shutting down", zap.String("signal", sig.String()))

	// Cancel context to stop goroutines
	cancel()

	// Stop HTTP server gracefully
	if httpServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := httpServer.Stop(shutdownCtx); err != nil {
			log.Error("Failed to stop HTTP server", zap.Error(err))
		} else {
			log.Info("HTTP server stopped")
		}
	}

	// Disconnect IMAP client
	if imapClient != nil {
		if err := imapClient.Disconnect(); err != nil {
			log.Error("Failed to disconnect IMAP client", zap.Error(err))
		} else {
			log.Info("IMAP client disconnected")
		}
	}

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("All services stopped")
	case <-time.After(30 * time.Second):
		log.Warn("Timeout waiting for services to stop")
	}
}

// parseFileWithCustomOutput parses a file and writes output using the specified writer
func parseFileWithCustomOutput(inputFile string, p *parser.Parser, outputWriter output.Writer, log *zap.Logger) error {
	// Check if input is a directory or file
	stat, err := os.Stat(inputFile)
	if err != nil {
		return fmt.Errorf("failed to stat input: %w", err)
	}

	if stat.IsDir() {
		return parseDirectoryWithCustomOutput(inputFile, p, outputWriter, log)
	} else {
		return parseSingleFileWithCustomOutput(inputFile, p, outputWriter, log)
	}
}

// parseDirectoryWithCustomOutput parses all files in a directory
func parseDirectoryWithCustomOutput(directory string, p *parser.Parser, outputWriter output.Writer, log *zap.Logger) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories for now
		}

		filePath := fmt.Sprintf("%s/%s", directory, entry.Name())
		log.Info("Processing file", zap.String("file", filePath))

		if err := parseSingleFileWithCustomOutput(filePath, p, outputWriter, log); err != nil {
			log.Warn("Failed to process file", zap.String("file", filePath), zap.Error(err))
			continue // Continue with other files
		}
	}

	return nil
}

// parseSingleFileWithCustomOutput parses a single file and writes output
func parseSingleFileWithCustomOutput(filePath string, p *parser.Parser, outputWriter output.Writer, log *zap.Logger) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse and write to output manually to avoid circular dependency
	return parseAndWriteOutput(data, p, outputWriter)
}

// parseAndWriteOutput parses data and writes to output writer
func parseAndWriteOutput(data []byte, p *parser.Parser, outputWriter output.Writer) error {
	var parseErrors []string

	// Try to parse as aggregate report first
	if aggregateReport, err := p.ParseAggregateFromBytes(data); err == nil {
		return outputWriter.WriteAggregateReport(aggregateReport)
	} else {
		parseErrors = append(parseErrors, fmt.Sprintf("aggregate: %v", err))
	}

	// Try to parse as forensic report
	if forensicReport, err := p.ParseForensicFromBytes(data); err == nil {
		return outputWriter.WriteForensicReport(forensicReport)
	} else {
		parseErrors = append(parseErrors, fmt.Sprintf("forensic: %v", err))
	}

	// Try to parse as SMTP TLS report
	if smtpTLSReport, err := p.ParseSMTPTLSFromBytes(data); err == nil {
		return outputWriter.WriteSMTPTLSReport(smtpTLSReport)
	} else {
		parseErrors = append(parseErrors, fmt.Sprintf("smtp_tls: %v", err))
	}

	return fmt.Errorf("unable to parse data as any supported report type. Details: %s",
		strings.Join(parseErrors, "; "))
}
