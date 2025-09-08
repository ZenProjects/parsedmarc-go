# parsedmarc-go

A Go implementation of the DMARC report parser, based on the original Python [parsedmarc](https://github.com/domainaware/parsedmarc) project.

The conversion to Golang has been done with Claude AI. 

With Claude, I also added support for:
- Clickhouse storage (with pre-configured Grafana dashboard).
- HTTP reporting method (RUA/RUF defined with the https/http scheme URI).
- Prometeus daemon mode monitoring (with IMAP or HTTP reporting).

I haven't converted Elasticsearch/Opensearch/Splunk storage because I don't use these products and can't test them.

For the same reason, I haven't converted Microsoft Graph and Gmail API support.

## Features

- ✅ DMARC aggregate report parsing ([RFC 7489](https://datatracker.ietf.org/doc/html/rfc7489)) - supports draft and 1.0 standard formats
- ✅ Forensic/failure report parsing ([RFC 6591 ARF format](https://datatracker.ietf.org/doc/html/rfc6591)) - supports auth-failure reports
- ✅ SMTP TLS report support ([RFC 8460](https://datatracker.ietf.org/doc/html/rfc8460))
  - ✅ Compressed file support (GZIP preferred, ZIP legacy support)
  - ✅ IP address geolocation (with MaxMind database)
  - ✅ Reverse DNS resolution


- ✅ Can parse reports from an inbox over IMAP 
  - ✅ TLS/SSL support for IMAP and HTTP
- ✅ Can parse reports posted over HTTP (POST/PUT methods - [IETF draft-kucherawy-dmarc-base](https://datatracker.ietf.org/doc/html/draft-kucherawy-dmarc-base-02#appendix-B.6))
  - ✅ Rate limiting and data validation

  
- ✅ JSON and CSV output formats.
  - ✅ Output to file or stdout.
- ✅ Optionally, store reports result in ClickHouse database.
- ✅ Optionally, send reports to Email or Kafka topic.


- ✅ Built-in Prometheus metrics (for the imap and httpd mode)

## Installation

### Prerequisites

- Go 1.21 or higher
- ClickHouse (optional, for storage)
- MaxMind GeoLite2 database (optional, for geolocation)

### Building

```bash
# Clone the project
git clone https://github.com/domainaware/parsedmarc-go
cd parsedmarc-go

# Install dependencies
go mod download

# Build
go build -o parsedmarc-go ./cmd/parsedmarc-go
```

### Tests

```bash
go test ./...
```
## Configuration

Copy the example configuration file:

```bash
cp config.yaml.example config.yaml
```

Edit the configuration according to your needs:

```yaml
# Logging
logging:
  level: info
  format: json
  output_path: stdout

# Parser
parser:
  offline: false
  ip_db_path: "/path/to/GeoLite2-City.mmdb"
  nameservers:
    - "1.1.1.1"
    - "1.0.0.1"
  dns_timeout: 2

# ClickHouse
clickhouse:
  enabled: true
  host: localhost
  port: 9000
  database: dmarc
  username: default
  password: ""
```

## Usage

### Command Line Options

```bash
Usage of parsedmarc-go:
  -config string
        Config file path (default "config.yaml")
  -daemon
        Run as daemon (enables IMAP and HTTP)
  -format string
        Output format: json, csv (default "json")
  -input string
        Input file or directory to parse
  -output string
        Output file (default: stdout)
  -version
        Show version information
```

### Parsing a single file

```bash
./parsedmarc-go -input report.xml
```

### Output to JSON file

```bash
./parsedmarc-go -input report.xml -output results.json -format json
```

### Output to CSV file

```bash
./parsedmarc-go -input report.xml -output results.csv -format csv
```

### Output to stdout (default)

```bash
./parsedmarc-go -input report.xml -format json
```

### Parsing a directory

```bash
./parsedmarc-go -input /path/to/reports/ -output all_reports.json -format json
```

### Daemon mode (IMAP + HTTP)

```bash
./parsedmarc-go -daemon -config config.yaml
```

### HTTP server only

```bash
# Enable HTTP in config.yaml then:
./parsedmarc-go -daemon
```

### IMAP client only

```bash
# Enable IMAP in config.yaml then:
./parsedmarc-go -daemon
```

### With custom configuration

```bash
./parsedmarc-go -config /path/to/config.yaml -input report.xml
```

### Environment variables

You can also use environment variables for configuration:

```bash
export CLICKHOUSE_HOST=clickhouse.example.com
export CLICKHOUSE_PORT=9000
export CLICKHOUSE_USERNAME=myuser
export CLICKHOUSE_PASSWORD=mypassword
export IMAP_HOST=imap.example.com
export IMAP_USERNAME=dmarc@example.com
export IMAP_PASSWORD=password
export HTTP_ENABLED=true
export HTTP_PORT=8080

./parsedmarc-go -daemon
```

## HTTP Endpoints

### RFC 7489 Compliant (Simple)

```bash
# Submit DMARC report
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/xml" \
  --data @report.xml

# Submit forensic report
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: text/plain" \
  --data @forensic-report.txt

# Submit SMTP TLS report
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/json" \
  --data @smtp-tls-report.json
```

### Monitoring endpoints (prometheus exporter)

```bash
# Service health
curl http://localhost:8080/health

# Prometheus metrics
curl http://localhost:8080/metrics
```

### Available Prometheus metrics

- `parsedmarc_http_requests_total` - Total HTTP requests count
- `parsedmarc_http_request_duration_seconds` - HTTP request duration
- `parsedmarc_reports_processed_total` - Successfully processed reports
- `parsedmarc_reports_failed_total` - Failed reports
- `parsedmarc_http_active_connections` - Active HTTP connections
- `parsedmarc_report_size_bytes` - Size of received reports
- `parsedmarc_parser_reports_total` - Parsed reports by type
- `parsedmarc_parser_failures_total` - Parsing failures by type
- `parsedmarc_imap_connections_total` - IMAP connection attempts
- `parsedmarc_imap_messages_total` - Processed IMAP messages

## ClickHouse Structure

The program automatically creates the following tables:

### dmarc_aggregate_reports
Main table for aggregate report metadata.

### dmarc_aggregate_records
Table for individual aggregate report records.

### dmarc_forensic_reports
Table for forensic reports.

## ClickHouse Query Examples

### Top 10 most reported domains

```sql
SELECT 
    domain,
    count() as report_count,
    sum(count) as total_messages
FROM dmarc_aggregate_records 
WHERE begin_date >= today() - 30
GROUP BY domain 
ORDER BY total_messages DESC 
LIMIT 10;
```

### DMARC compliance rate by organization

```sql
SELECT 
    org_name,
    countIf(dmarc_aligned = 1) as aligned_count,
    countIf(dmarc_aligned = 0) as not_aligned_count,
    round((aligned_count / (aligned_count + not_aligned_count)) * 100, 2) as alignment_rate
FROM dmarc_aggregate_records 
WHERE begin_date >= today() - 7
GROUP BY org_name 
ORDER BY alignment_rate DESC;
```

### Most frequent source IPs

```sql
SELECT 
    source_ip_address,
    source_country,
    source_reverse_dns,
    sum(count) as message_count
FROM dmarc_aggregate_records 
WHERE begin_date >= today() - 7
GROUP BY source_ip_address, source_country, source_reverse_dns
ORDER BY message_count DESC 
LIMIT 20;
```

## Supported Standards

parsedmarc-go implements the following email authentication and reporting standards:

- **<a href="https://tools.ietf.org/html/rfc7489">RFC 7489</a>** - Domain-based Message Authentication, Reporting, and Conformance (DMARC)
  - Aggregate reports (RUA)
  - Policy configuration and validation
  
- **<a href="https://tools.ietf.org/html/rfc6591">RFC 6591</a>** - Authentication Failure Reporting Using the Abuse Reporting Format
  - Forensic/failure reports (RUF)
  - Detailed authentication failure information
  
- **<a href="https://tools.ietf.org/html/rfc8460">RFC 8460</a>** - SMTP TLS Reporting
  - TLS connection and policy reporting
  - SMTP transport security analysis


## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Sean Whalen](https://github.com/seanthegeek) for the original Python parsedmarc project
