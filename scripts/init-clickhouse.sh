#!/bin/bash

# Script d'initialisation de ClickHouse pour les tests

set -e

echo "🗃️  Initializing ClickHouse test database..."

# Configuration de connexion
CLICKHOUSE_HOST="localhost"
CLICKHOUSE_PORT="8123"
CLICKHOUSE_USER="parsedmarc"
CLICKHOUSE_PASSWORD="test123"
CLICKHOUSE_DATABASE="parsedmarc_test"

# Fonction pour exécuter des requêtes ClickHouse
run_query() {
    local query="$1"
    curl -s -X POST \
        --user "${CLICKHOUSE_USER}:${CLICKHOUSE_PASSWORD}" \
        --data-binary "$query" \
        "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/"
}

# Créer la base de données
echo "Creating database ${CLICKHOUSE_DATABASE}..."
run_query "CREATE DATABASE IF NOT EXISTS ${CLICKHOUSE_DATABASE}"

# Créer les tables pour les rapports d'agrégation
echo "Creating aggregate_reports table..."
run_query "
USE ${CLICKHOUSE_DATABASE};

CREATE TABLE IF NOT EXISTS aggregate_reports (
    id UUID DEFAULT generateUUIDv4(),
    date_created DateTime DEFAULT now(),
    xml_schema String,
    org_name String,
    org_email String,
    report_id String,
    begin_date DateTime,
    end_date DateTime,
    domain String,
    adkim String,
    aspf String,
    p String,
    sp String,
    pct String,
    fo String
) ENGINE = MergeTree()
ORDER BY (date_created, org_name, report_id);
"

# Créer les tables pour les enregistrements des rapports d'agrégation
echo "Creating aggregate_records table..."
run_query "
USE ${CLICKHOUSE_DATABASE};

CREATE TABLE IF NOT EXISTS aggregate_records (
    id UUID DEFAULT generateUUIDv4(),
    report_id String,
    source_ip IPv4,
    source_country String,
    source_reverse_dns String,
    source_base_domain String,
    count UInt32,
    disposition String,
    dkim String,
    spf String,
    header_from String,
    envelope_from String,
    envelope_to String,
    date_created DateTime DEFAULT now()
) ENGINE = MergeTree()
ORDER BY (date_created, report_id, source_ip);
"

# Créer les tables pour les rapports forensiques
echo "Creating forensic_reports table..."
run_query "
USE ${CLICKHOUSE_DATABASE};

CREATE TABLE IF NOT EXISTS forensic_reports (
    id UUID DEFAULT generateUUIDv4(),
    date_created DateTime DEFAULT now(),
    feedback_type String,
    arrival_date DateTime,
    subject String,
    message_id String,
    authentication_results String,
    source_ip IPv4,
    source_country String,
    delivery_result String,
    auth_failure Array(String),
    reported_domain String
) ENGINE = MergeTree()
ORDER BY (date_created, reported_domain);
"

# Créer les tables pour les rapports SMTP TLS
echo "Creating smtp_tls_reports table..."
run_query "
USE ${CLICKHOUSE_DATABASE};

CREATE TABLE IF NOT EXISTS smtp_tls_reports (
    id UUID DEFAULT generateUUIDv4(),
    date_created DateTime DEFAULT now(),
    organization_name String,
    date_range_begin DateTime,
    date_range_end DateTime,
    contact_info String,
    report_id String,
    policy_domain String
) ENGINE = MergeTree()
ORDER BY (date_created, organization_name, report_id);
"

echo "✅ ClickHouse test database initialized successfully!"

# Vérifier que les tables ont été créées
echo "📊 Verifying tables..."
run_query "
USE ${CLICKHOUSE_DATABASE};
SHOW TABLES;
"

echo "🎉 ClickHouse setup complete!"