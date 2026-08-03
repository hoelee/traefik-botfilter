# Traefik Bot Filter

[![Go](https://img.shields.io/badge/Go-1.20+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/hoelee/traefik-botfilter)](LICENSE)

`traefik-botfilter` is a dependency-free Traefik middleware plugin that rejects common scanner requests and temporarily bans suspicious clients with bounded, in-memory per-IP scoring. It is a lightweight first layer of protection, not a replacement for a WAF.

## Features

- Blocks configured paths, extensions, and User-Agent substrings before the request reaches the backend.
- Optionally requires `User-Agent`, `Accept`, and `Host` headers.
- Detects internally inconsistent browser User-Agents.
- Scores suspicious requests and backend `404` responses, then applies a temporary ban.
- Supports CIDR allowlists and trusted proxy client-IP headers.
- Uses only the Go standard library and keeps its per-client cache bounded.

## Install From The Plugin Catalog

After a release tag has been accepted by the catalog, declare the plugin in Traefik's static configuration:

```yaml
experimental:
  plugins:
    botfilter:
      moduleName: github.com/hoelee/traefik-botfilter
      version: v0.1.1
```

Declare a middleware in dynamic configuration and attach it to a router:

```yaml
http:
  middlewares:
    botfilter:
      plugin:
        botfilter:
          requireUserAgent: true
          requireAccept: true
          blockedPaths:
            - /.env
            - /.git
          blockedUserAgents:
            - sqlmap
            - nikto

  routers:
    app:
      rule: Host(`app.example.com`)
      middlewares:
        - botfilter
      service: app
```

Plugin names have two distinct roles above: `botfilter` is the local Traefik plugin identifier, while `github.com/hoelee/traefik-botfilter` is its module path. Keep the identifier the same under `experimental.plugins` and `http.middlewares.<name>.plugin`.

## Local Development

Traefik local plugins must be placed under a directory matching the module path:

```text
plugins-local/
  src/
    github.com/
      hoelee/
        traefik-botfilter/
          .traefik.yml
          go.mod
          botfilter.go
          ...
```

Use `localPlugins` instead of `plugins` in static configuration:

```yaml
experimental:
  localPlugins:
    botfilter:
      moduleName: github.com/hoelee/traefik-botfilter
```

## Example

[`example/traefik.yml`](example/traefik.yml) enables the local plugin and reads [`example/dynamic.yml`](example/dynamic.yml). The dynamic configuration protects `example.localhost` and forwards accepted requests to a backend listening on `127.0.0.1:8081`.

Start a test backend, for example:

```bash
go run github.com/traefik/whoami@latest --port 8081
```

Start Traefik with the example configuration, making the repository available at `/plugins-local/src/github.com/hoelee/traefik-botfilter` in the Traefik process or container. Then test it:

```bash
# A browser-like request reaches the backend.
curl -i http://example.localhost/ \
  -H "User-Agent: Mozilla/5.0 Chrome/120.0 AppleWebKit/537.36 Safari/537.36" \
  -H "Accept: text/html"

# Scanner paths and blocked user agents receive 403.
curl -i http://example.localhost/.env
curl -i http://example.localhost/ -A sqlmap
```

## Configuration

| Option | Default | Description |
| --- | --- | --- |
| `statusCode` | `403` | HTTP status returned for blocked requests. |
| `requireUserAgent` | `false` | Ban clients with no `User-Agent`. |
| `requireAccept` | `false` | Ban clients with no `Accept` header. |
| `requireHost` | `false` | Ban requests with no `Host`. |
| `browserValidation` | `false` | Score inconsistent browser User-Agents. |
| `temporaryBanMinutes` | `15` | Duration of a temporary ban. |
| `scoreThreshold` | `100` | Score at which a client is banned. |
| `scoreWindowMinutes` | `15` | Time window for score accumulation. |
| `maxTrackedIPs` | `50000` | Maximum cache entries. |
| `maxScoreEventsPerIP` | `16` | Maximum score events retained per client. |
| `whitelistCIDRs` | none | CIDRs that bypass all filtering. |
| `blockedUserAgents` | none | Case-insensitive User-Agent substrings to block. |
| `blockedPaths` | none | Paths and path prefixes to block. |
| `blockedExtensions` | none | File extensions to block. |
| `randomArticlePatterns` | `/content/` | Path prefixes that add a small first-request score. |
| `clientIPHeader` | empty | Client-IP header accepted from trusted proxies. |
| `trustedProxyCIDRs` | none | Proxy CIDRs allowed to supply `clientIPHeader`. |
| `logBlockedRequests` | `false` | Log blocked requests through Traefik's process logger. |

Score options are `emptyUserAgentScore` (40), `missingAcceptScore` (20), `blockedUserAgentScore` (80), `badPathScore` (50), `randomArticleScore` (15), `notFoundScore` (40), and `fakeBrowserScore` (40). Set a score to `0` to disable that signal.

## Proxy Configuration

Only configure `clientIPHeader` when Traefik receives traffic from a proxy you trust. Also set `trustedProxyCIDRs`; the plugin ignores the header for all other remote addresses to prevent clients from choosing their own cache identity.

```yaml
clientIPHeader: X-Forwarded-For
trustedProxyCIDRs:
  - 10.0.0.0/8
```

Ensure the trusted proxy overwrites, rather than appends untrusted values to, that header.

## Publishing To The Catalog

The repository must be public, have the `traefik-plugin` GitHub topic, contain a valid root `.traefik.yml` manifest, and have an annotated or lightweight Git tag for each release. Push a new semantic-version tag after merging the package-name fix, for example:

```bash
git tag v1.0.1
git push origin v1.0.1
```

The package is intentionally named `traefik_botfilter`: Traefik's Yaegi loader derives that Go identifier from the final module path segment, replacing `-` with `_`. This must remain aligned with `github.com/hoelee/traefik-botfilter` for `CreateConfig` and `New` to load from the catalog.

## Development

```bash
go test ./...
go vet ./...
gofmt -w *.go
```

## Limitations

This plugin reduces opportunistic scans and low-effort bot traffic. It does not reliably protect against distributed botnets, residential proxies, sophisticated browser automation, or application vulnerabilities. Pair it with rate limiting, a CDN or WAF, TLS, and application-level security controls for Internet-facing services.

## Contributing

Issues and pull requests are welcome. Include the Traefik version, plugin version, relevant configuration, and logs when reporting a problem.

## License

This project is licensed under the [MIT License](LICENSE).
