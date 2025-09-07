package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/logger"
	"parsedmarc-go/internal/output"
	"parsedmarc-go/internal/parser"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run output_example.go <input_file> <format>")
		fmt.Println("Format: json or csv")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	format := os.Args[2]

	// Initialize logger (minimal config)
	logConfig := config.LoggingConfig{
		Level:  "info",
		Format: "console",
	}

	logger, err := logger.New(logConfig)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Initialize parser (offline mode)
	parserConfig := config.ParserConfig{
		Offline: true,
	}

	p := parser.New(parserConfig, nil, logger)

	// Create output writer
	outputFormat := output.Format(format)
	if outputFormat != output.FormatJSON && outputFormat != output.FormatCSV {
		log.Fatalf("Invalid format: %s (use json or csv)", format)
	}

	outputWriter, err := output.NewWriter(output.Config{
		Format: outputFormat,
		File:   "", // stdout
	})
	if err != nil {
		log.Fatalf("Failed to create output writer: %v", err)
	}
	defer outputWriter.Close()

	// Read and parse file
	data, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	// Parse the data based on content type detection
	if len(data) > 0 {
		// Try to determine the file type from content
		if bytes.Contains(data[:min(100, len(data))], []byte("<?xml")) ||
			bytes.Contains(data[:min(100, len(data))], []byte("<feedback>")) {
			// This looks like XML (aggregate or forensic report)
			if bytes.Contains(data, []byte("</feedback>")) {
				// Forensic report
				report, err := p.ParseForensicFromBytes(data)
				if err != nil {
					log.Fatalf("Failed to parse forensic report: %v", err)
				}
				err = outputWriter.WriteForensicReport(report)
				if err != nil {
					log.Fatalf("Failed to write forensic report: %v", err)
				}
			} else {
				// Aggregate report
				report, err := p.ParseAggregateFromBytes(data)
				if err != nil {
					log.Fatalf("Failed to parse aggregate report: %v", err)
				}
				err = outputWriter.WriteAggregateReport(report)
				if err != nil {
					log.Fatalf("Failed to write aggregate report: %v", err)
				}
			}
		} else if bytes.Contains(data, []byte("smtp-tls-reporting")) ||
			bytes.Contains(data, []byte("\"policy-domain\"")) {
			// SMTP TLS report (JSON format)
			report, err := p.ParseSMTPTLSFromBytes(data)
			if err != nil {
				log.Fatalf("Failed to parse SMTP TLS report: %v", err)
			}
			err = outputWriter.WriteSMTPTLSReport(report)
			if err != nil {
				log.Fatalf("Failed to write SMTP TLS report: %v", err)
			}
		} else {
			log.Fatalf("Unable to determine report type from file content")
		}
	} else {
		log.Fatalf("Empty file")
	}

	fmt.Fprintf(os.Stderr, "Successfully parsed %s and output as %s\n", inputFile, format)
}
