# Dockerfile multi-stage pour parsedmarc-go avec Alpine Linux

# Stage 1: Builder
FROM golang:1.23-alpine AS builder

# Installer les dépendances de build nécessaires
RUN apk add --no-cache git ca-certificates tzdata

# Créer un utilisateur non-root pour la sécurité
RUN adduser -D -g '' appuser

# Définir le répertoire de travail
WORKDIR /build

# Copier les fichiers de dépendances Go
COPY go.mod go.sum ./

# Télécharger les dépendances
RUN go mod download

# Copier le code source
COPY . .

# Construire l'application avec optimisations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o parsedmarc-go \
    ./cmd/parsedmarc-go

# Stage 2: Image finale
FROM alpine:3.19

# Installer les certificats CA et timezone data
RUN apk --no-cache add ca-certificates tzdata

# Créer un utilisateur non-root
RUN adduser -D -g '' appuser

# Créer les répertoires nécessaires
RUN mkdir -p /app/reports /app/config

# Copier l'utilisateur depuis le builder
COPY --from=builder /etc/passwd /etc/passwd

# Copier le binaire compilé
COPY --from=builder /build/parsedmarc-go /app/parsedmarc-go

# Copier les fichiers de configuration exemple
COPY --from=builder /build/config.yaml.example /app/config.yaml.example

# Définir les permissions
RUN chown -R appuser:appuser /app

# Changer vers l'utilisateur non-root
USER appuser

# Définir le répertoire de travail
WORKDIR /app

# Exposer le port par défaut (ajustez selon votre configuration)
EXPOSE 8080

# Définir les volumes pour la configuration et les rapports
VOLUME ["/app/config", "/app/reports"]

# Point d'entrée
ENTRYPOINT ["./parsedmarc-go"]

# Commande par défaut
CMD ["-config", "/app/config/config.yaml"]