# ClickHouse Integration

ClickHouse is the recommended database backend for parsedmarc-go, providing fast analytical queries and efficient storage for DMARC report data.

## Overview

ClickHouse integration provides:
- **High Performance**: Columnar storage optimized for analytical queries
- **Scalability**: Handle millions of DMARC records efficiently
- **Real-time Analytics**: Query data as it arrives
- **Data Compression**: Automatic compression reduces storage costs
- **Partitioning**: Automatic date-based partitioning for performance

## Installation

### ClickHouse Server

#### Docker
```bash
docker run -d \
  --name clickhouse-server \
  -p 8123:8123 \
  -p 9000:9000 \
  --volume clickhouse-data:/var/lib/clickhouse \
  clickhouse/clickhouse-server:latest
```

#### Docker Compose
```yaml
version: '3.8'
services:
  clickhouse:
    image: clickhouse/clickhouse-server:latest
    container_name: clickhouse
    ports:
      - "8123:8123"
      - "9000:9000"
    volumes:
      - clickhouse_data:/var/lib/clickhouse
    environment:
      CLICKHOUSE_DB: dmarc
      CLICKHOUSE_USER: default
      CLICKHOUSE_PASSWORD: ""

volumes:
  clickhouse_data:
```

#### Native Installation

**Ubuntu/Debian:**
```bash
sudo apt-get install -y apt-transport-https ca-certificates dirmngr
sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 8919F6BD2B48D754

echo "deb https://packages.clickhouse.com/deb stable main" | sudo tee \
    /etc/apt/sources.list.d/clickhouse.list
sudo apt-get update

sudo apt-get install -y clickhouse-server clickhouse-client
sudo service clickhouse-server start
```

**RHEL/CentOS:**
```bash
sudo yum install -y yum-utils
sudo yum-config-manager --add-repo https://packages.clickhouse.com/rpm/clickhouse.repo
sudo yum install -y clickhouse-server clickhouse-client
sudo systemctl enable clickhouse-server
sudo systemctl start clickhouse-server
```

### Configuration

Configure parsedmarc-go to use ClickHouse:

```yaml
clickhouse:
  enabled: true
  host: localhost
  port: 9000  # Native protocol port
  username: default
  password: ""
  database: dmarc
  dial_timeout: 10s
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 1h
```

## Database Schema

parsedmarc-go automatically creates the necessary tables and structures:

### Aggregate Reports Tables

#### `dmarc_aggregate_reports`
```sql
CREATE TABLE dmarc_aggregate_reports (
    id UUID DEFAULT generateUUIDv4(),
    org_name String,
    email String,
    extra_contact_info String,
    report_id String,
    begin_date DateTime64(3),
    end_date DateTime64(3),
    domain String,
    adkim String,
    aspf String,
    p String,
    sp String,
    pct UInt32,
    received_at DateTime64(3) DEFAULT now64()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(begin_date)
ORDER BY (domain, begin_date, org_name, report_id)
SETTINGS index_granularity = 8192;
```

#### `dmarc_aggregate_records`
```sql
CREATE TABLE dmarc_aggregate_records (
    report_id String,
    source_ip_address IPv4,
    source_country String DEFAULT 'Unknown',
    source_reverse_dns String DEFAULT '',
    source_base_domain String DEFAULT '',
    count UInt32,
    disposition String,
    dkim_aligned UInt8,
    spf_aligned UInt8,
    dmarc_aligned UInt8,
    header_from String,
    envelope_from String DEFAULT '',
    envelope_to String DEFAULT '',
    dkim_domain String DEFAULT '',
    dkim_selector String DEFAULT '',
    dkim_result String DEFAULT '',
    spf_domain String DEFAULT '',
    spf_scope String DEFAULT '',
    spf_result String DEFAULT '',
    begin_date DateTime64(3),
    received_at DateTime64(3) DEFAULT now64()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(begin_date)
ORDER BY (begin_date, source_ip_address, header_from, report_id)
SETTINGS index_granularity = 8192;
```

### Forensic Reports Table

