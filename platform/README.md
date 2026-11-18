# platform

Small, self-contained pieces that were first written inside product repos and
are reused across the self-hosted stacks. Each directory is standalone: copy the
folder, not the repository.

| module | language | what it does | extracted from |
|---|---|---|---|
| `java-cache/` | Java 21 / Spring Boot | size + TTL bounded Caffeine cache, role-gated mutations | `hoelee/springboot-hoelee-demo` |
| `py-monitor/` | Python 3 / stdlib | URL normalisation, price parsing, retrying HTTP for scrapers | `hoelee/carousell-monitor` |
| `shell-sync/` | POSIX shell / Docker | multi-account IMAP sync with OAuth2 token refresh | `hoelee/email-sync-oauth2` |

The Traefik middleware in the repository root (`botfilter.go` and friends) is the
fourth piece and the reason this repository exists; see the top-level README.

These three modules were copied in from the repositories above when this repository
was opened in August 2026. Their commit dates are preserved from the original work
rather than reset, so the Java module carries 2020-2025 dates inside a newer
repository.
