# parsedmarc-go

Une implémentation en Go du parseur de rapports DMARC, basée sur le projet Python original [parsedmarc](https://github.com/domainaware/parsedmarc).

## Fonctionnalités

- ✅ Parsing des rapports DMARC agrégés (XML)
- ✅ Support des fichiers compressés (ZIP, GZIP)
- ✅ Configuration via fichiers YAML avec Viper
- ✅ Logging structuré avec Zap
- ✅ Stockage en base de données ClickHouse
- ✅ Géolocalisation des adresses IP (avec base MaxMind)
- ✅ Résolution DNS inversée
- ✅ Client IMAP pour récupérer les rapports par email
- ✅ Serveur HTTP simple (RFC 7489 conforme)
- ✅ Rate limiting et validation des données
- ✅ Métriques Prometheus intégrées
- ✅ Support TLS/SSL pour IMAP et HTTP
- ✅ Parsing des rapports forensiques
- ✅ Support des rapports SMTP TLS

## Installation

### Prérequis

- Go 1.21 ou supérieur
- ClickHouse (optionnel, pour le stockage)
- Base de données MaxMind GeoLite2 (optionnelle, pour la géolocalisation)

### Compilation

```bash
# Cloner le projet
git clone https://github.com/domainaware/parsedmarc-go
cd parsedmarc-go

# Installer les dépendances
go mod download

# Compiler
go build -o parsedmarc-go ./cmd/parsedmarc-go
```

## Configuration

Copiez le fichier de configuration exemple :

```bash
cp config.yaml.example config.yaml
```

Modifiez la configuration selon vos besoins :

```yaml
# Logging
logging:
  level: info
  format: json
  output_path: stdout

# Parser
parser:
  offline: false
  ip_db_path: "/path/to/GeoLite2-City.mmdb"
  nameservers:
    - "1.1.1.1"
    - "1.0.0.1"
  dns_timeout: 2

# ClickHouse
clickhouse:
  enabled: true
  host: localhost
  port: 9000
  database: dmarc
  username: default
  password: ""
```

## Utilisation

### Parsing d'un fichier unique

```bash
./parsedmarc-go -input report.xml
```

### Parsing d'un répertoire

```bash
./parsedmarc-go -input /path/to/reports/
```

### Mode daemon (IMAP + HTTP)

```bash
./parsedmarc-go -daemon -config config.yaml
```

### Serveur HTTP uniquement

```bash
# Activer HTTP dans config.yaml puis:
./parsedmarc-go -daemon
```

### Client IMAP uniquement

```bash
# Activer IMAP dans config.yaml puis:
./parsedmarc-go -daemon
```

### Avec configuration personnalisée

```bash
./parsedmarc-go -config /path/to/config.yaml -input report.xml
```

### Variables d'environnement

Vous pouvez également utiliser des variables d'environnement pour la configuration :

```bash
export CLICKHOUSE_HOST=clickhouse.example.com
export CLICKHOUSE_PORT=9000
export CLICKHOUSE_USERNAME=myuser
export CLICKHOUSE_PASSWORD=mypassword
export IMAP_HOST=imap.example.com
export IMAP_USERNAME=dmarc@example.com
export IMAP_PASSWORD=password
export HTTP_ENABLED=true
export HTTP_PORT=8080

./parsedmarc-go -daemon
```

## Endpoints HTTP

### RFC 7489 Conforme (Simple)

```bash
# Envoyer un rapport DMARC
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/xml" \
  --data @report.xml

# Envoyer un rapport forensique
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: text/plain" \
  --data @forensic-report.txt

# Envoyer un rapport SMTP TLS
curl -X POST http://localhost:8080/dmarc/report \
  -H "Content-Type: application/json" \
  --data @smtp-tls-report.json
```

### Endpoints de monitoring

```bash
# Santé du service
curl http://localhost:8080/health

# Métriques Prometheus
curl http://localhost:8080/metrics
```

### Métriques Prometheus disponibles

- `parsedmarc_http_requests_total` - Nombre total de requêtes HTTP
- `parsedmarc_http_request_duration_seconds` - Durée des requêtes HTTP
- `parsedmarc_reports_processed_total` - Rapports traités avec succès
- `parsedmarc_reports_failed_total` - Rapports en échec
- `parsedmarc_http_active_connections` - Connexions HTTP actives
- `parsedmarc_report_size_bytes` - Taille des rapports reçus
- `parsedmarc_parser_reports_total` - Rapports parsés par type
- `parsedmarc_parser_failures_total` - Échecs de parsing par type
- `parsedmarc_imap_connections_total` - Tentatives de connexion IMAP
- `parsedmarc_imap_messages_total` - Messages IMAP traités

## Structure ClickHouse

Le programme crée automatiquement les tables suivantes :

### dmarc_aggregate_reports
Table principale pour les métadonnées des rapports agrégés.

### dmarc_aggregate_records
Table pour les enregistrements individuels des rapports agrégés.

### dmarc_forensic_reports
Table pour les rapports forensiques.

## Exemples de requêtes ClickHouse

### Top 10 des domaines les plus reportés

```sql
SELECT 
    domain,
    count() as report_count,
    sum(count) as total_messages
FROM dmarc_aggregate_records 
WHERE begin_date >= today() - 30
GROUP BY domain 
ORDER BY total_messages DESC 
LIMIT 10;
```

### Taux de conformité DMARC par organisation

```sql
SELECT 
    org_name,
    countIf(dmarc_aligned = 1) as aligned_count,
    countIf(dmarc_aligned = 0) as not_aligned_count,
    round((aligned_count / (aligned_count + not_aligned_count)) * 100, 2) as alignment_rate
FROM dmarc_aggregate_records 
WHERE begin_date >= today() - 7
GROUP BY org_name 
ORDER BY alignment_rate DESC;
```

### IPs sources les plus fréquentes

```sql
SELECT 
    source_ip_address,
    source_country,
    source_reverse_dns,
    sum(count) as message_count
FROM dmarc_aggregate_records 
WHERE begin_date >= today() - 7
GROUP BY source_ip_address, source_country, source_reverse_dns
ORDER BY message_count DESC 
LIMIT 20;
```

## Différences avec parsedmarc Python

- **Performance** : Implémentation native Go plus rapide
- **Simplicité** : Moins de dépendances externes
- **ClickHouse natif** : Support intégré pour ClickHouse
- **Configuration** : Configuration via YAML avec variables d'environnement
- **Logging** : Logging structuré avec Zap
- **HTTP simple** : Serveur HTTP minimaliste conforme RFC 7489
- **IMAP robuste** : Client IMAP avec reconnexion automatique
- **Métriques** : Prometheus intégré pour monitoring complet
- **Parsing complet** : Support aggregate, forensic et SMTP TLS

## Développement

### Structure du projet

```
.
├── cmd/
│   └── parsedmarc-go/          # Point d'entrée principal
├── internal/
│   ├── config/                 # Gestion configuration
│   ├── http/                   # Serveur HTTP
│   ├── imap/                   # Client IMAP
│   ├── logger/                 # Configuration logging
│   ├── parser/                 # Logique de parsing
│   ├── storage/
│   │   └── clickhouse/         # Implémentation ClickHouse
│   ├── utils/                  # Utilitaires
│   └── validation/             # Validation des données
├── config.yaml.example        # Exemple de configuration
├── Dockerfile                  # Image Docker
├── Makefile                    # Build et déploiement
└── go.mod                      # Modules Go
```

### Tests

```bash
go test ./...
```

### Contribution

1. Fork le projet
2. Créez une branche feature (`git checkout -b feature/nouvelle-fonctionnalite`)
3. Committez vos changements (`git commit -am 'Ajouter nouvelle fonctionnalité'`)
4. Push vers la branche (`git push origin feature/nouvelle-fonctionnalite`)
5. Créez une Pull Request

## Licence

Ce projet est sous licence Apache 2.0 - voir le fichier [LICENSE](LICENSE) pour plus de détails.

## Remerciements

- [Sean Whalen](https://github.com/seanthegeek) pour le projet Python original parsedmarc
- La communauté Go pour les excellentes bibliothèques utilisées