#### `dmarc_forensic_reports`
```sql
CREATE TABLE dmarc_forensic_reports (
    id UUID DEFAULT generateUUIDv4(),
    feedback_type String,
    user_agent String DEFAULT '',
    version String DEFAULT '',
    original_mail_from String DEFAULT '',
    original_rcpt_to String DEFAULT '',
    arrival_date DateTime64(3),
    subject String DEFAULT '',
    message_id String DEFAULT '',
    authentication_results String DEFAULT '',
    delivery_result String DEFAULT '',
    auth_failure String DEFAULT '',
    reported_domain String,
    reported_uri String DEFAULT '',
    source_ip_address IPv4,
    source_country String DEFAULT 'Unknown',
    source_reverse_dns String DEFAULT '',
    source_base_domain String DEFAULT '',
    sample String DEFAULT '',
    received_at DateTime64(3) DEFAULT now64()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(arrival_date)
ORDER BY (arrival_date, reported_domain, source_ip_address, id)
SETTINGS index_granularity = 8192;
```

### SMTP TLS Reports Table

#### `smtp_tls_reports`
```sql
CREATE TABLE smtp_tls_reports (
    id UUID DEFAULT generateUUIDv4(),
    organization_name String,
    date_range_begin DateTime64(3),
    date_range_end DateTime64(3),
    contact_info String DEFAULT '',
    report_id String,
    policy_type String DEFAULT '',
    policy_string String DEFAULT '',
    policy_domain String,
    policy_mx_host String DEFAULT '',
    total_successful_session_count UInt32 DEFAULT 0,
    total_failure_session_count UInt32 DEFAULT 0,
    result_type String DEFAULT '',
    sending_mta_ip IPv4,
    receiving_mx_hostname String DEFAULT '',
    receiving_mx_helo String DEFAULT '',
    receiving_ip IPv4,
    failed_session_count UInt32 DEFAULT 0,
    additional_information String DEFAULT '',
    failure_reason_code String DEFAULT '',
    received_at DateTime64(3) DEFAULT now64()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(date_range_begin)
ORDER BY (date_range_begin, policy_domain, sending_mta_ip, report_id)
SETTINGS index_granularity = 8192;
```

## Indexing and Optimization

### Automatic Indexes

parsedmarc-go creates optimized indexes for common query patterns:

```sql
-- Date-based index for time range queries
ALTER TABLE dmarc_aggregate_records 
ADD INDEX idx_begin_date begin_date TYPE minmax GRANULARITY 3;

-- Domain index for filtering by domain
ALTER TABLE dmarc_aggregate_records 
ADD INDEX idx_header_from header_from TYPE bloom_filter GRANULARITY 1;

-- IP address index for source analysis
ALTER TABLE dmarc_aggregate_records 
ADD INDEX idx_source_ip source_ip_address TYPE bloom_filter GRANULARITY 1;

-- Organization index for aggregate reports
ALTER TABLE dmarc_aggregate_reports 
ADD INDEX idx_org_name org_name TYPE bloom_filter GRANULARITY 1;
```

### Materialized Views

For better query performance on large datasets, create materialized views:

#### Daily Summary View
```sql
CREATE MATERIALIZED VIEW dmarc_daily_summary
ENGINE = SummingMergeTree()
ORDER BY (date, domain, org_name)
AS SELECT
    toDate(begin_date) as date,
    domain,
    org_name,
    sum(count) as total_messages,
    sumIf(count, dmarc_aligned = 1) as aligned_messages,
    sumIf(count, disposition = 'reject') as rejected_messages,
    sumIf(count, disposition = 'quarantine') as quarantined_messages,
    uniq(source_ip_address) as unique_sources
FROM dmarc_aggregate_records
GROUP BY date, domain, org_name;
```

#### Domain Compliance View
```sql
CREATE MATERIALIZED VIEW domain_compliance_summary
ENGINE = ReplacingMergeTree()
ORDER BY (domain, date)
AS SELECT
    domain,
    toDate(begin_date) as date,
    sum(count) as total_messages,
    sumIf(count, dmarc_aligned = 1) as pass_messages,
    round((sumIf(count, dmarc_aligned = 1) * 100.0) / sum(count), 2) as pass_rate,
    uniq(org_name) as reporting_orgs,
    uniq(source_ip_address) as unique_sources
FROM dmarc_aggregate_records
GROUP BY domain, date;
```

#### Source IP Analysis View
```sql
CREATE MATERIALIZED VIEW source_ip_analysis
ENGINE = ReplacingMergeTree()
ORDER BY (source_ip_address, date)
AS SELECT
    source_ip_address,
    toDate(begin_date) as date,
    source_country,
    source_reverse_dns,
    source_base_domain,
    sum(count) as total_messages,
    sumIf(count, dmarc_aligned = 1) as pass_messages,
    uniq(domain) as unique_domains,
    uniq(org_name) as reporting_orgs
FROM dmarc_aggregate_records
GROUP BY source_ip_address, date, source_country, source_reverse_dns, source_base_domain;
```

