#!/bin/bash
set -Eeuo pipefail

# IMAP Sync Manager - multi-account OAuth2 and password authentication.

TOKEN_DIR="${TOKEN_DIR:-/tokens}"
LOG_DIR="${LOG_DIR:-/var/log/imapsync}"
PID_DIR="${PID_DIR:-/tmp/imapsync}"
HEARTBEAT_FILE="${LOG_DIR}/.heartbeat"
NUM_ACCOUNTS="${NUM_ACCOUNTS:-1}"
SYNC_INTERVAL="${SYNC_INTERVAL:-21600}" # six hours
SYNC_CRON="${SYNC_CRON:-}"
ONCE="${ONCE:-false}"

# Notifications are disabled unless NOTIFY_MODE is explicitly set.
# Valid modes: none, batch_success, account_success, failure.
NOTIFY_MODE="${NOTIFY_MODE:-none}"
SMTP_HOST="${SMTP_HOST:-}"
SMTP_PORT="${SMTP_PORT:-587}"
SMTP_USER="${SMTP_USER:-}"
SMTP_PASS="${SMTP_PASS:-}"
SMTP_FROM="${SMTP_FROM:-}"
SMTP_TO="${SMTP_TO:-}"
SMTP_TLS="${SMTP_TLS:-starttls}"
NOTIFICATION_FAILURE_FILE="${LOG_DIR}/.notification_failed"
NOTIFICATION_LOG="${LOG_DIR}/notification.log"

mkdir -p "$LOG_DIR" "$PID_DIR" "$TOKEN_DIR"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

get_var() {
    local varname="$1"
    echo "${!varname:-}"
}

oauth_authorization_invalid() {
    local logfile="$1"
    tail -n 200 "$logfile" 2>/dev/null | grep -Eqi \
        'invalid_grant|expired.*(token|credential)|token.*expired|refresh token.*(expired|revoked|invalid)|authorization.*(expired|revoked)|AADSTS700082|AADSTS700084|AUTHENTICATE failed|authentication.*failed|authentication unsuccessful|invalid credentials'
}

log_oauth_authorization_guidance() {
    local i="$1"
    local provider="$2"
    local logfile="$3"

    if oauth_authorization_invalid "$logfile"; then
        log "Account $i: OAuth2 authorization is expired, revoked, or rejected by $provider. Reauthorize with: docker compose run --rm imapsync authorize $i"
        log "Account $i: After reauthorization, run immediately with: docker compose exec imapsync sync.sh sync $i"
    fi
}

validate_settings() {
    if ! [[ "$NUM_ACCOUNTS" =~ ^[1-9][0-9]*$ ]]; then
        log "Configuration error: NUM_ACCOUNTS must be a positive integer"
        return 1
    fi

    if [ -z "$SYNC_CRON" ] && ! [[ "$SYNC_INTERVAL" =~ ^[1-9][0-9]*$ ]]; then
        log "Configuration error: SYNC_INTERVAL must be a positive number of seconds"
        return 1
    fi

    case "$NOTIFY_MODE" in
        none|batch_success|account_success|failure) ;;
        *)
            log "Configuration error: NOTIFY_MODE must be none, batch_success, account_success, or failure"
            return 1
            ;;
    esac

    if [ "$NOTIFY_MODE" != "none" ]; then
        if [ -z "$SMTP_HOST" ] || [ -z "$SMTP_FROM" ] || [ -z "$SMTP_TO" ]; then
            log "Configuration error: SMTP_HOST, SMTP_FROM, and SMTP_TO are required when notifications are enabled"
            return 1
        fi
        if ! [[ "$SMTP_PORT" =~ ^[1-9][0-9]*$ ]]; then
            log "Configuration error: SMTP_PORT must be a valid port number"
            return 1
        fi
        if { [ -n "$SMTP_USER" ] && [ -z "$SMTP_PASS" ]; } || { [ -z "$SMTP_USER" ] && [ -n "$SMTP_PASS" ]; }; then
            log "Configuration error: SMTP_USER and SMTP_PASS must either both be set or both be empty"
            return 1
        fi
        case "$SMTP_TLS" in starttls|tls|none) ;; *)
            log "Configuration error: SMTP_TLS must be starttls, tls, or none"
            return 1
            ;;
        esac
    fi
}

