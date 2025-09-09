# Configuration

parsedmarc-go uses YAML configuration files with support for environment variables. This guide covers all configuration options and common setup scenarios.

## Quick Start Configuration

Copy the example configuration file and edit according to your needs:

```bash
cp config.yaml.example config.yaml
```

## Configuration File Structure

The configuration file is organized into logical sections:

```yaml
# Logging configuration
logging:
  level: info
  format: json
  output_path: stdout

# Parser settings
parser:
  offline: false
  ip_db_path: "/path/to/GeoLite2-City.mmdb"
  nameservers:
    - "1.1.1.1"
    - "1.0.0.1"
  dns_timeout: 2

# ClickHouse database
clickhouse:
  enabled: true
  host: localhost
  port: 9000
  database: dmarc
  username: default
  password: ""

# IMAP client
imap:
  enabled: false
  host: ""
  username: ""
  password: ""

# HTTP server
http:
  enabled: false
  port: 8080
  rate_limit: 60
```

## Environment Variables

All configuration options can be overridden with environment variables using underscore notation:

```bash
# Override ClickHouse settings
export CLICKHOUSE_HOST=clickhouse.example.com
export CLICKHOUSE_PORT=9000
export CLICKHOUSE_USERNAME=myuser
export CLICKHOUSE_PASSWORD=mypassword

# Override HTTP settings
export HTTP_ENABLED=true
export HTTP_PORT=8080

# Override logging
export LOGGING_LEVEL=debug
export LOGGING_FORMAT=console
```

## Logging Configuration

### Basic Options

```yaml
logging:
  level: info        # debug, info, warn, error
  format: json       # json, console
  output_path: stdout # stdout, stderr, or file path
```

### Log Levels

- **debug**: Detailed information for debugging
- **info**: General information about operations
- **warn**: Warning messages for potential issues
- **error**: Error messages for failures

### Log Formats

- **json**: Structured JSON logs (recommended for production)
- **console**: Human-readable console output (recommended for development)

### Example: File Logging

```yaml
logging:
  level: info
  format: json
  output_path: /var/log/parsedmarc-go/app.log
```

## Parser Configuration

### Offline Mode

```yaml
parser:
  offline: true  # Disable all external network requests
```

When `offline: true`:
- No DNS lookups for reverse DNS
- No GeoIP database queries
- No external file downloads

### DNS Configuration

```yaml
parser:
  nameservers:
    - "1.1.1.1"      # Cloudflare
    - "1.0.0.1"      # Cloudflare
    - "8.8.8.8"      # Google
    - "8.8.4.4"      # Google
  dns_timeout: 2     # Timeout in seconds
```

### GeoIP Database

```yaml
parser:
  ip_db_path: "/path/to/GeoLite2-City.mmdb"
```

Download GeoLite2 from MaxMind:

```bash
# Register at maxmind.com and download GeoLite2-City.mmdb
wget "https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=YOUR_KEY&suffix=tar.gz" -O GeoLite2-City.tar.gz
tar -xzf GeoLite2-City.tar.gz
sudo cp GeoLite2-City_*/GeoLite2-City.mmdb /usr/share/GeoIP/
```

## ClickHouse Configuration

### Basic Setup

```yaml
clickhouse:
  enabled: true
  host: localhost
  port: 9000
  database: dmarc
  username: default
  password: ""
```

### TLS/SSL Connection

```yaml
clickhouse:
  enabled: true
  host: clickhouse.example.com
  port: 9440
  database: dmarc
  username: myuser
  password: mypassword
  tls: true
  skip_verify: false  # Set to true for self-signed certificates
```

### Connection Pool

ClickHouse connections are automatically pooled with sensible defaults:
- **Max Open Connections**: 10
- **Max Idle Connections**: 5
- **Connection Max Lifetime**: 1 hour

### Database Schema

Tables are created automatically on first run:

- `dmarc_aggregate_reports` - Report metadata
- `dmarc_aggregate_records` - Individual report records
- `dmarc_forensic_reports` - Forensic report data

## IMAP Configuration

### Basic IMAP Setup

```yaml
imap:
  enabled: true
  host: imap.gmail.com
  port: 993
  username: dmarc@example.com
  password: your-password
  tls: true
  mailbox: INBOX
```

### IMAP with OAuth2 (Future)

```yaml
imap:
  enabled: true
  host: outlook.office365.com
  port: 993
  username: dmarc@example.com
  auth_method: oauth2
  oauth2:
    client_id: your-client-id
    client_secret: your-client-secret
    refresh_token: your-refresh-token
```

### Advanced IMAP Options

```yaml
imap:
  enabled: true
  host: imap.example.com
  port: 993
  username: dmarc@example.com
  password: your-password
  tls: true
  skip_verify: false          # Skip certificate verification
  mailbox: INBOX              # Mailbox to monitor
  archive_mailbox: Processed  # Move processed emails here
  delete_processed: false     # Delete instead of archiving
  check_interval: 300         # Check every 5 minutes
```

