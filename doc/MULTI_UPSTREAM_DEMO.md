# CoMPAS Multi-Upstream Routing Demo

This demo shows how to configure multiple upstream services with the CoMPAS Auth Proxy.

## Quick Start

1. **Set up environment variables:**
   ```bash
   export OIDC_PROVIDER_URL="http://localhost:8081/auth/realms/compas"
   export OIDC_CLIENT_ID="compas-auth-proxy"
   export OIDC_CLIENT_SECRET="your-client-secret"
   export OIDC_REDIRECT_URL="http://localhost:8080/auth/callback"
   export SESSION_SECRET="your-very-secret-session-key-minimum-32-chars"
   
   # Multi-upstream configuration (updated delimiter format)
   export UPSTREAM_ROUTES="/api/scl,http://localhost:8082,true;/api/history,http://localhost:8083,true;/api/location,http://localhost:8084,true;/,http://localhost:8085,false"
   ```

2. **Start the auth proxy:**
   ```bash
   go run cmd/main.go
   ```

## Routing Examples

With the above configuration, the following routing will happen:

### SCL Data Service (`/api/scl/*`)
- **Request:** `GET http://localhost:8080/api/scl/files`
- **Routed to:** `GET http://localhost:8082/files` (path stripped)
- **Headers added:**
  - `X-Auth-User: user-sub-id`
  - `X-Auth-Email: user@example.com`
  - `Authorization: Bearer access-token`

### History Viewer Service (`/api/history/*`)
- **Request:** `GET http://localhost:8080/api/history/recent`
- **Routed to:** `GET http://localhost:8083/recent` (path stripped)

### Location Manager Service (`/api/location/*`)
- **Request:** `POST http://localhost:8080/api/location/create`
- **Routed to:** `POST http://localhost:8084/create` (path stripped)

### Frontend Application (`/*`)
- **Request:** `GET http://localhost:8080/dashboard`
- **Routed to:** `GET http://localhost:8085/dashboard` (path preserved)

## Authentication Flow

1. User accesses any protected route (e.g., `/api/scl/files`)
2. Auth proxy checks for valid session
3. If not authenticated, redirects to OIDC provider
4. After successful authentication, user is redirected back
5. Session is created and request is forwarded to appropriate upstream
6. Authentication headers are automatically added

## Testing Endpoints

After authentication, you can test the different services:

```bash
# Test SCL Data Service
curl -b cookies.txt http://localhost:8080/api/scl/files

# Test History Viewer
curl -b cookies.txt http://localhost:8080/api/history/recent

# Test Location Manager  
curl -b cookies.txt http://localhost:8080/api/location/sites

# Test Frontend
curl -b cookies.txt http://localhost:8080/dashboard
```

## Configuration Benefits

- ✅ **Single Entry Point**: All services accessed through one authenticated proxy
- ✅ **Automatic Auth Headers**: User information automatically forwarded to services
- ✅ **Path Flexibility**: Choose whether to strip path prefixes per service
- ✅ **Service Isolation**: Each service can run independently
- ✅ **Easy Scaling**: Add new services by updating configuration
