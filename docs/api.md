# HTTP API Reference

This document describes the HTTP API endpoints provided by parsedmarc-go.

## Base URL

All API endpoints are served from the HTTP server configured in your `config.yaml`:

```
http://localhost:8080
```

## Authentication

Currently, parsedmarc-go does not implement authentication. It's recommended to:
- Run behind a reverse proxy with authentication
- Use firewall rules to restrict access
- Enable TLS for production deployments

## Endpoints

### POST /dmarc/report

Submit DMARC reports for processing.

#### Request

**Headers:**
- `Content-Type`: `application/xml`, `application/gzip`, `application/zip`, or `multipart/form-data`
- `Content-Encoding`: `gzip` (optional, for compressed content)

**Body:**
- Raw XML report data
- Gzipped XML report data  
- ZIP archive containing XML reports
- Multipart form with report files

#### Response

**Success (200 OK):**
```json
{
  "status": "success",
  "message": "Report processed successfully",
  "report_id": "12345678901234567890",
  "records_processed": 15,
  "processing_time_ms": 125
}
```

**Error (400 Bad Request):**
```json
{
  "status": "error",
  "message": "Invalid XML format",
  "error_code": "PARSE_ERROR"
}
```

**Error (413 Payload Too Large):**
```json
{
  "status": "error", 
  "message": "File size exceeds maximum allowed size",
  "error_code": "FILE_TOO_LARGE"
}
```

**Error (429 Too Many Requests):**
```json
{
  "status": "error",
  "message": "Rate limit exceeded",
  "error_code": "RATE_LIMITED"
}
```

**Error (500 Internal Server Error):**
```json
{
  "status": "error",
  "message": "Database connection failed",
  "error_code": "INTERNAL_ERROR"
}
```

#### Examples

**XML Report:**
```bash
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/xml" \
  -d @report.xml
```

**Gzipped Report:**
```bash
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/gzip" \
  -H "Content-Encoding: gzip" \
  --data-binary @report.xml.gz
```

**ZIP Archive:**
```bash
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/zip" \
  --data-binary @reports.zip
```

**Multipart Form:**
```bash
curl -X POST http://localhost:8080/dmarc/report \
  -F "report=@report.xml"
```

### GET /health

Health check endpoint for monitoring and load balancers.

#### Request

No parameters required.

#### Response

**Success (200 OK):**
```json
{
  "status": "ok",
  "timestamp": "2024-12-01T10:30:45Z",
  "version": "1.0.0",
  "uptime": "24h30m15s",
  "database_status": "connected",
  "imap_status": "connected"
}
```

**Service Degraded (200 OK):**
```json
{
  "status": "degraded",
  "timestamp": "2024-12-01T10:30:45Z",
  "version": "1.0.0",
  "uptime": "24h30m15s",
  "database_status": "disconnected",
  "imap_status": "connected",
  "issues": [
    "ClickHouse connection failed"
  ]
}
```

**Service Down (503 Service Unavailable):**
```json
{
  "status": "down",
  "timestamp": "2024-12-01T10:30:45Z",
  "version": "1.0.0",
  "uptime": "24h30m15s",
  "database_status": "disconnected",
  "imap_status": "disconnected"
}
```

#### Example

```bash
curl http://localhost:8080/health
```

### GET /metrics

Prometheus metrics endpoint for monitoring.

#### Request

No parameters required.

#### Response

**Success (200 OK):**
```prometheus
# HELP parsedmarc_reports_processed_total Total number of reports processed
# TYPE parsedmarc_reports_processed_total counter
parsedmarc_reports_processed_total{type="aggregate"} 1247
parsedmarc_reports_processed_total{type="forensic"} 23

# HELP parsedmarc_http_requests_total Total HTTP requests
# TYPE parsedmarc_http_requests_total counter
parsedmarc_http_requests_total{method="POST",endpoint="/dmarc/report",status="200"} 856

# HELP parsedmarc_processing_duration_seconds Time spent processing reports  
# TYPE parsedmarc_processing_duration_seconds histogram
parsedmarc_processing_duration_seconds_bucket{type="aggregate",le="0.1"} 234
parsedmarc_processing_duration_seconds_bucket{type="aggregate",le="0.5"} 1156
```

#### Example

```bash
curl http://localhost:8080/metrics
```

## Error Codes

| Code | Description |
|------|-------------|
| `PARSE_ERROR` | XML parsing failed |
| `VALIDATION_ERROR` | Report validation failed |
| `DB_ERROR` | Database operation failed |
| `FILE_TOO_LARGE` | File exceeds size limit |
| `RATE_LIMITED` | Request rate limit exceeded |
| `UNSUPPORTED_FORMAT` | Unsupported file format |
| `COMPRESSION_ERROR` | Failed to decompress file |
| `INTERNAL_ERROR` | Internal server error |

## Rate Limiting

The HTTP API implements rate limiting to prevent abuse:

- **Default limit**: 60 requests per minute per IP
- **Burst capacity**: 10 requests
- **Response header**: `X-RateLimit-Remaining`

Configure in `config.yaml`:
```yaml
http:
  rate_limit: 100      # requests per minute
  rate_burst: 20       # burst capacity
```

## File Size Limits

Maximum upload sizes:

- **Default**: 10MB per request
- **Configurable**: Set `max_upload_size` in config
- **Compressed files**: Automatically decompressed

Configure in `config.yaml`:
```yaml
http:
  max_upload_size: 52428800  # 50MB
```

## Content Types

Supported content types:

| Content-Type | Description |
|--------------|-------------|
| `application/xml` | Raw XML report |
| `application/gzip` | Gzip compressed XML |
| `application/zip` | ZIP archive with XML reports |
| `multipart/form-data` | Form upload with files |
| `application/octet-stream` | Binary data (auto-detected) |

## Security Considerations

1. **TLS Encryption**: Enable HTTPS in production
2. **Reverse Proxy**: Use nginx/Apache for additional security
3. **Firewall**: Restrict access to trusted networks
4. **Input Validation**: All inputs are validated and sanitized
5. **Resource Limits**: File size and rate limiting prevent DoS

## Client Libraries

### Go

```go
package main

import (
    "bytes"
    "net/http"
    "io"
    "os"
)

func submitDMARCReport(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return err
    }
    defer file.Close()

    resp, err := http.Post(
        "http://localhost:8080/dmarc/report",
        "application/xml",
        file,
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("API error: %s", body)
    }
    
    return nil
}
```

### Python

```python
import requests

def submit_dmarc_report(filename):
    with open(filename, 'rb') as f:
        response = requests.post(
            'http://localhost:8080/dmarc/report',
            headers={'Content-Type': 'application/xml'},
            data=f
        )
    
    if response.status_code == 200:
        return response.json()
    else:
        response.raise_for_status()

# Usage
result = submit_dmarc_report('report.xml')
print(f"Processed {result['records_processed']} records")
```

### Bash

```bash
#!/bin/bash

submit_report() {
    local file="$1"
    
    curl -X POST http://localhost:8080/dmarc/report \
        -H "Content-Type: application/xml" \
        -d @"$file" \
        -w "HTTP Status: %{http_code}\n"
}

# Process all XML files in directory
for file in *.xml; do
    if submit_report "$file"; then
        echo "Successfully processed $file"
    else
        echo "Failed to process $file"
    fi
done
```

## Testing

Test the API endpoints:

```bash
# Health check
curl -f http://localhost:8080/health

# Submit test report
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0"?>...'

# Check metrics
curl http://localhost:8080/metrics | grep parsedmarc_
```