## HTTP Server Configuration

### Basic HTTP Setup

```yaml
http:
  enabled: true
  host: 0.0.0.0
  port: 8080
```

### TLS/HTTPS

```yaml
http:
  enabled: true
  host: 0.0.0.0
  port: 8443
  tls: true
  cert_file: /path/to/cert.pem
  key_file: /path/to/key.pem
```

### Rate Limiting

```yaml
http:
  enabled: true
  port: 8080
  rate_limit: 60      # Requests per minute per IP
  rate_burst: 10      # Burst capacity
  max_upload_size: 52428800  # 50MB max upload
```

## Complete Configuration Examples

### Development Setup

```yaml
logging:
  level: debug
  format: console
  output_path: stdout

parser:
  offline: false
  nameservers: ["1.1.1.1", "1.0.0.1"]
  dns_timeout: 2

clickhouse:
  enabled: true
  host: localhost
  port: 9000
  database: dmarc

http:
  enabled: true
  port: 8080
  rate_limit: 100

imap:
  enabled: false
```

### Production Setup

```yaml
logging:
  level: info
  format: json
  output_path: /var/log/parsedmarc-go/app.log

parser:
  offline: false
  ip_db_path: /usr/share/GeoIP/GeoLite2-City.mmdb
  nameservers:
    - "1.1.1.1"
    - "1.0.0.1"
  dns_timeout: 2

clickhouse:
  enabled: true
  host: clickhouse.internal.com
  port: 9000
  database: dmarc
  username: parsedmarc
  password: ${CLICKHOUSE_PASSWORD}
  tls: true

http:
  enabled: true
  host: 0.0.0.0
  port: 8080
  tls: true
  cert_file: /etc/ssl/certs/parsedmarc.crt
  key_file: /etc/ssl/private/parsedmarc.key
  rate_limit: 60
  rate_burst: 10

imap:
  enabled: true
  host: imap.example.com
  port: 993
  username: dmarc@example.com
  password: ${IMAP_PASSWORD}
  tls: true
  mailbox: INBOX
  archive_mailbox: DMARC-Processed
  check_interval: 300
```

### High-Volume Setup

```yaml
logging:
  level: warn
  format: json
  output_path: /var/log/parsedmarc-go/app.log

parser:
  offline: false
  ip_db_path: /usr/share/GeoIP/GeoLite2-City.mmdb

clickhouse:
  enabled: true
  host: clickhouse-cluster.internal.com
  port: 9000
  database: dmarc
  username: parsedmarc
  password: ${CLICKHOUSE_PASSWORD}
  tls: true

http:
  enabled: true
  host: 0.0.0.0
  port: 8080
  rate_limit: 300       # Higher rate limit
  rate_burst: 50        # Higher burst
  max_upload_size: 104857600  # 100MB uploads

imap:
  enabled: true
  host: imap.example.com
  port: 993
  username: dmarc@example.com
  password: ${IMAP_PASSWORD}
  tls: true
  check_interval: 60    # Check every minute
```

## Configuration Validation

parsedmarc-go validates configuration on startup and provides helpful error messages:

```bash
parsedmarc-go -config config.yaml
```

Common validation errors:

- **Invalid log level**: Must be debug, info, warn, or error
- **Invalid port**: Must be between 1-65535
- **Missing TLS files**: cert_file and key_file required when TLS enabled
- **Invalid rate limits**: Must be positive integers

## Security Considerations

### Secrets Management

Never store passwords in plain text configuration files. Use environment variables:

```yaml
clickhouse:
  password: ${CLICKHOUSE_PASSWORD}
imap:
  password: ${IMAP_PASSWORD}
```

### File Permissions

Restrict access to configuration files:

```bash
sudo chown root:parsedmarc /etc/parsedmarc-go/config.yaml
sudo chmod 640 /etc/parsedmarc-go/config.yaml
```

### Network Security

- Use TLS for all external connections
- Restrict HTTP access with firewalls
- Use internal networks for ClickHouse connections

## Troubleshooting

### Common Issues

1. **ClickHouse connection failed**: Check host, port, and credentials
2. **IMAP authentication failed**: Verify username, password, and server settings
3. **TLS certificate errors**: Check certificate paths and validity
4. **Permission denied**: Ensure proper file permissions

### Debug Mode

Enable debug logging for troubleshooting:

```yaml
logging:
  level: debug
  format: console
```

Or use environment variable:

```bash
export LOGGING_LEVEL=debug
parsedmarc-go -daemon
```

## Configuration Reload

Currently, configuration changes require a service restart:

```bash
sudo systemctl restart parsedmarc-go
```

Future versions will support configuration hot-reloading.