send_notification() {
    local subject="$1"
    local body="$2"
    local config
    local -a recipients

    if [ "$NOTIFY_MODE" = "none" ]; then
        return 0
    fi

    config="$(mktemp "${PID_DIR}/msmtp.XXXXXX")"
    chmod 600 "$config"
    {
        printf 'defaults\n'
        printf 'host %s\n' "$SMTP_HOST"
        printf 'port %s\n' "$SMTP_PORT"
        printf 'from %s\n' "$SMTP_FROM"
        if [ -n "$SMTP_USER" ]; then
            printf 'auth on\nuser %s\npassword %s\n' "$SMTP_USER" "$SMTP_PASS"
        else
            printf 'auth off\n'
        fi
        case "$SMTP_TLS" in
            starttls) printf 'tls on\ntls_starttls on\n' ;;
            tls)      printf 'tls on\ntls_starttls off\n' ;;
            none)     printf 'tls off\n' ;;
        esac
    } > "$config"

    IFS=',' read -r -a recipients <<< "$SMTP_TO"
    if printf 'From: %s\nTo: %s\nSubject: %s\n\n%s\n' "$SMTP_FROM" "$SMTP_TO" "$subject" "$body" \
        | msmtp --file="$config" "${recipients[@]}" >> "$NOTIFICATION_LOG" 2>&1; then
        rm -f "$config" "$NOTIFICATION_FAILURE_FILE"
        log "Notification sent: $subject"
        return 0
    fi

    rm -f "$config"
    printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$subject" > "$NOTIFICATION_FAILURE_FILE"
    log "Notification delivery failed; container will remain unhealthy until a notification is sent successfully. Check $NOTIFICATION_LOG"
    return 1
}

source_oauth_provider() {
    case "$1" in
        office365|outlook|hotmail) echo "office365" ;;
        gmail|google)              echo "gmail" ;;
        *) return 1 ;;
    esac
}

