package main

import (
	"fmt"
	"log"
	"os"

	"parsedmarc-go/internal/config"
	"parsedmarc-go/internal/logger"
	"parsedmarc-go/internal/output"
	"parsedmarc-go/internal/parser"
)

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

	err = p.ParseWithOutput(data, outputWriter)
	if err != nil {
		log.Fatalf("Failed to parse file: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Successfully parsed %s and output as %s\n", inputFile, format)
}
