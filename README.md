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

- Go 1.21 oder höher
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

3. **Umgebungsvariablen konfigurieren:**
   ```bash
   cp .env.example .env
   # Bearbeiten Sie die .env-Datei mit Ihren Werten
   ```

4. **Anwendung starten:**
   ```bash
   make run
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

Die Anwendung wird über Umgebungsvariablen konfiguriert:

### Server-Konfiguration
| Variable | Beschreibung | Standard |
|----------|--------------|----------|
| `PORT` | Server-Port | `8080` |
| `HOST` | Server-Host | `0.0.0.0` |

### OpenID Connect
| Variable | Beschreibung | Erforderlich |
|----------|--------------|--------------|
| `OIDC_PROVIDER_URL` | OIDC Provider URL | ✅ |
| `OIDC_CLIENT_ID` | Client ID | ✅ |
| `OIDC_CLIENT_SECRET` | Client Secret | ✅ |
| `OIDC_REDIRECT_URL` | Redirect URL | ✅ |
| `OIDC_SCOPES` | OAuth2 Scopes | `openid,profile,email` |

### Proxy-Konfiguration
| Variable | Beschreibung | Erforderlich |
|----------|--------------|--------------|
| `UPSTREAM_URL` | Backend Service URL | ✅ |
| `SESSION_SECRET` | Session Encryption Key | ✅ |
| `SESSION_COOKIE_NAME` | Cookie Name | `compas-auth-session` |
| `SESSION_MAX_AGE` | Session Timeout (Sekunden) | `3600` |

### Sicherheit
| Variable | Beschreibung | Standard |
|----------|--------------|----------|
| `ALLOWED_ORIGINS` | Erlaubte CORS Origins | `*` |
| `TLS_CERT_FILE` | TLS Zertifikat Pfad | - |
| `TLS_KEY_FILE` | TLS Key Pfad | - |
| `INSECURE_SKIP_VERIFY` | TLS Verifikation überspringen | `false` |

## API Endpoints

### Authentifizierung
- `GET /auth/callback` - OIDC Callback Endpoint
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
     -e OIDC_PROVIDER_URL=https://your-oidc-provider.com \
     -e OIDC_CLIENT_ID=your-client-id \
     -e OIDC_CLIENT_SECRET=your-client-secret \
     -e OIDC_REDIRECT_URL=https://your-domain.com/auth/callback \
     -e UPSTREAM_URL=https://your-backend.com \
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
   - Überprüfen Sie die `OIDC_PROVIDER_URL`
   - Stellen Sie sicher, dass der Provider erreichbar ist

2. **"Invalid upstream URL"**
   - Überprüfen Sie die `UPSTREAM_URL` Formatierung
   - Stellen Sie sicher, dass das Backend erreichbar ist

3. **"Session not found"**
   - Session ist abgelaufen oder ungültig
   - Benutzer wird automatisch zur Anmeldung weitergeleitet

### Debug-Logs aktivieren

Für detaillierte Logs können Sie das Log-Level erhöhen:

```bash
export LOG_LEVEL=debug
./compas-auth-proxy
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
