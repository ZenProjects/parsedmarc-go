# Usage Guide

This guide covers how to use parsedmarc-go in various scenarios.

## Command Line Usage

### Basic Report Parsing

Parse a single DMARC report file:
```bash
parsedmarc-go -input /path/to/report.xml
```

Parse all files in a directory:
```bash
parsedmarc-go -input /path/to/reports/
```

### Daemon Mode

Run parsedmarc-go as a daemon to enable IMAP and HTTP services:
```bash
parsedmarc-go -daemon -config config.yaml
```

### Show Version
```bash
parsedmarc-go -version
```

## Configuration File

Create a configuration file to customize parsedmarc-go behavior:

```yaml
logging:
  level: info
  format: console

parser:
  output_format: json
  strip_attachment_payloads: false
  always_use_local_nameservers: false
  nameservers:
    - "1.1.1.1"
    - "8.8.8.8"

clickhouse:
  enabled: true
  host: localhost
  port: 9000
  username: default
  password: ""
  database: dmarc
  dial_timeout: 10s
  max_open_conns: 10

imap:
  enabled: true
  host: imap.gmail.com
  port: 993
  username: your-email@example.com
  password: your-app-password
  folders:
    - INBOX
  delete_processed: false
  reports_folder: "DMARC Reports"
  check_interval: 300s
  tls: true

http:
  enabled: true
  listen: ":8080"
  max_upload_size: 10MB
  rate_limit: 100
  read_timeout: 30s
  write_timeout: 30s
```

## IMAP Configuration

### Gmail Setup

1. Enable 2-factor authentication
2. Create an app password for parsedmarc-go
3. Configure IMAP settings:

```yaml
imap:
  enabled: true
  host: imap.gmail.com
  port: 993
  username: your-email@gmail.com
  password: your-app-password
  tls: true
  folders:
    - INBOX
  delete_processed: false
  reports_folder: "DMARC Reports"
```

### Microsoft 365 Setup

```yaml
imap:
  enabled: true
  host: outlook.office365.com
  port: 993
  username: your-email@company.com
  password: your-password
  tls: true
  folders:
    - INBOX
  delete_processed: false
  reports_folder: "DMARC"
```

### Custom IMAP Server

```yaml
imap:
  enabled: true
  host: mail.example.com
  port: 993
  username: dmarc@example.com
  password: your-password
  tls: true
  folders:
    - INBOX
    - "DMARC Reports"
  delete_processed: true
  check_interval: 600s
```

## HTTP Endpoint Usage

### Submit DMARC Reports via HTTP

The HTTP endpoint accepts DMARC reports in various formats:

#### XML Reports (RFC 7489)

```bash
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/xml" \
  -d @report.xml
```

#### Compressed Reports

```bash
# Gzip compressed
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/gzip" \
  -H "Content-Encoding: gzip" \
  --data-binary @report.xml.gz

# Zip compressed
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/zip" \
  --data-binary @report.zip
```

#### Multipart Form Data

```bash
curl -X POST http://localhost:8080/dmarc/report \
  -F "report=@report.xml"
```

### Health Check

Check if the service is running:
```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "ok",
  "timestamp": "2024-12-01T10:30:45Z"
}
```

### Metrics Endpoint

Access Prometheus metrics:
```bash
curl http://localhost:8080/metrics
```

## Report Types

parsedmarc-go supports different types of DMARC reports:

### Aggregate Reports
- Standard DMARC aggregate reports (RFC 7489)
- Contain statistical data about email authentication
- Stored in `dmarc_aggregate_reports` and `dmarc_aggregate_records` tables

### Forensic Reports
- DMARC failure reports (RFC 6591)
- Contain detailed information about individual failed messages
- Stored in `dmarc_forensic_reports` table

### SMTP TLS Reports
- SMTP TLS reporting (RFC 8460)
- Information about TLS usage in SMTP connections
- Stored in `smtp_tls_reports` table

## Output Formats

### JSON Output
When parsing files directly, parsedmarc-go outputs structured JSON:

```json
{
  "report_metadata": {
    "org_name": "google.com",
    "email": "noreply-dmarc-support@google.com",
    "extra_contact_info": "",
    "report_id": "12345678901234567890",
    "date_range": {
      "begin": "2024-11-30T00:00:00Z",
      "end": "2024-11-30T23:59:59Z"
    }
  },
  "policy_published": {
    "domain": "example.com",
    "adkim": "r",
    "aspf": "r",
    "p": "none",
    "sp": "none",
    "pct": 100
  },
  "records": [
    {
      "source": {
        "ip": "192.0.2.1",
        "country": "US",
        "reverse_dns": "mail.example.net",
        "base_domain": "example.net"
      },
      "count": 15,
      "alignment": {
        "spf": "pass",
        "dkim": "fail",
        "dmarc": "fail"
      },
      "identifiers": {
        "header_from": "example.com"
      }
    }
  ]
}
```

