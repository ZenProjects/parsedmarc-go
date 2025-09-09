#!/bin/bash

# Script pour lancer les tests d'intégration avec tous les services

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "🚀 Starting integration tests setup..."

# Fonction pour nettoyer les ressources
cleanup() {
    echo "🧹 Cleaning up..."
    cd "${PROJECT_ROOT}"
    docker-compose -f docker-compose.test.yml down -v
}

# Trap pour nettoyer en cas d'interruption
trap cleanup EXIT

# Aller dans le répertoire du projet
cd "${PROJECT_ROOT}"

# Arrêter les services existants
echo "🛑 Stopping existing test services..."
docker-compose -f docker-compose.test.yml down -v

# Démarrer les services de test
echo "🐳 Starting test services..."
docker-compose -f docker-compose.test.yml up -d

# Attendre que tous les services soient prêts
echo "⏳ Waiting for services to be ready..."

# Attendre ClickHouse
echo "  - Waiting for ClickHouse..."
timeout=60
while ! curl -s http://localhost:8123/ping > /dev/null 2>&1; do
    sleep 2
    timeout=$((timeout-2))
    if [ $timeout -le 0 ]; then
        echo "❌ ClickHouse failed to start"
        exit 1
    fi
done

# Attendre Kafka
echo "  - Waiting for Kafka..."
timeout=60
while ! docker exec $(docker-compose -f docker-compose.test.yml ps -q kafka) kafka-broker-api-versions --bootstrap-server localhost:9092 > /dev/null 2>&1; do
    sleep 2
    timeout=$((timeout-2))
    if [ $timeout -le 0 ]; then
        echo "❌ Kafka failed to start"
        exit 1
    fi
done

# Attendre les autres services
sleep 10

echo "✅ All services are ready!"

# Initialiser la base de données ClickHouse
echo "🗃️  Initializing ClickHouse database..."
./scripts/init-clickhouse.sh

# Créer les topics Kafka
echo "📨 Creating Kafka topics..."
docker exec $(docker-compose -f docker-compose.test.yml ps -q kafka) kafka-topics --create --topic parsedmarc-reports --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 || true

# Configurer les variables d'environnement pour les tests
export PARSEDMARC_TEST_MODE=true
export CLICKHOUSE_URL="tcp://localhost:9000"
export CLICKHOUSE_USERNAME="parsedmarc"
export CLICKHOUSE_PASSWORD="test123"
export CLICKHOUSE_DATABASE="parsedmarc_test"
export KAFKA_BROKERS="localhost:9092"

# Lancer les tests d'intégration
echo "🧪 Running integration tests..."
go test -v -tags=integration ./test/integration/... -timeout=10m

# Lancer les tests avec couverture
echo "📊 Running integration tests with coverage..."
go test -v -tags=integration -coverprofile=integration-coverage.out ./test/integration/...
go tool cover -html=integration-coverage.out -o integration-coverage.html

echo "✅ Integration tests completed successfully!"
echo "📈 Coverage report: integration-coverage.html"