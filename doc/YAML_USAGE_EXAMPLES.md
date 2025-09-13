# YAML Configuration Usage Examples

The OIDC Gateway now exclusively uses YAML configuration files for better organization and maintainability. Environment variable configuration has been deprecated and removed.

## Quick Start with YAML

1. **Copy an example configuration:**
   ```bash
   cp config.dev.yaml config.yaml
   ```

2. **Edit your configuration:**
   ```bash
   vim config.yaml
   ```

3. **Run with YAML configuration:**
   ```bash
   CONFIG_FILE=config.yaml ./compas-auth-proxy
   ```

## Configuration Method

### YAML Configuration (Required)

Set the `CONFIG_FILE` environment variable to point to your YAML configuration file:

```bash
export CONFIG_FILE=config.yaml
./compas-auth-proxy
```

**Benefits:**
- ✅ Hierarchical, structured configuration
- ✅ Better readability for complex setups
- ✅ Built-in validation and type safety
- ✅ Comments and documentation in config file
- ✅ Version control friendly
- ✅ Support for environment variable overrides for secrets

### Environment Variable Overrides for Secrets

Use YAML for base configuration and environment variables for secrets/overrides:

```bash
export CONFIG_FILE=config.yaml
export OIDC_CLIENT_SECRET=production-secret
export SESSION_SECRET=production-session-key
./compas-auth-proxy
```

This approach is **recommended for production** as it allows you to:
- Keep base configuration in version control (YAML)
- Override sensitive values with environment variables
- Use container orchestration tools (Docker, Kubernetes) for secrets

## Docker Usage Examples

### Docker with YAML Configuration

```bash
# Mount YAML config file
docker run -d \
  --name compas-auth-proxy \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -e CONFIG_FILE=/app/config.yaml \
  -e OIDC_CLIENT_SECRET=${OIDC_CLIENT_SECRET} \
  -e SESSION_SECRET=${SESSION_SECRET} \
  compas-auth-proxy:latest
```

### Docker Compose with YAML

```yaml
version: '3.8'
services:
  compas-auth-proxy:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./config.production.yaml:/app/config.yaml:ro
    environment:
      - CONFIG_FILE=/app/config.yaml
      - OIDC_CLIENT_SECRET=${OIDC_CLIENT_SECRET}
      - SESSION_SECRET=${SESSION_SECRET}
```

## Kubernetes Examples

### ConfigMap for YAML Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: compas-auth-config
data:
  config.yaml: |
    server:
      port: "8080"
      host: "0.0.0.0"
    oidc:
      provider_url: "https://keycloak.example.com/auth/realms/compas"
      client_id: "compas-auth-proxy"
      redirect_url: "https://compas.example.com/auth/callback"
      scopes: "openid,profile,email"
    proxy:
      routes:
        - path: "/api/scl"
          upstream_url: "http://scl-service:8080"
          strip_path: true
        - path: "/"
          upstream_url: "http://frontend-service:80"
          strip_path: false
    security:
      allowed_origins:
        - "https://compas.example.com"
```

### Deployment with ConfigMap and Secrets

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: compas-auth-proxy
spec:
  replicas: 2
  selector:
    matchLabels:
      app: compas-auth-proxy
  template:
    metadata:
      labels:
        app: compas-auth-proxy
    spec:
      containers:
      - name: compas-auth-proxy
        image: compas-auth-proxy:latest
        ports:
        - containerPort: 8080
        env:
        - name: CONFIG_FILE
          value: "/etc/config/config.yaml"
        - name: OIDC_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: compas-secrets
              key: oidc-client-secret
        - name: SESSION_SECRET
          valueFrom:
            secretKeyRef:
              name: compas-secrets
              key: session-secret
        volumeMounts:
        - name: config
          mountPath: /etc/config
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: compas-auth-config
```

## Configuration Validation

The application validates configuration on startup and provides helpful error messages:

```bash
# Missing required field
2025/09/13 05:19:47 Failed to load config: required configuration OIDC Provider URL is not set

# Short session secret
2025/09/13 05:19:47 Failed to load config: session secret must be at least 32 characters long

# Invalid YAML syntax
2025/09/13 05:19:47 Failed to load config: failed to parse YAML config: yaml: line 5: mapping values are not allowed in this context
```

## Migration from Environment Variables to YAML

If you were previously using environment variables, here's how to migrate:

1. **Create YAML configuration:**
   ```bash
   # Start with an example template
   cp config.example.yaml config.yaml
   ```

2. **Convert your environment variables to YAML structure:**
   ```yaml
   # OLD: OIDC_PROVIDER_URL=http://localhost:8081/auth/realms/compas
   # NEW:
   oidc:
     provider_url: "http://localhost:8081/auth/realms/compas"
   
   # OLD: UPSTREAM_ROUTES="/api/scl,http://localhost:8082,true;/,http://localhost:8085,false"
   # NEW:
   proxy:
     routes:
       - path: "/api/scl"
         upstream_url: "http://localhost:8082"
         strip_path: true
       - path: "/"
         upstream_url: "http://localhost:8085"
         strip_path: false
   ```

3. **Test the migration:**
   ```bash
   CONFIG_FILE=config.yaml ./compas-auth-proxy
   ```

4. **Update deployment scripts:**
   ```bash
   # Replace environment variable approach with CONFIG_FILE
   CONFIG_FILE=config.yaml ./compas-auth-proxy
   ```

## Best Practices

### Development
- Use `config.dev.yaml` for local development
- Keep secrets in environment variables or external secret management
- Use structured logging (`format: json`) for better debugging

### Production
- Use `config.production.yaml` as base configuration
- Override secrets with environment variables
- Enable health checks (`health.enabled: true`)
- Use `level: info` or `level: warn` for logging
- Enable TLS (`tls.cert_file` and `tls.key_file`)

### Security
- Never commit secrets to version control
- Use environment variables for `oidc.client_secret` and `session.secret`
- Validate allowed origins in production
- Use strong session secrets (minimum 32 characters)

## Troubleshooting

### Configuration Loading Issues

```bash
# Check which configuration method is being used
CONFIG_FILE=config.yaml ./compas-auth-proxy
# Output: "Loaded configuration from YAML file: config.yaml"

# Or without CONFIG_FILE
./compas-auth-proxy  
# Output: "Loaded configuration from environment variables"
```

### YAML Syntax Validation

```bash
# Validate YAML syntax before deployment
python3 -c "import yaml; yaml.safe_load(open('config.yaml'))" && echo "YAML is valid"
```

### Environment Variable Override Testing

```bash
# Test environment variable overrides
CONFIG_FILE=config.yaml OIDC_CLIENT_SECRET=test-override ./compas-auth-proxy
# The OIDC client secret will use "test-override" instead of the YAML value
```
