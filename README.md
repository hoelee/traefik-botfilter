# Traefik Bot Filter

> A lightweight Traefik middleware plugin that blocks scanners, malformed HTTP requests, and suspicious clients before they reach your backend.

![Go](https://img.shields.io/badge/Go-1.20+-00ADD8?logo=go)
![Traefik](https://img.shields.io/badge/Traefik-v3.x-24A1C1?logo=traefikproxy)
![License](https://img.shields.io/github/license/hoelee/traefik-botfilter)
![GitHub Release](https://img.shields.io/github/v/release/hoelee/traefik-botfilter)

`traefik-botfilter` is a dependency-free Traefik middleware plugin designed to stop common Internet scans, malformed HTTP requests, and low-quality bots before they reach your applications.

Unlike a traditional Web Application Firewall (WAF), Bot Filter focuses on lightweight request validation and heuristic scoring. It requires **no Redis, database, or external services**, making it suitable for self-hosted environments, home labs, and production deployments.

## Features

- 🚫 Block common scanner paths (`/.env`, `/wp-login.php`, `/phpmyadmin`, etc.)
- 🤖 Detect and reject known bot User-Agents
- 📄 Require valid HTTP request headers
- 🧠 Configurable heuristic scoring system
- ⛔ Temporary in-memory IP banning
- 🌐 CIDR whitelist support
- 🔒 Browser header validation
- ⚡ Dependency-free (Go standard library only)
- 💾 Bounded memory usage
- 🔄 Reverse proxy aware
- 📦 No Redis or database required

---

# Quick Start

## 1. Enable the plugin

### Local Plugin (Development)

```yaml
experimental:
  localPlugins:
    botfilter:
      moduleName: github.com/hoelee/traefik-botfilter
```

### Plugin Catalog (Future)

After the plugin is available in the official Traefik Plugin Catalog:

```yaml
experimental:
  plugins:
    botfilter:
      moduleName: github.com/hoelee/traefik-botfilter
      version: v1.0.0
```

> **Note**
>
> The Plugin Catalog installation only works after this repository has been indexed by the official Traefik Plugin Catalog.

---

## 2. Configure the middleware

```yaml
http:
  middlewares:
    botfilter:
      plugin:
        botfilter:

          statusCode: 403

          requireUserAgent: true
          requireAccept: true
          requireHost: true

          browserValidation: true

          temporaryBanMinutes: 15

          scoreThreshold: 100
          scoreWindowMinutes: 15

          whitelistCIDRs:
            - 127.0.0.1/32
            - 192.168.0.0/16

          blockedUserAgents:
            - curl
            - wget
            - python
            - Go-http-client
            - sqlmap
            - nikto
            - masscan

          blockedPaths:
            - /.env
            - /.git
            - /wp-login.php
            - /xmlrpc.php
            - /phpmyadmin

          blockedExtensions:
            - .env
            - .bak
            - .zip

          randomArticlePatterns:
            - /content/

          logBlockedRequests: false
```

---

## 3. Attach the middleware

```yaml
http:
  routers:

    website:
      rule: Host(`example.com`)
      service: website

      middlewares:
        - botfilter
```

---

# Installation

## Local Plugin

Clone the repository into Traefik's local plugin directory.

```
plugins-local/
└── src/
    └── github.com/
        └── hoelee/
            └── traefik-botfilter/
                ├── .traefik.yml
                ├── go.mod
                ├── config.go
                ├── botfilter.go
                └── ...
```

Docker Compose:

```yaml
services:

  traefik:

    image: traefik:v3.5

    restart: unless-stopped

    volumes:

      - /var/run/docker.sock:/var/run/docker.sock:ro

      - ./traefik.yml:/etc/traefik/traefik.yml:ro

      - ./dynamic.yml:/etc/traefik/dynamic.yml:ro

      - ./plugins-local:/plugins-local:ro
```

Restart Traefik after copying the plugin source.

---

# Example Configuration

## traefik.yml

```yaml
experimental:

  localPlugins:

    botfilter:

      moduleName: github.com/hoelee/traefik-botfilter
```

---

## dynamic.yml

```yaml
http:
  middlewares:
    botfilter:
      plugin:
        botfilter:
          statusCode: 403
          requireUserAgent: true
          requireAccept: true
          requireHost: true
          browserValidation: true
          temporaryBanMinutes: 15
          scoreThreshold: 100
          scoreWindowMinutes: 15
          whitelistCIDRs:
            - 127.0.0.1/32
            - 192.168.0.0/16
          blockedUserAgents:
            - curl
            - wget
            - python
            - Go-http-client
          blockedPaths:
            - /.env
            - /.git
            - /wp-login.php
            - /xmlrpc.php
          blockedExtensions:
            - .env
            - .bak
            - .zip
          logBlockedRequests: false
```

# Configuration Reference

| Option | Default | Description |
|---------|:-------:|-------------|
| `statusCode` | `403` | HTTP status code returned for blocked requests. |
| `requireUserAgent` | `false` | Reject requests without a `User-Agent` header. |
| `requireAccept` | `false` | Reject requests without an `Accept` header. |
| `requireHost` | `false` | Reject requests without a `Host` header. |
| `browserValidation` | `false` | Detect inconsistent browser headers. |
| `temporaryBanMinutes` | `15` | Temporary in-memory IP ban duration. |
| `scoreThreshold` | `100` | Score required before an IP is temporarily banned. |
| `scoreWindowMinutes` | `15` | Sliding window used for score accumulation. |
| `maxTrackedIPs` | `50000` | Maximum number of IP addresses stored in memory. |
| `maxScoreEventsPerIP` | `16` | Maximum scoring events stored per client. |
| `whitelistCIDRs` | None | Clients that bypass all filtering. |
| `blockedUserAgents` | None | User-Agent substrings immediately rejected. |
| `blockedPaths` | None | URL paths that are immediately rejected. |
| `blockedExtensions` | None | Dangerous file extensions to reject. |
| `randomArticlePatterns` | `/content/` | URL prefixes that contribute to suspicion scoring. |
| `clientIPHeader` | Empty | Header containing the real client IP when behind trusted proxies. |
| `trustedProxyCIDRs` | None | Trusted proxies allowed to provide `clientIPHeader`. |
| `logBlockedRequests` | `false` | Log rejected requests to the Traefik log. |

## Score Configuration

The following values determine how much suspicion is added for different request characteristics.

| Option | Default |
|---------|---------|
| `emptyUserAgentScore` | `40` |
| `missingAcceptScore` | `20` |
| `blockedUserAgentScore` | `80` |
| `badPathScore` | `50` |
| `randomArticleScore` | `15` |
| `notFoundScore` | `40` |
| `fakeBrowserScore` | `40` |

Setting any score to `0` disables that individual signal without disabling the rest of the protection.

---

# How It Works

Bot Filter combines immediate blocking rules with a lightweight heuristic scoring engine.

```text
Incoming Request
        │
        ▼
Required Header Validation
        │
        ▼
Blocked Path Detection
        │
        ▼
Blocked User-Agent Detection
        │
        ▼
Browser Validation
        │
        ▼
Heuristic Scoring
        │
        ▼
Score ≥ Threshold ?
   ┌────┴────┐
   │         │
   ▼         ▼
Reject    Forward
403       Backend
```

Each client accumulates a temporary suspicion score.

When the score reaches the configured threshold, the client is temporarily banned for the configured duration.

The score automatically expires using a sliding time window, allowing legitimate users to recover without manual intervention.

---

# Performance

Bot Filter is intentionally lightweight.

| Item | Value |
|------|------:|
| Dependencies | None |
| External Services | None |
| Redis | No |
| Database | No |
| Background Workers | None |
| Thread Safe | Yes |
| Memory Usage | Bounded |
| Reverse Proxy Support | Yes |

The plugin only uses the Go standard library and stores a bounded amount of per-client state.

---

# Limitations

Bot Filter is **not** intended to replace a full Web Application Firewall (WAF).

It is designed to eliminate:

- Internet scanners
- Opportunistic bots
- Malformed HTTP requests
- Common exploit probes
- Low-quality scraping traffic

It cannot reliably stop:

- Large botnets
- Residential proxy networks
- Human-assisted attacks
- Browser automation that perfectly mimics legitimate traffic

For those scenarios, combine Bot Filter with:

- Cloudflare
- Traefik Rate Limit
- Fail2Ban
- Reverse Proxy Firewalls
- CDN Bot Protection

---

# Examples

This directory contains working examples for running **Traefik Bot Filter**.

## Files

| File | Description |
|------|-------------|
| `docker-compose.yml` | Complete Docker Compose example using Traefik and the local plugin. |
| `traefik.yml` | Static Traefik configuration. |
| `dynamic.yml` | Dynamic configuration containing the Bot Filter middleware. |

---

## Quick Start

Clone this repository:

```bash
git clone https://github.com/hoelee/traefik-botfilter.git
```

Copy the plugin into Traefik's local plugin directory:

```text
plugins-local/
└── src/
    └── github.com/
        └── hoelee/
            └── traefik-botfilter/
```

Start the example:

```bash
docker compose up -d
```

Open the dashboard:

```
http://localhost:8080/dashboard/
```

Example service:

```
http://localhost/
```

---

## Middleware Flow

```
Internet
    │
    ▼
Traefik
    │
    ▼
Bot Filter
    │
    ▼
Backend Service
```

---

## Plugin Configuration

The middleware is configured in **dynamic.yml**.

The plugin is enabled in **traefik.yml**.

---

## Testing

### Normal Browser

```
curl http://localhost/
```

Expected:

```
HTTP/1.1 200 OK
```

---

### Missing User-Agent

```
curl -H "User-Agent:" http://localhost/
```

Expected:

```
HTTP/1.1 403 Forbidden
```

---

### Blocked Path

```
curl http://localhost/.env
```

Expected:

```
HTTP/1.1 403 Forbidden
```

---

### Blocked User-Agent

```
curl -A "sqlmap" http://localhost/
```

Expected:

```
HTTP/1.1 403 Forbidden
```

---

## Notes

These examples use the **local plugin** loader.

After the plugin is published in the official Traefik Plugin Catalog, replace:

```yaml
experimental:
  localPlugins:
    botfilter:
      moduleName: github.com/hoelee/traefik-botfilter
```

with:

```yaml
experimental:
  plugins:
    botfilter:
      moduleName: github.com/hoelee/traefik-botfilter
      version: v1.0.0
```

---

# Best Practices

For production deployments:

- Place Bot Filter before Rate Limit middleware.
- Protect the origin server from direct Internet access.
- Use HTTPS only.
- Enable access logs only when required.
- Keep Traefik updated.
- Use a CDN or WAF for Internet-facing services.

Recommended middleware order:

```yaml
middlewares:
  - botfilter
  - ratelimit
  - headers
  - compress
```

---

# Roadmap

## v1.0

- Request validation
- User-Agent filtering
- Browser validation
- Path filtering
- Temporary IP bans
- Heuristic scoring

## v1.1

- Regular expression matching
- Prometheus metrics
- Configurable response body
- IPv6 optimizations
- Better browser fingerprint validation

## v2.0

- Redis shared cache
- Multi-instance synchronization
- Dashboard statistics
- ASN filtering
- GeoIP filtering
- Optional CAPTCHA integration

---

# Development

Clone the repository:

```bash
git clone https://github.com/hoelee/traefik-botfilter.git
cd traefik-botfilter
```

Run tests:

```bash
go test ./...
```

Run static analysis:

```bash
go vet ./...
```

Format source code:

```bash
go fmt ./...
```

---

# Contributing

Contributions are welcome.

If you discover a bug, have a feature request, or would like to improve the plugin, please open an Issue or Pull Request.

Please include:

- Traefik version
- Go version
- Plugin version
- Example configuration
- Relevant logs

---

# License

This project is licensed under the MIT License.

See the [LICENSE](LICENSE) file for details.

---

# Acknowledgements

- Traefik Labs
- Go Community
- Contributors and users of the project

---

## Star the Project

If this plugin helps protect your services, please consider giving the repository a ⭐ on GitHub.

It helps others discover the project and supports future development.

