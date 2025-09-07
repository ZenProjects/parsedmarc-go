# Grafana Dashboards for ClickHouse

This directory contains Grafana dashboards specifically designed for parsedmarc-go with ClickHouse backend.

## Dashboard Overview

### 1. DMARC Reports Overview (`dashboard-dmarc-overview.json`)
- **Purpose**: High-level overview of DMARC aggregate reports
- **Key Metrics**:
  - Daily message volume trends
  - DMARC compliance rates
  - Policy action distribution
  - Top reporting organizations
  - Source IP analysis
  - Geographic distribution

### 2. DMARC Forensic Reports (`dashboard-dmarc-forensic.json`)
- **Purpose**: Detailed analysis of forensic (failure) reports
- **Key Metrics**:
  - Forensic report counts and trends
  - Feedback types distribution
  - Delivery results analysis
  - Top reported domains
  - Source IP forensic analysis

## Prerequisites

1. **ClickHouse Data Source Plugin**
   ```bash
   grafana-cli plugins install grafana-clickhouse-datasource
   # OR for Docker
   docker run -d -p 3000:3000 -e "GF_INSTALL_PLUGINS=grafana-clickhouse-datasource" grafana/grafana
   ```

2. **parsedmarc-go Running**
   - ClickHouse database with DMARC data
   - Tables created by parsedmarc-go

## Installation

### Method 1: Manual Import

1. Open Grafana web interface
2. Navigate to **Dashboards** → **Import**
3. Copy and paste the JSON content from dashboard files
4. Configure data source connection

### Method 2: Provisioning (Recommended)

Create provisioning configuration:

#### Data Source (`/etc/grafana/provisioning/datasources/clickhouse.yaml`)

```yaml
apiVersion: 1

datasources:
  - name: ClickHouse DMARC
    type: grafana-clickhouse-datasource
    access: proxy
    url: http://localhost:8123
    database: dmarc
    basicAuth: false
    isDefault: true
    jsonData:
      server: localhost
      port: 8123
      username: default
      defaultDatabase: dmarc
      dialTimeout: 10s
      maxIdleConns: 10
      maxOpenConns: 10
      connMaxLifetime: 14400s
    secureJsonData:
      password: ""
```

#### Dashboards (`/etc/grafana/provisioning/dashboards/dashboards.yaml`)

```yaml
apiVersion: 1

providers:
  - name: DMARC ClickHouse
    orgId: 1
    folder: DMARC
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /var/lib/grafana/dashboards/dmarc-clickhouse
```

Copy dashboard files to `/var/lib/grafana/dashboards/dmarc-clickhouse/`

### Method 3: Docker Compose

```yaml
version: '3.8'

services:
  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    environment:
      - GF_INSTALL_PLUGINS=grafana-clickhouse-datasource
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/provisioning:/etc/grafana/provisioning
      - ./grafana/clickhouse:/var/lib/grafana/dashboards/dmarc-clickhouse
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

## Configuration

### Data Source Connection

1. **Server**: `localhost` (or your ClickHouse host)
2. **Port**: `8123` (HTTP) or `9000` (Native)
3. **Database**: `dmarc`
4. **Username**: `default` (or your ClickHouse user)
5. **Password**: Leave empty for default setup

### Security Considerations

For production deployments:

1. **Create dedicated ClickHouse user**:
   ```sql
   CREATE USER grafana_reader IDENTIFIED BY 'secure_password';
   GRANT SELECT ON dmarc.* TO grafana_reader;
   ```

2. **Use TLS connections**:
   ```yaml
   jsonData:
     server: clickhouse.example.com
     port: 8443
     protocol: https
     tlsSkipVerify: false
   ```

3. **Network isolation**:
   - Use internal networks for ClickHouse
   - Restrict Grafana access with authentication

## Dashboard Customization

### Time Ranges
All dashboards support flexible time ranges:
- Last 24 hours
- Last 7 days (default)
- Last 30 days
- Custom ranges

### Variables
Template variables available:
- `$time_range`: Configurable time range in days
- Add custom domain filters if needed

### Queries Optimization

For large datasets, consider:

1. **Materialized Views**:
   ```sql
   CREATE MATERIALIZED VIEW dmarc_daily_summary
   ENGINE = SummingMergeTree()
   ORDER BY (date, org_name, domain)
   AS SELECT
     toDate(begin_date) as date,
     org_name,
     domain,
     sum(count) as total_messages,
     sumIf(count, dmarc_aligned = 1) as aligned_messages
   FROM dmarc_aggregate_records
   GROUP BY date, org_name, domain;
   ```

2. **Indexes**:
   ```sql
   ALTER TABLE dmarc_aggregate_records 
   ADD INDEX idx_begin_date begin_date TYPE minmax GRANULARITY 3;
   
   ALTER TABLE dmarc_aggregate_records 
   ADD INDEX idx_org_name org_name TYPE bloom_filter GRANULARITY 1;
   ```

### Custom Panels

Add custom panels for your specific needs:

1. **Domain-specific filters**
2. **Custom time aggregations**
3. **Advanced geographic visualizations**
4. **Alert thresholds**

## Performance Tips

1. **Use appropriate time ranges**: Avoid querying years of data
2. **Leverage ClickHouse's columnar storage**: Select only needed columns
3. **Use sampling for large datasets**: `SAMPLE 0.1` for 10% sampling
4. **Create proper partitions**: Partition tables by date

## Troubleshooting

### Common Issues

1. **No data showing**:
   - Verify ClickHouse connection
   - Check if parsedmarc-go is populating data
   - Verify table names and schema

2. **Slow queries**:
   - Add appropriate indexes
   - Use materialized views
   - Optimize query time ranges

3. **Permission errors**:
   - Check ClickHouse user permissions
   - Verify database access

### Debug Queries

Test queries directly in ClickHouse:

```sql
-- Check if data exists
SELECT count() FROM dmarc_aggregate_reports;
SELECT count() FROM dmarc_aggregate_records;

-- Check recent data
SELECT max(begin_date) FROM dmarc_aggregate_records;

-- Test basic aggregation
SELECT toDate(begin_date) as date, sum(count) as messages 
FROM dmarc_aggregate_records 
WHERE begin_date >= now() - interval 7 day 
GROUP BY date 
ORDER BY date;
```

## Dashboard Updates

To update dashboards:

1. **Manual**: Re-import JSON files
2. **Provisioning**: Update files and restart Grafana
3. **Version control**: Keep dashboards in Git for tracking changes

## Contributing

When contributing dashboard improvements:

1. Test with sample data
2. Optimize queries for performance
3. Follow Grafana dashboard best practices
4. Document any new variables or configuration
5. Include screenshots of new panels

## Links

- [Grafana Documentation](https://grafana.com/docs/)
- [ClickHouse Data Source Plugin](https://grafana.com/grafana/plugins/grafana-clickhouse-datasource/)
- [parsedmarc-go Documentation](../docs/source/index.md)