### ClickHouse Storage
When ClickHouse is enabled, reports are automatically stored in normalized tables optimized for analytics and reporting.

## Performance Tuning

### Parser Configuration

```yaml
parser:
  # Process reports concurrently
  max_workers: 4
  
  # Reduce memory usage for large attachments
  strip_attachment_payloads: true
  
  # Use local nameservers for better performance
  always_use_local_nameservers: true
  nameservers:
    - "127.0.0.1"
    - "1.1.1.1"
```

### ClickHouse Optimization

```yaml
clickhouse:
  # Increase connection pool for high throughput
  max_open_conns: 50
  max_idle_conns: 10
  
  # Batch inserts for better performance
  batch_size: 1000
  batch_timeout: 5s
  
  # Connection settings
  dial_timeout: 30s
  read_timeout: 60s
  write_timeout: 60s
```

### HTTP Server Tuning

```yaml
http:
  # Increase limits for high volume
  max_upload_size: 50MB
  rate_limit: 1000
  
  # Adjust timeouts
  read_timeout: 60s
  write_timeout: 60s
  idle_timeout: 120s
  
  # Enable compression
  compression: true
```

## Monitoring and Logging

### Log Levels

Set appropriate log levels:
```yaml
logging:
  level: info  # debug, info, warn, error
  format: json # json, console
  output: stdout # stdout, stderr, or file path
```

### Structured Logging

parsedmarc-go uses structured logging with contextual information:
- Request IDs for HTTP requests
- Report metadata for parsing operations
- Performance metrics for database operations

### Metrics

Monitor these key Prometheus metrics:
- `parsedmarc_reports_processed_total`: Total reports processed
- `parsedmarc_reports_failed_total`: Failed report processing
- `parsedmarc_http_requests_total`: HTTP endpoint usage
- `parsedmarc_imap_messages_processed`: IMAP message processing

## Troubleshooting

### Common Issues

#### DNS Resolution Problems
```yaml
parser:
  always_use_local_nameservers: false
  nameservers:
    - "8.8.8.8"
    - "1.1.1.1"
```

#### IMAP Connection Issues
- Check firewall settings for port 993/143
- Verify TLS settings match server requirements
- Ensure credentials are correct (use app passwords for Gmail)

#### ClickHouse Connection Problems
- Verify ClickHouse is running and accessible
- Check database permissions
- Ensure network connectivity

#### High Memory Usage
- Enable `strip_attachment_payloads: true`
- Reduce `max_workers` if processing large files
- Monitor with `parsedmarc_memory_usage_bytes` metric

### Debug Mode

Enable debug logging for troubleshooting:
```yaml
logging:
  level: debug
```

### Testing Configuration

Validate your configuration:
```bash
parsedmarc-go -config config.yaml -input test-report.xml
```

## Integration Examples

### Docker Compose

```yaml
version: '3.8'
services:
  parsedmarc-go:
    image: parsedmarc-go:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/etc/parsedmarc-go/config.yaml
      - ./reports:/var/lib/parsedmarc-go/reports
    depends_on:
      - clickhouse
    
  clickhouse:
    image: clickhouse/clickhouse-server:latest
    ports:
      - "8123:8123"
      - "9000:9000"
    volumes:
      - clickhouse_data:/var/lib/clickhouse
    environment:
      CLICKHOUSE_DB: dmarc

volumes:
  clickhouse_data:
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: parsedmarc-go
spec:
  replicas: 3
  selector:
    matchLabels:
      app: parsedmarc-go
  template:
    metadata:
      labels:
        app: parsedmarc-go
    spec:
      containers:
      - name: parsedmarc-go
        image: parsedmarc-go:latest
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config
          mountPath: /etc/parsedmarc-go
        env:
        - name: CLICKHOUSE_HOST
          value: "clickhouse-service"
      volumes:
      - name: config
        configMap:
          name: parsedmarc-go-config
```

### Systemd Service

```ini
[Unit]
Description=parsedmarc-go DMARC Report Parser
After=network.target

[Service]
Type=simple
User=parsedmarc
ExecStart=/usr/local/bin/parsedmarc-go -daemon -config /etc/parsedmarc-go/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## Best Practices

1. **Security**
   - Use dedicated database user with minimal privileges
   - Store sensitive credentials in environment variables
   - Enable TLS for all network communications

2. **Performance**
   - Use ClickHouse materialized views for complex queries
   - Partition tables by date for better query performance
   - Monitor resource usage and adjust worker counts accordingly

3. **Reliability**
   - Implement proper backup strategies for ClickHouse data
   - Use health checks in production deployments
   - Monitor metrics and set up alerting

4. **Maintenance**
   - Regular log rotation
   - Database maintenance and optimization
   - Keep parsedmarc-go updated to latest version