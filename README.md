# Traefik Bot Filter

`traefik-botfilter` is a dependency-free Traefik middleware plugin for public
sites that are receiving scanners or unsophisticated HTTP floods. It blocks
known scan paths before they reach the upstream and keeps a bounded, local
per-client score cache for temporary bans.

It is deliberately a **request filter**, not a bot-management service. A
distributed attacker that sends valid browser-like requests from many IPs
needs an upstream CDN/WAF or firewall as well.

## What it does

- Immediately rejects and temporarily bans configured scanner paths, file
  extensions, and User-Agent tokens.
- Optionally requires `User-Agent`, `Accept`, and `Host`. A failed required
  check is immediately cached as a temporary ban, so repeated requests do not
  reach Kiwix.
- Adds scores in a configurable sliding window. Defaults implement the stated
  model: empty UA `+40`, missing Accept `+20`, bad path `+50`, blocked UA
  `+80`, first request directly to `/content/` `+15`, and an upstream `404`
  `+40`.
- Bans when the score reaches `scoreThreshold` (default `100`). A blocked scan
  path always bans immediately, independently of the score threshold.
- Applies conservative browser plausibility checks. It detects internally
  inconsistent browser User-Agents, but does not claim to distinguish every
  automation client from a real browser. That requires a JavaScript challenge
  at a CDN/WAF layer.
- Bounds memory with `maxTrackedIPs` (default `50,000`) and
  `maxScoreEventsPerIP` (default `16`), with lock-safe access and no cleanup
  goroutine.
- Never trusts `X-Forwarded-For` unless `clientIPHeader` is explicitly set
  **and** the TCP peer is in `trustedProxyCIDRs`.

## Important fixes before enabling it

Your Kiwix service currently publishes `6001:8080`. Requests to that port go
straight to Kiwix and bypass this middleware, rate limiting, and Traefik
access controls. Remove this line unless you need a separate protected private
listener:

```yaml
kiwix:
  # ports:
  #   - "6001:8080"
```

Also narrow the current error middleware. `400-599` makes every ordinary
Kiwix 404 and upstream 502 call `host.docker.internal:44440`, which adds work
and obscures the original failure. Keep it only for rate-limit responses:

```yaml
cp-ratelimit-errorpages:
  errors:
    status:
      - "429-429"
    service: srv-error
    query: "/{status}.html"
```

The access log supplied with this request contains 57,141 requests with an
empty User-Agent (`"-"`), including thousands of `404` and `502` responses.
With `requireUserAgent: true`, those requests are rejected at Traefik before
they consume Kiwix CPU.

## Configuration

Add the following to the dynamic file provider configuration. The middleware
must be attached to each public Kiwix router.

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

          whitelistCIDRs:
            - 192.168.0.0/16
            - 10.0.0.0/8

          temporaryBanMinutes: 15
          scoreThreshold: 100
          scoreWindowMinutes: 15
          maxTrackedIPs: 50000
          maxScoreEventsPerIP: 16

          blockedUserAgents:
            - curl
            - wget
            - python
            - Go-http-client
            - masscan
            - sqlmap
            - zgrab
            - nikto

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

          # This weak signal contributes only 15 points. Direct links to a
          # Kiwix article remain possible; they are not banned by themselves.
          randomArticlePatterns:
            - /content/

          # Leave both unset when Traefik accepts traffic directly. If a CDN
          # or load balancer is in front, configure its exact source CIDRs and
          # ensure it overwrites this header.
          # clientIPHeader: X-Forwarded-For
          # trustedProxyCIDRs:
          #   - 203.0.113.0/24

          # Do not turn this on during an attack unless short diagnostics are
          # needed: per-request disk logging can itself become expensive.
          logBlockedRequests: false
```

Apply it before `cp-ratelimit` so rejected requests do not consume the
rate-limiter's work. Keep the narrowed errors middleware as the outer wrapper
for the rate limiter:

```yaml
http:
  routers:
    rtr-default-wiki:
      # ... existing rule/service fields ...
      middlewares:
        - cp-ratelimit-errorpages
        - botfilter
        - cp-ratelimit
