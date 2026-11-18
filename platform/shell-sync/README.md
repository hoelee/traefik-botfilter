# shell-sync

Multi-account IMAP sync driven by `mbsync`, with OAuth2 token refresh for
providers that no longer accept plain passwords. Extracted from
`hoelee/email-sync-oauth2`.

The entrypoint refreshes each account's token, regenerates the `mbsyncrc` from
environment variables, then runs incremental sync on a timer. Credentials come
from the environment only - see `.env.example`.

```
make build && make run
```
