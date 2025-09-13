# YAML Configuration Guide

# YAML Configuration Guide

The OIDC Gateway uses YAML configuration files for structured and maintainable configuration. Environment variable configuration has been removed in favor of YAML for better organization and type safety.

**Note**: As of this version, environment variables are no longer supported for base configuration. Only YAML configuration files are accepted, with environment variable overrides available for sensitive values.

## Configuration Files

Three example YAML configuration files are provided:

- `config.example.yaml` - Basic example with all configuration options
- `config.dev.yaml` - Minimal development configuration  
- `config.production.yaml` - Production-ready configuration with detailed comments

## Configuration Structure

The YAML configuration is organized into logical sections:

### Server Configuration
```yaml
server:
  port: 8080
  host: "0.0.0.0"
```

### TLS Configuration
```yaml
tls:
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"
  insecure_skip_verify: false
```

### OpenID Connect Configuration
```yaml
oidc:
  provider_url: "http://localhost:8081/auth/realms/compas"
  client_id: "compas-auth-proxy"
  client_secret: "your-client-secret-here"
  redirect_url: "http://localhost:8080/auth/callback"
  scopes: "openid,profile,email"
```

### Session Management
```yaml
session:
  secret: "your-very-secret-session-key-here-minimum-32-chars"
  cookie_name: "compas-auth-session"
  max_age: 3600
```

### Multi-Upstream Proxy Configuration
```yaml
proxy:
  routes:
    - path: "/api/scl"
      upstream_url: "http://localhost:8082"
      strip_path: true
    - path: "/api/history"
      upstream_url: "http://localhost:8083"
      strip_path: true
    - path: "/"
      upstream_url: "http://localhost:8085"
      strip_path: false
```

### Security Configuration
```yaml
security:
  allowed_origins:
    - "http://localhost:3000"
    - "http://localhost:8080"
```

### Logging Configuration
```yaml
logging:
  level: "info"  # debug, info, warn, error
  format: "json"  # json, text
```

## Usage

### YAML Configuration (Required)

The application requires a YAML configuration file. You can either:
- Set the `CONFIG_FILE` environment variable to point to your configuration file, or
- Create a `config.yaml` file in the current directory (default fallback)

1. **Copy an example configuration:**
   ```bash
   cp config.example.yaml config.yaml
   ```

2. **Edit the configuration:**
   ```bash
   vim config.yaml
   ```

3. **Run with YAML configuration:**
   ```bash
   # Option 1: Use default config.yaml
   ./compas-auth-proxy
   
   # Option 2: Specify config file explicitly
   CONFIG_FILE=config.yaml ./compas-auth-proxy
   ```

### Docker Usage with YAML

Mount the configuration file as a volume:

```bash
docker run -d \
  --name compas-auth-proxy \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -e CONFIG_FILE=/app/config.yaml \
  compas-auth-proxy:latest
```

### Docker Compose with YAML

See `docker-compose.yaml.example` for a complete example using YAML configuration.

## Configuration Validation

The application validates the configuration on startup and will report any missing required fields or invalid values.

Required fields:
- `oidc.provider_url`
- `oidc.client_id`
- `oidc.client_secret`
- `oidc.redirect_url`
- `session.secret` (minimum 32 characters)
- `proxy.routes` (at least one route)

## Environment Variable Override

Even when using YAML configuration, you can override sensitive values with environment variables for security:

```bash
export OIDC_CLIENT_SECRET="production-secret"
export SESSION_SECRET="production-session-key"
CONFIG_FILE=config.yaml ./compas-auth-proxy
```

This is the recommended approach for production environments where you want to keep secrets out of configuration files.

## Configuration Precedence

The configuration is loaded in the following order (later sources override earlier ones):

1. YAML configuration file (base configuration)
2. Environment variable overrides for sensitive values

This allows for flexible deployment scenarios where base configuration comes from YAML files and secrets come from environment variables or orchestration systems.
