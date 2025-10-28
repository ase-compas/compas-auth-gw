# CoMPAS Auth Proxy

Ein moderner Authentication Proxy für das CoMPAS (Common Platform for Substation Automation) System, entwickelt in Go mit OpenID Connect (OIDC) Support.

## Features

- 🔐 **OpenID Connect Authentication** - Sichere Benutzerauthentifizierung
- 🔄 **Reverse Proxy** - Weiterleitung authentifizierter Anfragen an Backend-Services
- 🍪 **Session Management** - Sichere Session-Verwaltung mit konfigurierbarer Ablaufzeit
- 🌐 **CORS Support** - Konfigurierbare Cross-Origin Resource Sharing
- 📊 **Health Checks** - Überwachung der Anwendungsgesundheit
- 🐳 **Docker Support** - Containerisierung für einfache Bereitstellung
- 🔒 **TLS Support** - HTTPS-Unterstützung für Produktionsumgebungen

## Architektur

```
┌─────────────┐    ┌──────────────────┐    ┌─────────────┐
│   Client    │───▶│  Auth Proxy      │───▶│  Backend    │
│             │    │  (OIDC + Proxy)  │    │  Services   │
└─────────────┘    └──────────────────┘    └─────────────┘
                            │
                            ▼
                   ┌─────────────────┐
                   │  OIDC Provider  │
                   │  (e.g. Keycloak)│
                   └─────────────────┘
```

## Schnellstart

### Voraussetzungen

- Go 1.20 oder höher
- Docker und Docker Compose (optional)
- OIDC Provider (z.B. Keycloak)

### Installation

1. **Repository klonen:**
   ```bash
   git clone <repository-url>
   cd compas-auth-proxy
   ```

2. **Dependencies installieren:**
   ```bash
   go mod tidy
   ```

3. **Konfiguration erstellen:**
   ```bash
   cp config.example.yaml config.yaml
   # Bearbeiten Sie config.yaml mit Ihren Werten
   ```

4. **Anwendung starten:**
   ```bash
   # Option 1: Use default config.yaml
   go run ./cmd
   
   # Option 2: Specify config file explicitly
   CONFIG_FILE=config.yaml go run ./cmd
   ```

### Mit Docker

1. **Mit Docker Compose starten:**
   ```bash
   make docker-run
   ```

2. **Logs anzeigen:**
   ```bash
   make docker-logs
   ```

## Konfiguration

Die Anwendung verwendet YAML-Konfigurationsdateien für eine strukturierte und lesbare Konfiguration.

### Konfigurationsdateien

- `config.example.yaml` - Beispielkonfiguration mit allen Optionen
- `config.dev.yaml` - Entwicklungsoptimierte Konfiguration
- `config.production.yaml` - Produktions-Template mit ausführlichen Kommentaren

### Grundlegende YAML-Struktur

```yaml
# Server-Konfiguration
server:
  port: "8080"
  host: "0.0.0.0"

# OpenID Connect
oidc:
  provider_url: "http://localhost:8081/auth/realms/compas"
  client_id: "compas-auth-proxy"
  client_secret: "ihr-client-secret"
  redirect_url: "http://localhost:8080/oidc/callback"
  scopes: "openid,profile,email"

# Session-Verwaltung
session:
  secret: "ihr-sehr-sicherer-session-schlüssel-mindestens-32-zeichen"
  cookie_name: "compas-session"
  max_age: 3600

# Multi-Upstream Proxy
proxy:
  routes:
    - path: "/api/scl"
      upstream_url: "http://scl-service:8081"
      strip_path: true
    - path: "/"
      upstream_url: "http://frontend:80"
      strip_path: false

# Sicherheit
security:
  allowed_origins:
    - "http://localhost:3000"
    - "http://localhost:8080"
```

### Konfigurationsdatei

Die Anwendung lädt automatisch eine Konfigurationsdatei in folgender Reihenfolge:
1. Datei aus `CONFIG_FILE` Umgebungsvariable (falls gesetzt)
2. `config.yaml` im aktuellen Verzeichnis (Standard-Fallback)

### Umgebungsvariablen-Overrides

Sensible Werte können über Umgebungsvariablen überschrieben werden:

```bash
# Option 1: Mit expliziter Konfigurationsdatei
export CONFIG_FILE=config.yaml
export OIDC_CLIENT_SECRET=produktions-secret
export SESSION_SECRET=produktions-session-key
./compas-auth-proxy

# Option 2: Mit Standard config.yaml
export OIDC_CLIENT_SECRET=produktions-secret
export SESSION_SECRET=produktions-session-key
./compas-auth-proxy
### Multi-Upstream Routing

Das Multi-Upstream-System ermöglicht es, verschiedene URL-Pfade zu unterschiedlichen Backend-Services zu routen:

#### Routing-Beispiel:
```yaml
proxy:
  routes:
    - path: "/api/scl"
      upstream_url: "http://scl-service:8081"
      strip_path: true
    - path: "/api/history" 
      upstream_url: "http://history-service:8082"
      strip_path: true
    - path: "/api/location"
      upstream_url: "http://location-service:8083" 
      strip_path: true
    - path: "/"
      upstream_url: "http://frontend:80"
      strip_path: false
