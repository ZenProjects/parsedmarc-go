# Monitoring and Metrics

This guide covers monitoring parsedmarc-go using Prometheus metrics and setting up alerting.

## Prometheus Metrics

parsedmarc-go exposes Prometheus-compatible metrics at `/metrics` endpoint.

### Available Metrics

#### Processing Metrics

```prometheus
# Total reports processed
parsedmarc_reports_processed_total{type="aggregate|forensic|tls"} counter

# Failed report processing
parsedmarc_reports_failed_total{type="aggregate|forensic|tls", reason="parse_error|db_error|validation_error"} counter

# Processing duration
parsedmarc_report_processing_duration_seconds{type="aggregate|forensic|tls"} histogram

# Current processing queue size
parsedmarc_processing_queue_size gauge
```

#### HTTP Metrics

```prometheus
# HTTP requests
parsedmarc_http_requests_total{method="GET|POST", endpoint="/health|/metrics|/dmarc/report", status="200|400|500"} counter

# HTTP request duration
parsedmarc_http_request_duration_seconds{method="GET|POST", endpoint="/health|/metrics|/dmarc/report"} histogram

# Active HTTP connections
parsedmarc_http_connections_active gauge

# Upload size
parsedmarc_http_upload_size_bytes histogram
```

#### IMAP Metrics

```prometheus
# IMAP messages processed
parsedmarc_imap_messages_processed_total{mailbox="INBOX"} counter

# IMAP connection status
parsedmarc_imap_connection_status{server="imap.example.com"} gauge

# IMAP check duration
parsedmarc_imap_check_duration_seconds{mailbox="INBOX"} histogram

# Messages in mailbox
parsedmarc_imap_messages_in_mailbox{mailbox="INBOX"} gauge
```

#### Database Metrics

```prometheus
# ClickHouse operations
parsedmarc_clickhouse_operations_total{operation="insert|select", table="dmarc_aggregate_reports|dmarc_aggregate_records|dmarc_forensic_reports"} counter

# ClickHouse operation duration
parsedmarc_clickhouse_operation_duration_seconds{operation="insert|select"} histogram

# ClickHouse connection pool
parsedmarc_clickhouse_connections_active gauge
parsedmarc_clickhouse_connections_idle gauge
```

#### System Metrics

```prometheus
# Memory usage
parsedmarc_memory_usage_bytes gauge

# CPU usage
parsedmarc_cpu_usage_percent gauge

# Goroutines
parsedmarc_goroutines_total gauge

# File descriptors
parsedmarc_file_descriptors_open gauge
```

## Metrics Endpoint

Access metrics at:
```bash
curl http://localhost:8080/metrics
```

Example output:
```prometheus
# HELP parsedmarc_reports_processed_total Total number of reports processed
# TYPE parsedmarc_reports_processed_total counter
parsedmarc_reports_processed_total{type="aggregate"} 1247
parsedmarc_reports_processed_total{type="forensic"} 23

# HELP parsedmarc_http_requests_total Total HTTP requests
# TYPE parsedmarc_http_requests_total counter
parsedmarc_http_requests_total{method="POST",endpoint="/dmarc/report",status="200"} 856
parsedmarc_http_requests_total{method="GET",endpoint="/health",status="200"} 12450

# HELP parsedmarc_processing_duration_seconds Time spent processing reports
# TYPE parsedmarc_processing_duration_seconds histogram
parsedmarc_processing_duration_seconds_bucket{type="aggregate",le="0.1"} 234
parsedmarc_processing_duration_seconds_bucket{type="aggregate",le="0.5"} 1156
```

## Prometheus Configuration

### Prometheus Setup

Add parsedmarc-go to your `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'parsedmarc-go'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
    scrape_interval: 30s
    scrape_timeout: 10s
```

### Docker Compose with Prometheus

```yaml
version: '3.8'

services:
  parsedmarc-go:
    image: parsedmarc-go:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/app/config.yaml

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_INSTALL_PLUGINS=grafana-clickhouse-datasource
    volumes:
      - grafana_data:/var/lib/grafana
    depends_on:
      - prometheus

volumes:
  prometheus_data:
  grafana_data:
```

## Grafana Dashboards

### Prometheus Data Source

Add Prometheus as a data source in Grafana:
- URL: `http://prometheus:9090`
- Access: Server (default)

### Key Dashboards

#### Processing Performance

