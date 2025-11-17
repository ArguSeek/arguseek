# Production Security Guide

ArguSeek is designed to be **open-by-default** for simplicity and ease of local development.

**⚠️ For production deployments using HTTP mode, you MUST add security controls externally.**

## Transport Modes and Security

ArguSeek supports two transport modes with different security characteristics:

### Stdio Mode (Default)
When running without the `-http` flag, ArguSeek communicates via stdin/stdout. This is the default and recommended mode for:
- **Local development** with MCP clients like Claude Code CLI
- **Subprocess integration** where the client manages the process lifecycle

**Security characteristics:**
- ✅ No network exposure (no open ports)
- ✅ Process isolation managed by the client
- ✅ No authentication needed (relies on OS-level process permissions)
- ✅ Ideal for single-user local development

**Security considerations for stdio mode:**
- Ensure API keys in environment variables are protected at the OS level
- The calling process has full access to ArguSeek's capabilities
- Suitable for trusted local environments only

### HTTP Mode (`-http` flag)
When running with the `-http` flag, ArguSeek starts an HTTP server on port 8080. This mode is required for:
- **Docker containers** (networking requires exposed ports)
- **Remote deployments** (Cloud Run, VPS, etc.)
- **Multi-client access** (multiple users or services)

**Security characteristics:**
- ⚠️ Network-exposed endpoint
- ⚠️ No built-in authentication or authorization
- ⚠️ Anyone with network access can use the server and consume API quota
- ✅ Can be secured externally (see strategies below)

**When to use HTTP mode:**
- Deploying to cloud platforms (Cloud Run, AWS, Azure)
- Using Docker Compose for team deployments
- Need to share one ArguSeek instance across multiple clients

## Authentication Strategies (HTTP Mode Only)

The following authentication strategies apply only when running ArguSeek in HTTP mode with the `-http` flag.

### Option 1: Reverse Proxy Authentication

Use a reverse proxy (nginx, Caddy, Traefik, or Apache) to add authentication in front of the ArguSeek server.

**Example with nginx and Basic Auth:**

```nginx
server {
    listen 443 ssl;
    server_name arguseek.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        auth_basic "ArguSeek API";
        auth_basic_user_file /etc/nginx/.htpasswd;

        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Example with Caddy and API keys:**

```caddyfile
arguseek.example.com {
    @authorized {
        header Authorization "Bearer YOUR_SECRET_TOKEN"
    }

    handle @authorized {
        reverse_proxy localhost:8080
    }

    handle {
        respond "Unauthorized" 401
    }
}
```

### Option 2: Cloud Run IAM Authentication

When deploying to Google Cloud Run, enable IAM authentication to restrict access.

**Deploy with IAM:**

```bash
gcloud run deploy arguseek-server \
  --source . \
  --region us-central1 \
  --no-allow-unauthenticated \
  --set-env-vars GOOGLE_API_KEY=your-key,GOOGLE_CSE_ID=your-cse-id
```

**Grant access to specific service accounts:**

```bash
gcloud run services add-iam-policy-binding arguseek-server \
  --region=us-central1 \
  --member='serviceAccount:client@project.iam.gserviceaccount.com' \
  --role='roles/run.invoker'
```

**Client authentication:**

Clients must include an identity token in requests:

```bash
curl -H "Authorization: Bearer $(gcloud auth print-identity-token)" \
  https://arguseek-prod-xxx.a.run.app/health