```

Use the same middleware list on `rtr-wiki` and `rtr-yes` if they expose Kiwix.

## Install in Traefik

### Production: remote plugin

The module name in `.traefik.yml` is intentionally the one requested here.
Create the public repository `github.com/hoelee/traefik-botfilter`, push this
directory, and create the immutable Git tag `v0.1.0`. Then put this in the
**static** Traefik configuration (`traefik.yml`), not `dynamic.yml`:

```yaml
experimental:
  plugins:
    botfilter:
      moduleName: github.com/hoelee/traefik-botfilter
      version: v0.1.0
```

Restart Traefik after changing static plugin configuration. Do not retag an
existing version; publish `v0.1.1` for later changes.

### Test locally first

Traefik local plugins need the Go module at the exact module path below
`/plugins-local/src`. On the Synology host, copy or clone this repository to:

```text
/volume1/docker/traefik-plugins/src/github.com/hoelee/traefik-botfilter
```

Add this readonly mount to the Traefik service:

```yaml
volumes:
  - /volume1/docker/traefik-plugins:/plugins-local:ro
```

And use this static configuration instead of the remote `plugins` block:

```yaml
experimental:
  localPlugins:
    botfilter:
      moduleName: github.com/hoelee/traefik-botfilter
```

Restart Traefik and confirm its startup log says that the `botfilter` plugin
loaded before exposing the public router.

## Option reference

| Option | Default | Meaning |
| --- | ---: | --- |
| `statusCode` | `403` | HTTP status for rejected requests (`400`–`599`). |
| `temporaryBanMinutes` | `15` | In-memory ban duration. |
| `scoreThreshold` | `100` | Score at which a client is banned. |
| `scoreWindowMinutes` | `15` | Sliding window for score events. |
| `maxTrackedIPs` | `50000` | Hard maximum size of the per-IP state map. |
| `maxScoreEventsPerIP` | `16` | Bound on score events retained per client. |
| `requireUserAgent` | `false` | Immediately ban missing User-Agent requests. |
| `requireAccept` | `false` | Immediately ban missing Accept requests. |
| `requireHost` | `false` | Immediately ban missing Host requests. |
| `browserValidation` | `false` | Score implausible Mozilla-family header combinations. |
| `whitelistCIDRs` | none | Clients that bypass all checks and cache updates. |
| `clientIPHeader` | empty | Optional header used only from `trustedProxyCIDRs`. |
| `trustedProxyCIDRs` | none | TCP peer ranges allowed to supply the client-IP header. |
| `randomArticlePatterns` | `/content/` | First-request path prefixes that add `randomArticleScore`. |
| `logBlockedRequests` | `false` | Opt-in rejected-request logging. |

All listed score fields are configurable: `emptyUserAgentScore`,
`missingAcceptScore`, `blockedUserAgentScore`, `badPathScore`,
`randomArticleScore`, `notFoundScore`, and `fakeBrowserScore`. Set a score to
`0` to disable that one signal; scanner-path and required-header rejections
still ban immediately.

## Operational limits and recommended defences

This plugin protects Kiwix from the malformed-header/scanner pattern in the
log, but it cannot stop a botnet that rotates IPs and perfectly imitates
browser headers. For that case:

1. Put the hostname behind a CDN/WAF with bot challenge and request-rate rules.
2. Firewall the NAS so public clients cannot reach Kiwix's port `6001` or any
   Traefik entry point other than the intended public port.
3. Do not expose the NAS origin address in DNS or other services; otherwise
   attackers can bypass the CDN.
4. Keep Traefik access logs sampled or rotate them quickly during an incident.
   Per-request synchronous disk logging becomes material at flood volume.
5. `deploy.resources` is commonly ignored by non-Swarm Docker Compose. Verify
   resource limits with `docker inspect` on the NAS rather than assuming the
   `deploy` block limits CPU.

The cache is intentionally local to a Traefik process and is reset on
container restart. That is appropriate for a low-overhead edge filter; use a
CDN/WAF or shared store if bans must survive restarts or be shared by multiple
Traefik replicas.

## Development

```text
go test ./...
go vet ./...
```

The implementation only uses the Go standard library, which reduces plugin
startup and supply-chain risk.