## Performance Tuning

### Connection Pool Settings

```yaml
clickhouse:
  max_open_conns: 50      # Increase for high throughput
  max_idle_conns: 10      # Keep connections alive
  conn_max_lifetime: 1h   # Connection reuse
  dial_timeout: 30s       # Connection timeout
```

### Batch Processing

```yaml
clickhouse:
  batch_size: 1000       # Insert records in batches
  batch_timeout: 5s      # Maximum wait time for batch
  async_insert: true     # Use async inserts for better performance
```

### Memory Settings

Configure ClickHouse memory limits in `/etc/clickhouse-server/config.xml`:

```xml
<clickhouse>
    <max_memory_usage>10000000000</max_memory_usage> <!-- 10GB -->
    <max_bytes_before_external_group_by>8000000000</max_bytes_before_external_group_by>
    <max_bytes_before_external_sort>8000000000</max_bytes_before_external_sort>
</clickhouse>
```

### Partitioning Strategy

Default partitioning by month is optimal for most use cases:
- Efficient for time-range queries
- Balanced partition sizes
- Automatic cleanup of old data

For high-volume deployments, consider daily partitioning:
```sql
PARTITION BY toYYYYMMDD(begin_date)
```

## Common Queries

### DMARC Compliance Rate
```sql
SELECT 
    domain,
    round((sumIf(count, dmarc_aligned = 1) * 100.0) / sum(count), 2) as compliance_rate,
    sum(count) as total_messages
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 30 day
GROUP BY domain
ORDER BY total_messages DESC;
```

### Top Failing Source IPs
```sql
SELECT 
    source_ip_address,
    source_country,
    source_reverse_dns,
    sum(count) as failed_messages,
    uniq(domain) as affected_domains
FROM dmarc_aggregate_records 
WHERE dmarc_aligned = 0 
  AND begin_date >= now() - interval 7 day
GROUP BY source_ip_address, source_country, source_reverse_dns
ORDER BY failed_messages DESC
LIMIT 20;
```

### Daily Message Volume
```sql
SELECT 
    toDate(begin_date) as date,
    sum(count) as total_messages,
    sumIf(count, dmarc_aligned = 1) as pass_messages,
    sumIf(count, disposition = 'reject') as rejected_messages
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 30 day
GROUP BY date
ORDER BY date;
```

### Forensic Report Analysis
```sql
SELECT 
    reported_domain,
    count() as report_count,
    uniq(source_ip_address) as unique_sources,
    groupArray(DISTINCT auth_failure) as failure_types
FROM dmarc_forensic_reports 
WHERE arrival_date >= now() - interval 7 day
GROUP BY reported_domain
ORDER BY report_count DESC;
```

## Data Retention

### Automatic Cleanup

Set up TTL for automatic data cleanup:

```sql
-- Keep aggregate data for 2 years
ALTER TABLE dmarc_aggregate_reports MODIFY TTL received_at + INTERVAL 2 YEAR;
ALTER TABLE dmarc_aggregate_records MODIFY TTL received_at + INTERVAL 2 YEAR;

-- Keep forensic data for 6 months (privacy considerations)
ALTER TABLE dmarc_forensic_reports MODIFY TTL received_at + INTERVAL 6 MONTH;

-- Keep TLS reports for 1 year
ALTER TABLE smtp_tls_reports MODIFY TTL received_at + INTERVAL 1 YEAR;
```

### Manual Cleanup

Remove old partitions manually:
```sql
-- List partitions
SELECT name, active FROM system.parts WHERE table = 'dmarc_aggregate_records';

-- Drop old partitions (example: data older than 2 years)
ALTER TABLE dmarc_aggregate_records DROP PARTITION '202201';
```

## Backup and Recovery

### Backup Script
```bash
#!/bin/bash
BACKUP_DIR="/backup/clickhouse"
DATE=$(date +%Y%m%d_%H%M%S)

# Create backup
clickhouse-client --query "BACKUP DATABASE dmarc TO File('$BACKUP_DIR/dmarc_$DATE')"

# Compress backup
cd $BACKUP_DIR
tar -czf "dmarc_$DATE.tar.gz" "dmarc_$DATE"
rm -rf "dmarc_$DATE"

# Clean old backups (keep 30 days)
find $BACKUP_DIR -name "dmarc_*.tar.gz" -mtime +30 -delete
```