```

### Option 3: API Gateway with Custom Authorizer

Use AWS API Gateway, Google Cloud Endpoints, or Azure API Management to add:
- API key validation
- JWT token verification
- Rate limiting
- Request throttling

**Example with AWS API Gateway + Lambda Authorizer:**

1. Create Lambda authorizer to validate API keys
2. Configure API Gateway to proxy requests to ArguSeek
3. Attach authorizer to all routes

### Option 4: VPN / Private Network

Deploy ArguSeek in a private network (VPC, private subnet) and require VPN access.

**Benefits:**
- Network-level isolation
- No application-level auth needed
- Suitable for internal tools

**Considerations:**
- Requires VPN infrastructure
- May complicate CI/CD pipelines

## Rate Limiting

ArguSeek does not implement rate limiting. Add it at the infrastructure layer:

**Option 1: Reverse Proxy Rate Limiting**

**nginx:**
```nginx
http {
    limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;

    server {
        location /mcp {
            limit_req zone=api burst=20 nodelay;
            proxy_pass http://localhost:8080;
        }
    }
}
```

**Caddy:**
```caddyfile
arguseek.example.com {
    rate_limit {
        zone api {
            key {remote_host}
            events 10
            window 1s
        }
    }
    reverse_proxy localhost:8080
}
```

**Option 2: Cloud Run Concurrency Limits**

```bash
gcloud run deploy arguseek-server \
  --concurrency 80 \
  --max-instances 10
```

**Option 3: API Gateway Throttling**

Configure per-client quotas in your API Gateway.

## OAuth Discovery Endpoint

ArguSeek exposes `/.well-known/oauth-authorization-server` for MCP client compatibility.

**What it does:**
- Returns minimal OAuth metadata (`{"issuer":"..."}`)
- Signals to MCP clients that **no OAuth authentication is required**
- Does **not** implement actual OAuth flows

**Configuration:**

Set the `OAUTH_ISSUER` environment variable to your service's public URL:

```bash
export OAUTH_ISSUER="https://arguseek.example.com"
```

If not set, defaults to `http://localhost:8080` for local development.

**Security Note:** This endpoint is informational only and does not perform authentication. Secure your service using one of the strategies above.

## Input Validation

ArguSeek validates URLs for the `fetch_url` tool to prevent SSRF attacks. Additional validation is minimal by design.

**If you need stricter validation:**
1. Use a reverse proxy to inspect requests
2. Add WAF rules (ModSecurity, AWS WAF, Cloudflare WAF)
3. Fork the codebase and add custom validation in `internal/mcp/handler.go`

## HTTPS / TLS

ArguSeek listens on HTTP (port 8080 by default) without TLS.

**For production:**
- Terminate TLS at a reverse proxy or load balancer
- Use Cloud Run (provides automatic HTTPS)
- Use a CDN (Cloudflare, Fastly) with TLS

**Never expose the HTTP port directly to the internet.**

## Secrets Management

ArguSeek requires API keys via environment variables:
- `GOOGLE_API_KEY` (required)
- `GOOGLE_CSE_ID` (required)
- `GEMINI_API_KEY` (optional, falls back to `GOOGLE_API_KEY`)

**Best practices:**
- Use a secrets manager (GCP Secret Manager, AWS Secrets Manager, HashiCorp Vault)
- Never commit secrets to version control
- Rotate keys regularly
- Use service-specific keys (don't share keys across environments)

**Example with GCP Secret Manager:**

```bash
gcloud run deploy arguseek-server \
  --source . \
  --set-secrets GOOGLE_API_KEY=google-api-key:latest,GOOGLE_CSE_ID=google-cse-id:latest
```

## Logging and Monitoring

ArguSeek logs to stdout/stderr. In production:

**Structured logging:**
- Logs are JSON-formatted (when `DEBUG=true`)
- Include request IDs for tracing

**Monitor:**
- Request rates
- Error rates (check for 500 errors in `internal/mcp/handler.go`)
- API quota usage (Google APIs)
- Response times

**Tools:**
- Cloud Logging (GCP)
- CloudWatch (AWS)
- Application Insights (Azure)
- Self-hosted: ELK stack, Grafana Loki

## Summary Checklist

Before deploying to production:

- [ ] Add authentication (reverse proxy, IAM, API gateway, or VPN)
- [ ] Configure rate limiting
- [ ] Set `OAUTH_ISSUER` environment variable
- [ ] Terminate TLS externally (reverse proxy or cloud platform)
- [ ] Use secrets manager for API keys
- [ ] Configure logging and monitoring
- [ ] Test authentication and authorization
- [ ] Document your security setup for your team

**Remember:** ArguSeek is designed to be secured externally. Choose the authentication strategy that fits your infrastructure and compliance requirements.
