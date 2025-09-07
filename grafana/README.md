# Grafana Dashboards for parsedmarc-go

This directory contains Grafana dashboards and configurations for visualizing DMARC report data processed by parsedmarc-go with ClickHouse backend.

## Directory Structure

```
grafana/
├── clickhouse/                    # ClickHouse-specific dashboards
│   ├── dashboard-dmarc-overview.json   # Main overview dashboard
│   ├── dashboard-dmarc-forensic.json   # Forensic reports dashboard
│   ├── datasource.json                 # ClickHouse data source configuration
│   └── README.md                       # Detailed setup instructions
├── grafana-dmarc-reports*.png     # Dashboard screenshots
└── README.md                      # This file
```

## Quick Start

### 1. Install ClickHouse Plugin

```bash
grafana-cli plugins install grafana-clickhouse-datasource
```

Or with Docker:
```bash
docker run -d -p 3000:3000 \
  -e "GF_INSTALL_PLUGINS=grafana-clickhouse-datasource" \
  grafana/grafana:latest
```

### 2. Configure Data Source

Import the data source configuration from `clickhouse/datasource.json` or create manually:

- **Type**: ClickHouse
- **URL**: `http://localhost:8123`
- **Database**: `dmarc`
- **Username**: `default`
- **Password**: (empty for default)

### 3. Import Dashboards

Import the dashboard JSON files from the `clickhouse/` directory:

1. Go to **Dashboards** → **Import**
2. Upload `dashboard-dmarc-overview.json`
3. Upload `dashboard-dmarc-forensic.json`
4. Select your ClickHouse data source

## Dashboard Overview

### DMARC Overview Dashboard
![DMARC Overview](grafana-dmarc-reports00.png)

Features:
- Message volume trends
- DMARC compliance rates
- Policy disposition analysis
- Geographic distribution
- Top source IPs and organizations

### Forensic Reports Dashboard
![Forensic Reports](grafana-dmarc-reports01.png)

Features:
- Forensic report trends
- Authentication failure analysis
- Detailed failure breakdowns
- Source IP forensic analysis

## Requirements

- Grafana 8.0+
- ClickHouse data source plugin
- parsedmarc-go with ClickHouse integration
- ClickHouse database with DMARC tables

## Configuration

See `clickhouse/README.md` for detailed configuration instructions including:

- Data source setup
- Dashboard customization
- Performance optimization
- Security considerations

## Docker Compose Example

```yaml
version: '3.8'
services:
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_INSTALL_PLUGINS=grafana-clickhouse-datasource
    volumes:
      - ./grafana/clickhouse:/var/lib/grafana/dashboards
    depends_on:
      - clickhouse
      
  clickhouse:
    image: clickhouse/clickhouse-server:latest
    ports:
      - "8123:8123"
      - "9000:9000"
    environment:
      CLICKHOUSE_DB: dmarc
```

## Support

For detailed setup instructions and troubleshooting, see:
- [ClickHouse Integration Guide](../docs/clickhouse.md)
- [Grafana Configuration Guide](../docs/grafana.md)
- [parsedmarc-go Documentation](../docs/index.md)