sync_account() {
    local i="$1"
    local src_host="$(get_var "ACCOUNT_${i}_SOURCE_HOST")"
    local src_user="$(get_var "ACCOUNT_${i}_SOURCE_USER")"
    local src_auth="$(get_var "ACCOUNT_${i}_SOURCE_AUTH")"
    local src_pass="$(get_var "ACCOUNT_${i}_SOURCE_PASS")"
    local src_token="$(get_var "ACCOUNT_${i}_SOURCE_TOKEN")"
    local src_provider="$(get_var "ACCOUNT_${i}_SOURCE_PROVIDER")"
    local dest_host="$(get_var "ACCOUNT_${i}_DEST_HOST")"
    local dest_user="$(get_var "ACCOUNT_${i}_DEST_USER")"
    local dest_pass="$(get_var "ACCOUNT_${i}_DEST_PASS")"
    local delete_opt="$(get_var "ACCOUNT_${i}_DELETE")"
    local dry_run="$(get_var "ACCOUNT_${i}_DRY_RUN")"
    local extra_opts="$(get_var "ACCOUNT_${i}_EXTRA_OPTS")"
    local logfile="${LOG_DIR}/account${i}.log"
    local pidfile="${PID_DIR}/account${i}.pid"
    local oauth_provider
    local oauth_log
    local oauth_authorization_detected=false
    local sync_exit
    local -a args

    SYNC_ACCOUNT_RESULT="skipped"
    if [ -z "$src_host" ] && [ -z "$dest_host" ]; then
        log "Account $i: SKIP (not configured)"
        return 0
    fi
    if [ -z "$src_host" ] || [ -z "$src_user" ] || [ -z "$dest_host" ] || [ -z "$dest_user" ] || [ -z "$dest_pass" ]; then
        log "Account $i: CONFIGURATION FAILED (source host/user and destination host/user/password are required)"
        SYNC_ACCOUNT_RESULT="failure"
        echo "$(date +%s)" > "${LOG_DIR}/account${i}.last_failure"
        return 1
    fi

    if [ -f "$pidfile" ]; then
        local old_pid
        old_pid="$(cat "$pidfile")"
        if kill -0 "$old_pid" 2>/dev/null; then
            log "Account $i: SKIP (already running, PID $old_pid)"
            return 0
        fi
    fi
    echo $$ > "$pidfile"

    log "Account $i: SYNC START - $src_user ($src_host) -> $dest_user ($dest_host)"
    args=(
        --host1 "$src_host" --user1 "$src_user" --ssl1
        --host2 "$dest_host" --user2 "$dest_user" --password2 "$dest_pass" --ssl2
        --noauthmd5
    )

    case "$src_auth" in
        oauth2)
            if ! oauth_provider="$(source_oauth_provider "$src_provider")"; then
                log "Account $i: CONFIGURATION FAILED (OAuth2 provider must be office365, outlook, hotmail, gmail, or google)"
                SYNC_ACCOUNT_RESULT="failure"
                echo "$(date +%s)" > "${LOG_DIR}/account${i}.last_failure"
                rm -f "$pidfile"
                return 1
            fi
            if [ -z "$src_token" ]; then
                log "Account $i: CONFIGURATION FAILED (ACCOUNT_${i}_SOURCE_TOKEN is required for OAuth2)"
                SYNC_ACCOUNT_RESULT="failure"
                echo "$(date +%s)" > "${LOG_DIR}/account${i}.last_failure"
                rm -f "$pidfile"
                return 1
            fi
            if [ ! -s "$src_token" ]; then
                log "Account $i: OAuth2 token is missing. Authorize with: docker compose run --rm imapsync authorize $i"
                SYNC_ACCOUNT_RESULT="failure"
                echo "$(date +%s)" > "${LOG_DIR}/account${i}.last_failure"
                rm -f "$pidfile"
                return 1
            fi
            # The helper prints access and refresh tokens. Keep that output in a
            # short-lived, root-only file instead of the mounted account log.
            oauth_log="$(mktemp "${PID_DIR}/oauth.XXXXXX")"
            chmod 600 "$oauth_log"
            # The upstream helper opens an interactive browser flow when refresh
            # fails. A scheduled container must never wait indefinitely for that.
            timeout --kill-after=5s 90s oauth2_imap --provider "$oauth_provider" --application imapsync --token_file "$src_token" "$src_user" > "$oauth_log" 2>&1 || true
            if oauth_authorization_invalid "$oauth_log"; then
                log_oauth_authorization_guidance "$i" "$src_provider" "$oauth_log"
                oauth_authorization_detected=true
            fi
            rm -f "$oauth_log"
            if [ ! -s "$src_token" ]; then
                log "Account $i: OAuth2 token refresh or authorization failed"
                log_oauth_authorization_guidance "$i" "$src_provider" "$logfile"
                SYNC_ACCOUNT_RESULT="failure"
                echo "$(date +%s)" > "${LOG_DIR}/account${i}.last_failure"
                rm -f "$pidfile"
                return 1
            fi
            args+=(--oauthaccesstoken1 "$src_token")
            ;;
        password)
            if [ -z "$src_pass" ]; then
                log "Account $i: CONFIGURATION FAILED (ACCOUNT_${i}_SOURCE_PASS is required for password authentication)"
                SYNC_ACCOUNT_RESULT="failure"
                echo "$(date +%s)" > "${LOG_DIR}/account${i}.last_failure"
                rm -f "$pidfile"
                return 1
            fi
            args+=(--password1 "$src_pass")
            ;;
        *)
            log "Account $i: CONFIGURATION FAILED (ACCOUNT_${i}_SOURCE_AUTH must be oauth2 or password)"
            SYNC_ACCOUNT_RESULT="failure"
            echo "$(date +%s)" > "${LOG_DIR}/account${i}.last_failure"
            rm -f "$pidfile"
            return 1
            ;;
    esac

    [ "$delete_opt" = "true" ] && args+=(--delete2)
    [ "$dry_run" = "true" ] && args+=(--dry)
    if [ -n "$extra_opts" ]; then
        # Intentional option splitting: this variable supplies extra CLI flags.
        # shellcheck disable=SC2206
        args+=($extra_opts)
    fi

    # Capture the real exit status. Reading $? after an `if` block returns the
    # status of the `if` itself (always 0), which silently reported every
    # failure as exit 0 and let sync_all treat it as success.
    sync_exit=0
    imapsync "${args[@]}" >> "$logfile" 2>&1 || sync_exit=$?

    if [ "$sync_exit" -eq 0 ]; then
        log "Account $i: SYNC OK"
        echo "$(date +%s)" > "${LOG_DIR}/account${i}.last_success"
        SYNC_ACCOUNT_RESULT="success"
        rm -f "$pidfile"
        return 0
    fi

    log "Account $i: SYNC FAILED (exit $sync_exit)"
    echo "$(date +%s)" > "${LOG_DIR}/account${i}.last_failure"
    if [ "$src_auth" = "oauth2" ] && [ "$oauth_authorization_detected" = "false" ]; then
        log_oauth_authorization_guidance "$i" "$src_provider" "$logfile"
    fi
    SYNC_ACCOUNT_RESULT="failure"
    rm -f "$pidfile"
    return "$sync_exit"
}

