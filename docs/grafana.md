# Grafana Dashboards

This guide covers setting up Grafana dashboards for visualizing DMARC report data stored in ClickHouse.

## Overview

Grafana provides powerful visualization capabilities for DMARC report analysis. With ClickHouse as the data source, you can create interactive dashboards showing:

- DMARC compliance trends over time
- Source IP analysis and geolocation
- Domain authentication results
- Email volume and disposition statistics
- Forensic report details

## Installation

### Grafana Setup

#### Docker Compose (Recommended)

```yaml
version: '3.8'
services:
  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/provisioning:/etc/grafana/provisioning
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
      GF_INSTALL_PLUGINS: grafana-clickhouse-datasource
    depends_on:
      - clickhouse

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

volumes:
  grafana_data:
  clickhouse_data:
```

#### Native Installation

**Ubuntu/Debian:**
```bash
sudo apt-get install -y software-properties-common
sudo add-apt-repository "deb https://packages.grafana.com/oss/deb stable main"
wget -q -O - https://packages.grafana.com/gpg.key | sudo apt-key add -
sudo apt-get update
sudo apt-get install grafana
sudo systemctl enable grafana-server
sudo systemctl start grafana-server
```

**RHEL/CentOS:**
```bash
sudo yum install -y https://dl.grafana.com/oss/release/grafana-9.0.0-1.x86_64.rpm
sudo systemctl enable grafana-server
sudo systemctl start grafana-server
```

### ClickHouse Plugin

Install the ClickHouse data source plugin:

```bash
# Using grafana-cli
sudo grafana-cli plugins install grafana-clickhouse-datasource

# Or add to Docker environment
GF_INSTALL_PLUGINS=grafana-clickhouse-datasource
```

## Data Source Configuration

### Add ClickHouse Data Source

1. Access Grafana at `http://localhost:3000` (admin/admin)
2. Go to **Configuration** > **Data Sources**
3. Click **Add data source**
4. Select **ClickHouse**
5. Configure connection:

```yaml
# Data source settings
Name: DMARC ClickHouse
URL: http://clickhouse:8123
Database: dmarc
Username: default
Password: (leave empty for default)
```

### Connection Settings

**HTTP Settings:**
- URL: `http://clickhouse:8123` (Docker) or `http://localhost:8123`
- Timeout: 30s
- HTTP Method: POST

**Database Settings:**
- Default Database: `dmarc`
- Username: `default`
- Password: (empty for default setup)

**Advanced Settings:**
- Use Compression: Yes
- Session Timeout: 60s
- Query Timeout: 30s

### Test Connection

Click **Save & Test** to verify the connection works correctly.

## Dashboard Templates

### Import Pre-built Dashboards

The project includes pre-built dashboard templates in the `grafana/` directory:

1. **DMARC Overview Dashboard**: General statistics and trends
2. **DMARC Forensic Dashboard**: Detailed forensic report analysis
3. **Source Analysis Dashboard**: IP source and geolocation analysis

#### Import via UI

1. Go to **Dashboards** > **Import**
2. Upload the JSON files from `grafana/clickhouse/` directory
3. Select **DMARC ClickHouse** as the data source

#### Automated Provisioning

Create provisioning configuration:

```yaml
# grafana/provisioning/datasources/clickhouse.yaml
apiVersion: 1
datasources:
  - name: DMARC ClickHouse
    type: grafana-clickhouse-datasource
    access: proxy
    url: http://clickhouse:8123
    database: dmarc
    user: default
    isDefault: true
```

```yaml
# grafana/provisioning/dashboards/dmarc.yaml
apiVersion: 1
providers:
  - name: 'DMARC Dashboards'
    folder: 'DMARC'
    type: file
    options:
      path: /etc/grafana/provisioning/dashboards/dmarc
```

## Dashboard Panels

### DMARC Compliance Rate

Track overall DMARC compliance over time:

```sql
SELECT 
    toDate(begin_date) as date,
    round((sumIf(count, dmarc_aligned = 1) * 100.0) / sum(count), 2) as compliance_rate
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 30 day
GROUP BY date
ORDER BY date
```

**Panel Configuration:**
- Visualization: Time series
- Y-Axis: Percentage (0-100)
- Unit: Percent
- Color: Green for high values, red for low values

### Message Volume by Disposition

Show email message volume by DMARC disposition:

```sql
SELECT 
    toDate(begin_date) as date,
    disposition,
    sum(count) as message_count
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 30 day
GROUP BY date, disposition
ORDER BY date
```

**Panel Configuration:**
- Visualization: Stacked bar chart
- Legend: Show disposition types
- Colors: Red (reject), Yellow (quarantine), Green (none)

### Top Failing Source IPs

Identify sources sending non-compliant emails:

```sql
SELECT 
    source_ip_address,
    source_country,
    sum(count) as failed_messages
FROM dmarc_aggregate_records 
WHERE dmarc_aligned = 0 
  AND begin_date >= now() - interval 7 day
GROUP BY source_ip_address, source_country
ORDER BY failed_messages DESC
LIMIT 20
```

**Panel Configuration:**
- Visualization: Table
- Columns: IP Address, Country, Failed Messages
- Sort: By failed messages (descending)

### Geographic Distribution

Map showing email sources by country:

```sql
SELECT 
    source_country as country,
    sum(count) as message_count,
    round((sumIf(count, dmarc_aligned = 1) * 100.0) / sum(count), 2) as compliance_rate
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 7 day
  AND source_country != 'Unknown'
GROUP BY source_country
ORDER BY message_count DESC
```

**Panel Configuration:**
- Visualization: Geomap
- Map type: World map
- Value field: message_count
- Color scale: Based on compliance_rate

### Domain Analysis

Track performance across different domains:

```sql
SELECT 
    domain,
    sum(count) as total_messages,
    round((sumIf(count, dmarc_aligned = 1) * 100.0) / sum(count), 2) as compliance_rate,
    uniq(org_name) as reporting_orgs
FROM dmarc_aggregate_records r
JOIN dmarc_aggregate_reports ar ON r.report_id = ar.report_id
WHERE begin_date >= now() - interval 30 day
GROUP BY domain
ORDER BY total_messages DESC
```

**Panel Configuration:**
- Visualization: Table
- Columns: Domain, Total Messages, Compliance Rate, Reporting Orgs
- Thresholds: Color code compliance rates

### Authentication Failures

Break down DKIM and SPF failures:

```sql
SELECT 
    toDate(begin_date) as date,
    multiIf(
        dkim_aligned = 0 AND spf_aligned = 0, 'Both Failed',
        dkim_aligned = 0, 'DKIM Failed',
        spf_aligned = 0, 'SPF Failed',
        'All Passed'
    ) as auth_status,
    sum(count) as message_count
FROM dmarc_aggregate_records
WHERE begin_date >= now() - interval 30 day
GROUP BY date, auth_status
ORDER BY date, auth_status
```

**Panel Configuration:**
- Visualization: Stacked area chart
- Legend: Show authentication status
- Fill: 0.5 opacity

### Forensic Reports Summary

When forensic reports are available:

```sql
SELECT 
    toDate(arrival_date) as date,
    reported_domain,
    count() as report_count,
    groupArray(DISTINCT auth_failure) as failure_types
FROM dmarc_forensic_reports 
WHERE arrival_date >= now() - interval 7 day
GROUP BY date, reported_domain
ORDER BY date DESC, report_count DESC
```

**Panel Configuration:**
- Visualization: Table
- Columns: Date, Domain, Report Count, Failure Types
- Auto-refresh: 5 minutes

## Advanced Queries

### Weekly Compliance Trends

```sql
SELECT 
    toStartOfWeek(begin_date) as week,
    round((sumIf(count, dmarc_aligned = 1) * 100.0) / sum(count), 2) as compliance_rate,
    sum(count) as total_messages
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 12 week
GROUP BY week
ORDER BY week
```

### Source IP Reputation Analysis

```sql
SELECT 
    source_ip_address,
    source_reverse_dns,
    uniq(domain) as unique_domains,
    sum(count) as total_messages,
    round((sumIf(count, dmarc_aligned = 1) * 100.0) / sum(count), 2) as compliance_rate,
    max(begin_date) as last_seen
FROM dmarc_aggregate_records
WHERE begin_date >= now() - interval 30 day
GROUP BY source_ip_address, source_reverse_dns
HAVING total_messages > 100
ORDER BY compliance_rate ASC, total_messages DESC
```

### Policy Effectiveness

