# Stage 1: Builder
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Benötigte Build-Tools installieren
RUN apk add --no-cache git ca-certificates tzdata build-base

# Abhängigkeiten kopieren und herunterladen
COPY go.mod go.sum ./
RUN go mod download

# Quellcode kopieren
COPY . .

# Anwendung bauen
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o double-take ./cmd/server

# Stage 2: Runner
FROM alpine:3.16

WORKDIR /app

# Abhängigkeiten installieren
RUN apk add --no-cache ca-certificates tzdata

# Zeitzone-Daten beibehalten
ENV TZ=Europe/Berlin

# Anwendungsbinär vom Builder-Stage kopieren
COPY --from=builder /app/double-take /app/double-take

# Web-Dateien kopieren
COPY --from=builder /app/web /app/web

# Datenverzeichnisse erstellen
RUN mkdir -p /data/snapshots /config

# Volumes definieren
VOLUME ["/data", "/config"]

# Port freigeben
EXPOSE 3000

# Startbefehl
CMD ["/app/double-take"]
