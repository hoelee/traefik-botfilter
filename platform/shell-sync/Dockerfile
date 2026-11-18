FROM alpine/git:2.47.2@sha256:062a01ad7a0eb17cff382bc5e26086b4d710e56dfdfdf001109a49b6d9bd378c AS imapsync-source

ARG IMAPSYNC_REV=93654c6025ff7814f983ab74dd300f9bed9282d9
RUN git clone https://github.com/imapsync/imapsync.git /src \
    && git -C /src checkout "$IMAPSYNC_REV" \
    && test "$(git -C /src rev-parse HEAD)" = "$IMAPSYNC_REV"

# Debian's own perl is used on purpose. The official perl:*-slim images ship a
# second interpreter in /usr/local/bin that shadows /usr/bin/perl on PATH but
# does not read Debian's @INC, so every libXXX-perl package installed below
# would be invisible to the interpreter that actually runs imapsync.
FROM debian:bookworm-slim

LABEL maintainer="Hoelee <me@hoelee.com>"
LABEL description="Multi-account IMAP sync with OAuth2 support for Office 365, Hotmail, Outlook.com, and Gmail"
LABEL org.opencontainers.image.source="https://github.com/hoelee/email-sync-oauth2"
LABEL org.opencontainers.image.licenses="MIT"

# Package list follows upstream INSTALL.d/INSTALL.Debian.txt for the pinned
# revision, plus libdigest-hmac-perl (ships Digest::HMAC_SHA1) and the runtime
# tools this image needs (cron, msmtp).
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    perl \
    libauthen-ntlm-perl \
    libcgi-pm-perl \
    libcrypt-openssl-rsa-perl \
    libdata-uniqid-perl \
    libdigest-hmac-perl \
    libemail-address-perl \
    libencode-imaputf7-perl \
    libfile-copy-recursive-perl \
    libfile-tail-perl \
    libhtml-parser-perl \
    libhttp-daemon-perl \
    libhttp-daemon-ssl-perl \
    libhttp-message-perl \
    libio-socket-inet6-perl \
    libio-socket-ssl-perl \
    libio-tee-perl \
    libjson-webtoken-perl \
    libmail-imapclient-perl \
    libmodule-scandeps-perl \
    libnet-dns-perl \
    libnet-server-perl \
    libparse-recdescent-perl \
    libproc-processtable-perl \
    libreadonly-perl \
    libregexp-common-perl \
    libsys-meminfo-perl \
    libterm-readkey-perl \
    libtest-mockobject-perl \
    libunicode-string-perl \
    liburi-perl \
    libwww-perl \
    cron \
    msmtp \
    netcat-openbsd \
    procps \
    && rm -rf /var/lib/apt/lists/*

# Install one immutable, verified upstream revision of imapsync and its current
# OAuth helper. The old helper URLs no longer exist upstream.
COPY --from=imapsync-source /src/imapsync /usr/local/bin/imapsync
COPY --from=imapsync-source /src/oauth2/oauth2_imap/oauth2_imap /usr/local/bin/oauth2_imap
RUN chmod +x /usr/local/bin/imapsync /usr/local/bin/oauth2_imap \
    && perl -c /usr/local/bin/imapsync \
    && perl -c /usr/local/bin/oauth2_imap \
    && imapsync --version

# Copy entrypoint
COPY sync.sh /sync.sh
RUN chmod +x /sync.sh && bash -n /sync.sh

# Health is liveness plus notification delivery. Sync freshness is exposed by
# accountN.last_success / accountN.last_failure files for the chosen schedule.
HEALTHCHECK --interval=5m --timeout=10s --start-period=60s --retries=3 \
    CMD test -f "${LOG_DIR:-/var/log/imapsync}/.heartbeat" && \
        test ! -f "${LOG_DIR:-/var/log/imapsync}/.notification_failed" && \
        test "$(cat "${LOG_DIR:-/var/log/imapsync}/.heartbeat")" -gt "$(date -d '5 minutes ago' +%s)" || \
        { echo "Health check failed: daemon heartbeat is stale or SMTP notification delivery failed. Check container logs."; exit 1; }

ENTRYPOINT ["/sync.sh"]