sync_all() {
    local i
    local configured=0
    local failures=0
    local notification_failures=0
    local successes=""
    local failed_accounts=""

    log "=== Sync cycle start ==="
    for ((i = 1; i <= NUM_ACCOUNTS; i++)); do
        if sync_account "$i"; then
            if [ "$SYNC_ACCOUNT_RESULT" = "success" ]; then
                configured=$((configured + 1))
                successes="${successes}${successes:+, }${i}"
                if [ "$NOTIFY_MODE" = "account_success" ]; then
                    send_notification "IMAP sync succeeded for account $i" "Account $i completed successfully at $(date '+%Y-%m-%d %H:%M:%S')." || notification_failures=1
                fi
            fi
        else
            configured=$((configured + 1))
            failures=$((failures + 1))
            failed_accounts="${failed_accounts}${failed_accounts:+, }${i}"
        fi
    done

    if [ "$NOTIFY_MODE" = "batch_success" ] && [ "$configured" -gt 0 ] && [ "$failures" -eq 0 ]; then
        send_notification "IMAP sync batch succeeded" "All configured accounts completed successfully at $(date '+%Y-%m-%d %H:%M:%S'): $successes." || notification_failures=1
    fi
    if [ "$NOTIFY_MODE" = "failure" ] && [ "$failures" -gt 0 ]; then
        send_notification "IMAP sync batch failed" "Sync failed for account(s) $failed_accounts at $(date '+%Y-%m-%d %H:%M:%S'). Check ${LOG_DIR}/accountN.log for details." || notification_failures=1
    fi

    log "=== Sync cycle complete (success: ${successes:-none}; failed: ${failed_accounts:-none}) ==="
    [ "$failures" -eq 0 ] && [ "$notification_failures" -eq 0 ]
}

start_heartbeat() {
    echo "$(date +%s)" > "$HEARTBEAT_FILE"
    while true; do
        echo "$(date +%s)" > "$HEARTBEAT_FILE"
        sleep 60
    done &
    HEARTBEAT_PID=$!
}

run_cron_scheduler() {
    local wrapper="${PID_DIR}/cron-sync-all.sh"
    local crontab_file="${PID_DIR}/sync.crontab"

    # Cron starts commands with a minimal environment. Preserve this container's
    # configuration in a root-only Bash wrapper so arbitrary ACCOUNT_N_* values work.
    {
        printf '#!/bin/bash\n'
        export -p
        printf 'exec /sync.sh sync-all\n'
    } > "$wrapper"
    chmod 700 "$wrapper"
    printf '%s %s >> /proc/1/fd/1 2>&1\n' "$SYNC_CRON" "$wrapper" > "$crontab_file"

    if ! crontab "$crontab_file"; then
        log "Configuration error: SYNC_CRON is not a valid cron expression: $SYNC_CRON"
        return 1
    fi
    log "Starting cron schedule: $SYNC_CRON"
    cron -f &
    CRON_PID=$!
    wait "$CRON_PID"
}

run_once() {
    log "Running in ONCE mode"
    local status=0
    sync_all || status=$?
    log "One-shot sync complete"
    return "$status"
}

run_daemon() {
    log "Starting daemon mode (accounts: $NUM_ACCOUNTS)"
    start_heartbeat
    trap 'kill "${HEARTBEAT_PID:-}" "${CRON_PID:-}" 2>/dev/null || true' EXIT

    if [ -n "$SYNC_CRON" ]; then
        run_cron_scheduler
        return
    fi

    while true; do
        sync_all || true
        log "Sleeping ${SYNC_INTERVAL}s before the next sync cycle"
        sleep "$SYNC_INTERVAL"
    done
}

case "${1:-}" in
    sync)
        shift
        validate_settings
        sync_account "${1:-1}"
        ;;
    sync-all)
        validate_settings
        sync_all
        ;;
    authorize)
        shift
        i="${1:-1}"
        src_user="$(get_var "ACCOUNT_${i}_SOURCE_USER")"
        src_token="$(get_var "ACCOUNT_${i}_SOURCE_TOKEN")"
        src_provider="$(get_var "ACCOUNT_${i}_SOURCE_PROVIDER")"
        validate_settings
        if ! provider="$(source_oauth_provider "$src_provider")" || [ -z "$src_user" ] || [ -z "$src_token" ]; then
            log "Account $i: authorization requires a supported OAuth2 provider, source user, and token path"
            exit 1
        fi
        log "Manual authorize: $src_user"
        exec oauth2_imap --startover --provider "$provider" --application imapsync --token_file "$src_token" "$src_user"
        ;;
    shell|bash|sh)
        exec /bin/bash
        ;;
    imapsync)
        shift
        exec imapsync "$@"
        ;;
    *)
        validate_settings
        if [ "$ONCE" = "true" ]; then
            run_once
        else
            run_daemon
        fi
        ;;
esac
