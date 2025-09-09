# Installation

parsedmarc-go offers multiple installation methods to suit different environments and use cases.

## Prerequisites

- **Operating System**: Linux, macOS, or Windows
- **Architecture**: x86_64 or ARM64
- **Optional**: Docker for containerized deployment
- **Optional**: ClickHouse database for data storage
- **Optional**: MaxMind GeoLite2 database for IP geolocation

## Method 1: Binary Release (Recommended)

Download the pre-compiled binary for your platform:

### Linux x86_64
```bash
curl -L -o parsedmarc-go https://github.com/ZenProjects/parsedmarc-go/releases/latest/download/parsedmarc-go-linux-amd64
chmod +x parsedmarc-go
sudo mv parsedmarc-go /usr/local/bin/
```

### Linux ARM64
```bash
curl -L -o parsedmarc-go https://github.com/ZenProjects/parsedmarc-go/releases/latest/download/parsedmarc-go-linux-arm64
chmod +x parsedmarc-go
sudo mv parsedmarc-go /usr/local/bin/
```

### macOS x86_64
```bash
curl -L -o parsedmarc-go https://github.com/ZenProjects/parsedmarc-go/releases/latest/download/parsedmarc-go-darwin-amd64
chmod +x parsedmarc-go
sudo mv parsedmarc-go /usr/local/bin/
```

### macOS ARM64 (M1/M2)
```bash
curl -L -o parsedmarc-go https://github.com/ZenProjects/parsedmarc-go/releases/latest/download/parsedmarc-go-darwin-arm64
chmod +x parsedmarc-go
sudo mv parsedmarc-go /usr/local/bin/
```

### Windows x86_64
```powershell
# Download from GitHub releases page and add to PATH
Invoke-WebRequest -Uri "https://github.com/ZenProjects/parsedmarc-go/releases/latest/download/parsedmarc-go-windows-amd64.exe" -OutFile "parsedmarc-go.exe"
```

## Method 2: Docker

### Pull from Registry
```bash
docker pull parsedmarc/parsedmarc-go:latest
```

### Build from Source
```bash
git clone https://github.com/ZenProjects/parsedmarc-go.git
cd parsedmarc-go
docker build -t parsedmarc-go .
```

### Run Container
```bash
# Create config directory
mkdir -p /opt/parsedmarc-go

# Copy example config
docker run --rm parsedmarc-go cat /app/config.yaml.example > /opt/parsedmarc-go/config.yaml

# Edit configuration
nano /opt/parsedmarc-go/config.yaml

# Run daemon
docker run -d --name parsedmarc-go \
  -p 8080:8080 \
  -v /opt/parsedmarc-go/config.yaml:/app/config.yaml \
  -v /opt/parsedmarc-go/data:/app/data \
  parsedmarc-go:latest -daemon
```

## Method 3: Build from Source

### Prerequisites
- Go 1.21 or later
- Git

### Build Steps
```bash
# Clone repository
git clone https://github.com/ZenProjects/parsedmarc-go.git
cd parsedmarc-go

# Download dependencies
go mod download

# Build binary
make build
# OR manually:
go build -o parsedmarc-go ./cmd/parsedmarc-go

# Install binary
sudo cp build/parsedmarc-go /usr/local/bin/
```

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
make test-coverage
```

### Cross-Compilation
```bash
# Build for different platforms
make build-linux    # Linux x86_64
make build-darwin   # macOS x86_64
make build-windows  # Windows x86_64
make build-all      # All platforms
```

## Verification

Verify the installation:

```bash
parsedmarc-go -version
```

Expected output:
```
parsedmarc-go version 1.0.0
```

## Configuration

Create a basic configuration file:

```bash
# Copy example configuration
curl -L -o config.yaml https://raw.githubusercontent.com/ZenProjects/parsedmarc-go/main/config.yaml.example

# Edit configuration
nano config.yaml
```

Minimal configuration for getting started:

```yaml
logging:
  level: info
  format: console

clickhouse:
  enabled: true
  host: localhost
  port: 9000
  database: dmarc

http:
  enabled: true
  port: 8080
```

## System Service

### systemd (Linux)

Create a service file:

```bash
sudo tee /etc/systemd/system/parsedmarc-go.service <<EOF
[Unit]
Description=parsedmarc-go DMARC Report Processor
After=network.target

[Service]
Type=simple
User=parsedmarc
Group=parsedmarc
ExecStart=/usr/local/bin/parsedmarc-go -daemon -config /etc/parsedmarc-go/config.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
```

Create user and directories:

```bash
sudo useradd -r -s /bin/false parsedmarc
sudo mkdir -p /etc/parsedmarc-go
sudo cp config.yaml /etc/parsedmarc-go/
sudo chown -R parsedmarc:parsedmarc /etc/parsedmarc-go
```

Enable and start service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable parsedmarc-go
sudo systemctl start parsedmarc-go
```

### Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  parsedmarc-go:
    image: parsedmarc-go:latest
    container_name: parsedmarc-go
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./data:/app/data
    command: ["-daemon"]
    restart: unless-stopped
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
      CLICKHOUSE_USER: default
      CLICKHOUSE_PASSWORD: ""

volumes:
  clickhouse_data:
```

Start with:

```bash
docker-compose up -d
```

## Health Check

Verify the service is running:

```bash
# Check HTTP endpoint
curl http://localhost:8080/health

# Check metrics
curl http://localhost:8080/metrics
```

## Next Steps

1. **Configure data sources**: Set up IMAP or HTTP endpoints
2. **Set up ClickHouse**: Configure database connection
3. **Configure Grafana**: Create dashboards for visualization
4. **Set up monitoring**: Configure Prometheus scraping

See the [Configuration](configuration.md) guide for detailed setup instructions.