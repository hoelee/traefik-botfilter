.PHONY: build up down logs status health authorize sync sync-all shell clean

# Build the Docker image
build:
	docker compose build

# Start the sync daemon
up:
	docker compose up -d
	@echo "Container started. Use 'make logs' to follow logs."

# Stop the container
down:
	docker compose down

# Follow logs
logs:
	docker logs -f email-sync-oauth2

# Show status of all accounts
status:
	@echo "=== Container Status ==="
	@docker inspect --format='Status: {{.State.Status}}' email-sync-oauth2 2>/dev/null || echo "Container not running"
	@echo ""
	@echo "=== Health Status ==="
	@docker inspect --format='Health: {{.State.Health.Status}}' email-sync-oauth2 2>/dev/null || echo "No healthcheck"
	@echo ""
	@echo "=== Last Sync Times ==="
	@for f in logs/*.last_success; do \
		[ -f "$$f" ] && echo "$$(basename $$f .last_success): $$(date -d @$$(cat $$f) 2>/dev/null || echo 'unknown')"; \
	done
	@echo ""
	@echo "=== Last Failures ==="
	@for f in logs/*.last_failure; do \
		[ -f "$$f" ] && echo "$$(basename $$f .last_failure): $$(date -d @$$(cat $$f) 2>/dev/null || echo 'unknown')"; \
	done

# Check health
health:
	@docker inspect --format='{{json .State.Health}}' email-sync-oauth2 | jq

# Authorize account (usage: make authorize ACCOUNT=1)
authorize:
	@if [ -z "$(ACCOUNT)" ]; then \
		echo "Usage: make authorize ACCOUNT=1"; \
		exit 1; \
	fi
	docker compose run --rm imapsync authorize $(ACCOUNT)

# Force sync account (usage: make sync ACCOUNT=1)
sync:
	@if [ -z "$(ACCOUNT)" ]; then \
		echo "Usage: make sync ACCOUNT=1"; \
		exit 1; \
	fi
	docker compose exec imapsync sync.sh sync $(ACCOUNT)

# Force a full batch now (all configured accounts)
sync-all:
	docker compose exec imapsync sync.sh sync-all

# Enter container shell
shell:
	docker compose exec imapsync sync.sh shell

# Clean up
clean:
	docker compose down -v
	rm -rf logs/*.log logs/*.last_* logs/.heartbeat logs/.notification_failed

# One-shot sync (build, run once, exit)
once:
	ONCE=true docker compose up --build --abort-on-container-exit