```

#### Routing-Regeln:
1. **Längste Übereinstimmung gewinnt**: Spezifischere Pfade haben Vorrang vor allgemeineren
2. **Pfad-Matching**: Ein Pfad `/api/scl` matched `/api/scl`, `/api/scl/`, `/api/scl/files`, etc.
3. **Root-Pfad**: Der Pfad `/` fungiert als Fallback für alle nicht gematchten Anfragen
4. **Authentifizierung**: Alle konfigurierten Routen erfordern eine gültige Authentifizierung
5. **Strip Path**: `true` = Pfad-Präfix entfernen, `false` = vollständigen Pfad beibehalten

## API Endpoints

### Authentifizierung
- `GET /oidc/callback` - OIDC Callback Endpoint
- `GET /auth/logout` - Benutzer Logout
- `GET /auth/userinfo` - Benutzerinformationen abrufen

### System
- `GET /health` - Health Check Endpoint

### Proxy
- `*` - Alle anderen Anfragen werden an den Backend-Service weitergeleitet

## Entwicklung

### Verfügbare Make-Targets

```bash
make help                 # Alle verfügbaren Targets anzeigen
make build               # Anwendung kompilieren
make run                 # Anwendung starten
make dev                 # Hot-Reload Entwicklung (erfordert air)
make test                # Tests ausführen
make test-coverage       # Tests mit Coverage
make lint                # Code Linting
make fmt                 # Code Formatierung
make security            # Sicherheitsscan
make docker-build        # Docker Image erstellen
make docker-run          # Mit Docker Compose starten
make generate-certs      # Selbstsignierte Zertifikate erstellen
```

### Entwicklungstools installieren

```bash
make install-tools
```

### Hot Reload Entwicklung

```bash
make dev
```

## Docker Deployment

### Mit GitHub Actions (CI/CD)

Das Projekt verwendet GitHub Actions für automatisierte Builds und Docker Image Publishing:

- **Entwicklung**: Jeder Push zu `main` oder `develop` erstellt automatisch ein Docker Image
- **Releases**: Veröffentlichungen erstellen getaggte Images auf Docker Hub und GitHub Container Registry
- Mehr Details: [GitHub Actions Workflows](.github/WORKFLOWS.md)

#### Images abrufen:

```bash
# Von GitHub Container Registry (automatisch bei jedem Build)
docker pull ghcr.io/ase-compas/compas-auth-gw:latest
docker pull ghcr.io/ase-compas/compas-auth-gw:v1.0.0

# Von Docker Hub (nur bei Releases)
docker pull <username>/compas-auth-gw:latest
```

### Produktions-Deployment

1. **Image erstellen:**
   ```bash
   docker build -t compas-auth-proxy:latest .
   ```

2. **Container starten:**
   ```bash
   docker run -d \
     --name compas-auth-proxy \
     -p 8080:8080 \
     -v $(pwd)/config.yaml:/app/config.yaml:ro \
     -e CONFIG_FILE=/app/config.yaml \
     -e OIDC_CLIENT_SECRET=your-client-secret \
     -e SESSION_SECRET=your-session-secret \
     compas-auth-proxy:latest
   ```

### Kubernetes Deployment

Beispiel Kubernetes-Manifeste sind im `k8s/` Verzeichnis verfügbar.

## Sicherheitsaspekte

- 🔐 Sichere Session-Verwaltung mit verschlüsselten Cookies
- 🛡️ CSRF-Schutz durch SameSite-Cookie-Attribut
- 🔒 TLS-Unterstützung für Produktionsumgebungen
- 🚫 Sichere Header-Weiterleitung an Backend-Services
- ⏰ Konfigurierbare Session-Timeouts
- 🔍 Automatische Bereinigung abgelaufener Sessions

## Monitoring

### Health Checks

```bash
curl http://localhost:8080/health
```

Antwort:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Metriken

Die Anwendung loggt alle HTTP-Anfragen mit:
- HTTP-Methode
- URL-Pfad
- Status-Code
- Anfragedauer
- Client-IP

## Fehlerbehebung

### Häufige Probleme

1. **"Failed to discover OIDC provider"**
   - Überprüfen Sie die `oidc.provider_url` in der YAML-Konfiguration
   - Stellen Sie sicher, dass der Provider erreichbar ist

2. **"Invalid upstream URL"**
   - Überprüfen Sie die `proxy.routes` Konfiguration in der YAML-Datei
   - Stellen Sie sicher, dass die Backend-Services erreichbar sind

3. **"Session not found"**
   - Session ist abgelaufen oder ungültig
   - Benutzer wird automatisch zur Anmeldung weitergeleitet

4. **"CONFIG_FILE environment variable not set and default config.yaml not found"**
   - Erstellen Sie eine `config.yaml` Datei im aktuellen Verzeichnis, oder
   - Setzen Sie `CONFIG_FILE=pfad/zu/ihrer/config.yaml` als Umgebungsvariable
   - Stellen Sie sicher, dass die YAML-Datei existiert und gültig ist

### Debug-Logs aktivieren

Für detaillierte Logs setzen Sie das Log-Level in der YAML-Konfiguration:

```yaml
logging:
  level: "debug"
  format: "text"  # oder "json" für strukturierte Logs
```

## Lizenz

MIT License - siehe [LICENSE](LICENSE) Datei für Details.

## Beitragen

1. Fork das Repository
2. Erstellen Sie einen Feature Branch (`git checkout -b feature/amazing-feature`)
3. Committen Sie Ihre Änderungen (`git commit -m 'Add amazing feature'`)
4. Pushen Sie den Branch (`git push origin feature/amazing-feature`)
5. Öffnen Sie eine Pull Request

## Support

Bei Fragen oder Problemen öffnen Sie bitte ein Issue im Repository.

---

**CoMPAS Auth Proxy** - Sichere Authentifizierung für das CoMPAS Ökosystem
