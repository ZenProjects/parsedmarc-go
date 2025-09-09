# parsedmarc-go

A **high-performance Go implementation** of the DMARC report parser, based on the original Python [parsedmarc](https://github.com/domainaware/parsedmarc) project.

## 🚀 Recent Major Enhancements

**Advanced Email Format Support** - Now handles complex email-based reports from major providers:
- **MIME multipart parsing** with automatic format detection
- **Base64 decoding** and **GZIP decompression** support
- **Provider compatibility**: Google, LinkedIn, Domain.de, Netease, and more
- **Enhanced error reporting** with precise line numbers for debugging

**Robust ClickHouse Integration** - Complete storage solution:
- **Dedicated SMTP TLS tables** for RFC 8460 reports
- **Optimized schema** with proper indexing and partitioning
- **Production-ready** with time-based partitioning and performance indexes

## 📋 Conversion & Enhancements

The conversion to Go was done with **Claude AI**, adding significant improvements:

✅ **Core enhancements:**
- ClickHouse storage with pre-configured Grafana dashboard
- HTTP reporting method (RUA/RUF with https/http scheme URI)  
- Prometheus daemon mode monitoring (IMAP + HTTP)
- **Advanced MIME email parsing** (NEW)
- **Enhanced error reporting with line numbers** (NEW)
- **Directory-based output mode** (NEW)

❌ **Not converted** (due to lack of testing capability):
- Elasticsearch/Opensearch/Splunk storage
- Microsoft Graph and Gmail API support

## 🌟 Core Features

### 📊 **Report Parsing** - Industry-leading format support
- ✅ **DMARC Aggregate Reports** ([RFC 7489](https://datatracker.ietf.org/doc/html/rfc7489))
  - Draft and 1.0 standard formats
  - Compressed file support (GZIP, ZIP)
  - Enhanced error reporting with line numbers

- ✅ **Forensic/Failure Reports** ([RFC 6591 ARF](https://datatracker.ietf.org/doc/html/rfc6591)) 
  - Plain text format parsing
  - **🆕 MIME multipart email parsing** (LinkedIn, Domain.de, Netease)
  - **🆕 Base64-encoded attachment support**
  - Automatic format detection and fallback

- ✅ **SMTP TLS Reports** ([RFC 8460](https://datatracker.ietf.org/doc/html/rfc8460))
  - Direct JSON format parsing
  - **🆕 MIME email format parsing** (Google, other providers)
  - **🆕 Base64 + GZIP compressed attachment pipeline** (`application/tlsrpt+gzip`)
  - Legacy compressed file support (GZIP, ZIP)

### 🌐 **Data Enhancement**
- ✅ IP address geolocation (MaxMind database integration)
- ✅ Reverse DNS resolution with caching
- ✅ Base domain extraction and normalization
- ✅ Enhanced error diagnostics with precise line numbers

### 📡 **Multiple Input Methods**
- ✅ **IMAP Email Processing** - Monitor mailboxes for incoming reports
  - TLS/SSL connection support
  - Automatic email archiving/deletion
  - Configurable check intervals
  
- ✅ **HTTP API Server** - Receive reports via HTTP POST/PUT ([IETF draft](https://datatracker.ietf.org/doc/html/draft-kucherawy-dmarc-base-02#appendix-B.6))
  - Rate limiting and request validation
  - Multiple content-type support (`application/xml`, `application/json`, `message/rfc822`)
  - File upload size limits and security

### 💾 **Flexible Output & Storage**
- ✅ **JSON and CSV output formats** with configurable fields
- ✅ **Multiple output modes:**
  - **File mode**: Concatenate all reports in single file
  - **🆕 Directory mode**: Save each report as separate timestamped file  
  - **Stdout**: Direct console output for piping
- ✅ **ClickHouse database storage** with optimized schema
- ✅ **Email delivery** via SMTP with attachment support
- ✅ **Kafka streaming** for real-time processing pipelines

### 📈 **Production Monitoring**
- ✅ **Built-in Prometheus metrics** for observability
- ✅ **Health check endpoints** for load balancer integration
- ✅ **Structured logging** with configurable levels (JSON/console)
- ✅ **Performance metrics** (parsing duration, success/failure rates)

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
        Output file or directory path (default: stdout)
  -version
        Show version information
```

### Parsing a single file

```bash
# Parse XML aggregate report
./parsedmarc-go -input report.xml

# Parse forensic email (with MIME attachments)
./parsedmarc-go -input forensic-report.eml

# Parse SMTP TLS email (with compressed attachments) 
./parsedmarc-go -input smtp-tls-report.eml
```

### Output to JSON file

```bash
./parsedmarc-go -input report.xml -output results.json -format json
```

### Output to CSV file

```bash
./parsedmarc-go -input report.xml -output results.csv -format csv
```

### Output to directory (separate files per report)

```bash
# Create output directory
mkdir ./reports_output

# Each report will be saved as a separate file with timestamp
./parsedmarc-go -input report.xml -output ./reports_output -format json
# Creates: reports_output/aggregate_20240101_120000_reportID.json

# In daemon mode, each incoming report creates a new file
./parsedmarc-go -daemon -output ./reports_output -format json
```

### Output to stdout (default)

```bash
./parsedmarc-go -input report.xml -format json
```

### Parsing a directory

```bash
# Concatenate all reports into a single file
./parsedmarc-go -input /path/to/reports/ -output all_reports.json -format json

# Save each report as a separate file
./parsedmarc-go -input /path/to/reports/ -output ./output_dir/ -format json
```

### Daemon mode (IMAP + HTTP)

Modify this section of the config.yml :
```yaml
# IMAP configuration for fetching reports from email
imap:
  enabled: true                          # Enable IMAP client
  host: "imap.host.com"                  # IMAP server hostname
  port: 993                              # IMAP server port (993 for TLS, 143 for plain)
  username: "user"                       # IMAP username
  password: "pass"                       # IMAP password
  tls: true                              # Use TLS/SSL connection
  skip_verify: false                     # Skip TLS certificate verification
  mailbox: "INBOX"                       # Mailbox to monitor
  archive_mailbox: "DMARC-Archive"       # Mailbox to move processed emails
  delete_processed: false                # Delete processed emails instead of archiving
  check_interval: 300                    # Check interval in seconds (5 minutes)

# HTTP server configuration for receiving reports
http:
  enabled: true                          # Enable HTTP server
  host: "0.0.0.0"                        # Host to bind to
  port: 8080                             # Port to listen on
  tls: false                             # Enable TLS/HTTPS
  cert_file: ""                          # TLS certificate file path (required if tls: true)
  key_file: ""                           # TLS private key file path (required if tls: true)
  rate_limit: 60                         # Requests per minute per IP
  rate_burst: 10                         # Burst capacity for rate limiter
  max_upload_size: 52428800              # Max upload size in bytes (50MB)
```

```bash
./parsedmarc-go -daemon -config config.yaml
```

### HTTP server only

Modify this section of the config.yml :
```yaml
# IMAP configuration for fetching reports from email
imap:
  enabled: false                          # Enable IMAP client

# HTTP server configuration for receiving reports
http:
  enabled: true                          # Enable HTTP server
  host: "0.0.0.0"                        # Host to bind to
  port: 8080                             # Port to listen on
  tls: false                             # Enable TLS/HTTPS
  cert_file: ""                          # TLS certificate file path (required if tls: true)
  key_file: ""                           # TLS private key file path (required if tls: true)
  rate_limit: 60                         # Requests per minute per IP
  rate_burst: 10                         # Burst capacity for rate limiter
  max_upload_size: 52428800              # Max upload size in bytes (50MB)
```

```bash
# Enable HTTP in config.yaml then:
./parsedmarc-go -daemon
```

### IMAP client only

Modify this section of the config.yml :
```yaml
# IMAP configuration for fetching reports from email
imap:
  enabled: true                          # Enable IMAP client
  host: "imap.host.com"                  # IMAP server hostname
  port: 993                              # IMAP server port (993 for TLS, 143 for plain)
  username: "user"                       # IMAP username
  password: "pass"                       # IMAP password
  tls: true                              # Use TLS/SSL connection
  skip_verify: false                     # Skip TLS certificate verification
  mailbox: "INBOX"                       # Mailbox to monitor
  archive_mailbox: "DMARC-Archive"       # Mailbox to move processed emails
  delete_processed: false                # Delete processed emails instead of archiving
  check_interval: 300                    # Check interval in seconds (5 minutes)

# HTTP server configuration for receiving reports
http:
  enabled: false                          # Enable HTTP server
```

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

### RFC 7489 Compliant (Multiple formats supported)

```bash
# Submit DMARC aggregate report (XML)
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/xml" \
  --data @report.xml

# Submit forensic report (text or email with MIME attachments)
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: text/plain" \
  --data @forensic-report.txt

curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: message/rfc822" \
  --data @forensic-email.eml

# Submit SMTP TLS report (JSON or email with compressed attachments)
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/json" \
  --data @smtp-tls-report.json

curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: message/rfc822" \
  --data @smtp-tls-email.eml
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

### dmarc_smtp_tls_reports
Table for SMTP TLS reports (RFC 8460).

### dmarc_smtp_tls_failures
Table for detailed SMTP TLS failure information.

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

### SMTP TLS success rates by organization

```sql
SELECT 
    organization_name,
    policy_domain,
    successful_session_count,
    failed_session_count,
    round((successful_session_count / (successful_session_count + failed_session_count)) * 100, 2) as success_rate
FROM dmarc_smtp_tls_reports 
WHERE begin_date >= today() - 7
ORDER BY success_rate ASC
LIMIT 10;
```

### Most common SMTP TLS failure types

```sql
SELECT 
    result_type,
    count() as failure_count,
    sum(failed_session_count) as total_failed_sessions
FROM dmarc_smtp_tls_failures 
WHERE created_at >= today() - 7
GROUP BY result_type 
ORDER BY total_failed_sessions DESC;
```

## Advanced Email Format Support

parsedmarc-go can automatically parse reports from various email formats, making it compatible with different email service providers:

### Forensic Reports (RUF)
- **Plain text format**: Simple feedback reports in email body
- **MIME multipart format**: Reports sent as email attachments
  - LinkedIn: `multipart/report` with `message/feedback-report` parts
  - Domain.de: `multipart/report` with named report attachments  
  - Netease: `multipart/mixed` with **base64-encoded** `message/feedback-report` attachments
  - Automatic base64 decoding and MIME parsing

### SMTP TLS Reports
- **Direct JSON format**: Standard JSON reports per RFC 8460
- **Email-based reports**: Reports sent as email attachments
  - Google: `multipart/report` with `application/tlsrpt+gzip` attachments
  - **Automatic base64 decoding** → **GZIP decompression** → JSON parsing
  - Support for other providers using similar email formats

### How it works
1. **Format detection**: Automatically detects if input is direct report data or email
2. **MIME parsing**: Extracts report content from email attachments  
3. **Encoding handling**: Decodes base64, decompresses GZIP automatically
4. **Fallback support**: If MIME parsing fails, falls back to simple text parsing
5. **Enhanced error reporting**: Shows precise line numbers for XML/JSON parsing errors

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