```promql
# Reports processed per second
rate(parsedmarc_reports_processed_total[5m])

# Processing success rate
rate(parsedmarc_reports_processed_total[5m]) / 
(rate(parsedmarc_reports_processed_total[5m]) + rate(parsedmarc_reports_failed_total[5m])) * 100

# Average processing time
rate(parsedmarc_report_processing_duration_seconds_sum[5m]) / 
rate(parsedmarc_report_processing_duration_seconds_count[5m])
```

#### HTTP Performance

```promql
# HTTP requests per second
rate(parsedmarc_http_requests_total[5m])

# HTTP error rate
rate(parsedmarc_http_requests_total{status=~"4..|5.."}[5m]) / 
rate(parsedmarc_http_requests_total[5m]) * 100

# Average response time
rate(parsedmarc_http_request_duration_seconds_sum[5m]) / 
rate(parsedmarc_http_request_duration_seconds_count[5m])
```

#### System Resources

```promql
# Memory usage
parsedmarc_memory_usage_bytes

# CPU usage
parsedmarc_cpu_usage_percent

# Active connections
parsedmarc_http_connections_active
```

## Alerting

### AlertManager Configuration

Create alert rules in `alert_rules.yml`:

```yaml
groups:
  - name: parsedmarc-go
    rules:
      - alert: ParsedmarcHighErrorRate
        expr: rate(parsedmarc_reports_failed_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High error rate in parsedmarc-go"
          description: "parsedmarc-go has error rate of {{ $value }} errors per second"

      - alert: ParsedmarcDown
        expr: up{job="parsedmarc-go"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "parsedmarc-go is down"
          description: "parsedmarc-go has been down for more than 1 minute"

      - alert: ParsedmarcHighMemoryUsage
        expr: parsedmarc_memory_usage_bytes > 500000000  # 500MB
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage in parsedmarc-go"
          description: "parsedmarc-go is using {{ $value | humanize }}B of memory"

      - alert: ParsedmarcLowProcessingRate
        expr: rate(parsedmarc_reports_processed_total[10m]) < 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Low processing rate in parsedmarc-go"
          description: "parsedmarc-go is processing only {{ $value }} reports per second"

      - alert: ParsedmarcQueueBacklog
        expr: parsedmarc_processing_queue_size > 100
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Processing queue backlog in parsedmarc-go"
          description: "parsedmarc-go has {{ $value }} items in processing queue"
```

## Health Checks

### Basic Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "ok",
  "timestamp": "2024-12-01T10:30:45Z",
  "version": "1.0.0",
  "uptime": "24h30m15s"
}
```

### Kubernetes Health Checks

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: parsedmarc-go
    image: parsedmarc-go:latest
    livenessProbe:
      httpGet:
        path: /health
        port: 8080
      initialDelaySeconds: 30
      periodSeconds: 10
      timeoutSeconds: 5
      failureThreshold: 3
    readinessProbe:
      httpGet:
        path: /health
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
      timeoutSeconds: 3
      failureThreshold: 2
```

### Docker Health Check

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1
```

## Log Monitoring

### Structured Logging

parsedmarc-go uses structured JSON logging:

```json
{
  "timestamp": "2024-12-01T10:30:45Z",
  "level": "info",
  "message": "Processing DMARC report",
  "report_id": "12345678901234567890",
  "org_name": "google.com",
  "domain": "example.com",
  "records": 15,
  "processing_time_ms": 125
}
```


## Performance Monitoring

### Key Performance Indicators

1. **Throughput**: Reports processed per second
2. **Latency**: Average processing time per report
3. **Error Rate**: Percentage of failed operations
4. **Resource Usage**: CPU, memory, and network utilization
5. **Queue Depth**: Backlog of pending operations


# Check processing metrics
curl -s http://localhost:8080/metrics | grep parsedmarc_reports_processed_total
```

### Debug Metrics

Enable debug metrics in development:

```bash
export PARSEDMARC_DEBUG_METRICS=true
parsedmarc-go -daemon
```

This exposes additional debug metrics:
- Garbage collection statistics
- Memory allocation details
- Goroutine stack traces

### Performance Profiling

Enable pprof endpoint for profiling:

```bash
# CPU profile
go tool pprof http://localhost:8080/debug/pprof/profile

# Memory profile
go tool pprof http://localhost:8080/debug/pprof/heap

# Goroutine profile
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

For more monitoring and observability best practices, see the [Configuration Guide](configuration.md).