### Recovery
```bash
# Extract backup
tar -xzf dmarc_20240101_120000.tar.gz

# Restore database
clickhouse-client --query "RESTORE DATABASE dmarc FROM File('/backup/clickhouse/dmarc_20240101_120000')"
```

## Monitoring

### Key Metrics to Monitor

1. **Query Performance**
   ```sql
   SELECT query, query_duration_ms, memory_usage, read_rows
   FROM system.query_log
   WHERE event_time > now() - INTERVAL 1 HOUR
   ORDER BY query_duration_ms DESC LIMIT 10;
   ```

2. **Disk Usage**
   ```sql
   SELECT 
       database,
       table,
       formatReadableSize(sum(data_compressed_bytes)) as compressed_size,
       formatReadableSize(sum(data_uncompressed_bytes)) as uncompressed_size,
       round(sum(data_compressed_bytes) / sum(data_uncompressed_bytes), 4) as compression_ratio
   FROM system.parts
   WHERE database = 'dmarc'
   GROUP BY database, table;
   ```

3. **Insert Performance**
   ```sql
   SELECT 
       toDate(event_time) as date,
       count() as insert_queries,
       avg(query_duration_ms) as avg_duration_ms,
       sum(ProfileEvents['InsertedRows']) as total_rows
   FROM system.query_log
   WHERE query_kind = 'Insert' AND database = 'dmarc'
   GROUP BY date
   ORDER BY date DESC;
   ```

### Health Checks

Create a simple health check query:
```sql
SELECT count() as total_reports, max(received_at) as latest_report
FROM dmarc_aggregate_reports;
```

## Troubleshooting

### Common Issues

#### Connection Problems
- Verify ClickHouse is running: `sudo systemctl status clickhouse-server`
- Check port accessibility: `netstat -tlnp | grep 9000`
- Test connection: `clickhouse-client --host localhost --port 9000`

#### Performance Issues
- Check query log for slow queries
- Verify indexes are being used with `EXPLAIN PLAN`
- Monitor memory usage during queries
- Consider adding more memory or optimizing queries

#### Data Inconsistencies
- Check for duplicate report IDs
- Verify data types match between source and target
- Monitor insert errors in logs

#### Disk Space Issues
- Set up TTL policies for automatic cleanup
- Monitor disk usage regularly
- Consider compression settings

### Debug Queries

Test data insertion:
```sql
-- Check recent inserts
SELECT count(), max(received_at) FROM dmarc_aggregate_reports;

-- Verify data consistency
SELECT report_id, count() FROM dmarc_aggregate_records GROUP BY report_id HAVING count() > 1000;

-- Check partition health
SELECT partition, active, rows FROM system.parts WHERE table = 'dmarc_aggregate_records';
```

## Security

### User Management

Create dedicated users with limited permissions:

```sql
-- Create read-only user for Grafana
CREATE USER grafana_reader IDENTIFIED BY 'secure_password';
GRANT SELECT ON dmarc.* TO grafana_reader;

-- Create application user for parsedmarc-go
CREATE USER parsedmarc_writer IDENTIFIED BY 'secure_password';
GRANT SELECT, INSERT ON dmarc.* TO parsedmarc_writer;
```

### Network Security

Configure ClickHouse to accept connections only from trusted hosts:

```xml
<!-- /etc/clickhouse-server/config.xml -->
<clickhouse>
    <listen_host>127.0.0.1</listen_host>
    <interserver_http_port>9009</interserver_http_port>
    
    <users>
        <default>
            <networks>
                <ip>127.0.0.1</ip>
                <ip>192.168.1.0/24</ip>
            </networks>
        </default>
    </users>
</clickhouse>
```

### Encryption

Enable TLS for secure connections:

```xml
<clickhouse>
    <https_port>8443</https_port>
    <tcp_port_secure>9440</tcp_port_secure>
    
    <openSSL>
        <server>
            <certificateFile>/etc/clickhouse-server/server.crt</certificateFile>
            <privateKeyFile>/etc/clickhouse-server/server.key</privateKeyFile>
        </server>
    </openSSL>
</clickhouse>
```

Update parsedmarc-go configuration:
```yaml
clickhouse:
  host: localhost
  port: 9440
  username: parsedmarc_writer
  password: secure_password
  database: dmarc
  tls: true
```