```sql
SELECT 
    domain,
    p as policy,
    sum(count) as total_messages,
    sumIf(count, disposition = 'reject') as rejected,
    sumIf(count, disposition = 'quarantine') as quarantined,
    round((sumIf(count, disposition IN ('reject', 'quarantine')) * 100.0) / sum(count), 2) as action_rate
FROM dmarc_aggregate_records r
JOIN dmarc_aggregate_reports ar ON r.report_id = ar.report_id
WHERE begin_date >= now() - interval 30 day
GROUP BY domain, p
ORDER BY total_messages DESC
```

## Dashboard Variables

### Time Range Variable

Create a custom time range variable:

```sql
-- Query Type: Query
-- Query:
SELECT value, text FROM (
    VALUES
    ('now()-1h', 'Last Hour'),
    ('now()-24h', 'Last 24 Hours'),
    ('now()-7d', 'Last 7 Days'),
    ('now()-30d', 'Last 30 Days'),
    ('now()-90d', 'Last 90 Days')
)
```

### Domain Filter

Create a domain selection variable:

```sql
-- Query Type: Query
-- Query:
SELECT DISTINCT domain
FROM dmarc_aggregate_reports
WHERE begin_date >= now() - interval 90 day
ORDER BY domain
```

### Country Filter

Create a country selection variable:

```sql
-- Query Type: Query  
-- Query:
SELECT DISTINCT source_country
FROM dmarc_aggregate_records
WHERE source_country != 'Unknown'
  AND begin_date >= now() - interval 30 day
ORDER BY source_country
```

## Alerting

### Compliance Rate Alert

Set up alerts for low compliance rates:

```sql
SELECT 
    round((sumIf(count, dmarc_aligned = 1) * 100.0) / sum(count), 2) as compliance_rate
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 24 hour
```

**Alert Conditions:**
- Threshold: compliance_rate < 90
- Evaluation: Every 1h for 2h
- Notification: Email, Slack, etc.

### Volume Anomaly Alert

Alert on unusual email volume:

```sql
SELECT sum(count) as total_messages
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 1 hour
```

**Alert Conditions:**
- Threshold: total_messages > (average * 3)
- Evaluation: Every 15m
- Notification: Immediate

## Performance Optimization

### Dashboard Performance

1. **Use time range limits**: Always include time filters
2. **Limit result sets**: Use LIMIT in queries
3. **Use materialized views**: Pre-aggregate common queries
4. **Cache queries**: Enable dashboard caching
5. **Optimize refresh rates**: Don't refresh too frequently

### Query Optimization

```sql
-- Good: Uses time filter and limits results
SELECT source_ip_address, sum(count) as messages
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 7 day
GROUP BY source_ip_address
ORDER BY messages DESC
LIMIT 20

-- Bad: No time filter, unlimited results
SELECT source_ip_address, sum(count) as messages
FROM dmarc_aggregate_records 
GROUP BY source_ip_address
ORDER BY messages DESC
```

## Troubleshooting

### Common Issues

1. **Data source connection failed**
   - Verify ClickHouse is running
   - Check network connectivity
   - Validate credentials

2. **No data in panels**
   - Check data source selection
   - Verify time range
   - Test queries manually

3. **Slow dashboard performance**
   - Add time filters to all queries
   - Use LIMIT clauses
   - Consider materialized views

4. **Plugin not working**
   - Verify plugin installation
   - Check Grafana logs
   - Restart Grafana service

### Debug Mode

Enable Grafana debug logging:

```ini
# /etc/grafana/grafana.ini
[log]
level = debug
```

Check ClickHouse query logs:
```sql
SELECT query, query_duration_ms, memory_usage
FROM system.query_log
WHERE event_time > now() - interval 1 hour
ORDER BY query_duration_ms DESC
```

## Best Practices

1. **Dashboard Organization**
   - Group related panels together
   - Use consistent naming conventions
   - Add panel descriptions
   - Set appropriate refresh rates

2. **Query Design**
   - Always include time filters
   - Use appropriate aggregation levels
   - Limit result sets
   - Test queries in ClickHouse first

3. **Visual Design**
   - Choose appropriate visualizations
   - Use consistent color schemes
   - Set meaningful thresholds
   - Configure proper units and formats

4. **Performance**
   - Monitor dashboard load times
   - Optimize slow queries
   - Use caching where appropriate
   - Consider using materialized views

For more advanced configuration and customization, see the [ClickHouse Integration Guide](clickhouse.md).