#!/usr/bin/env bash
set -Eeuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
umask 077
dc() { docker compose "$@"; }
die() { echo "$*" >&2; exit 1; }
command -v docker >/dev/null || die 'Install Docker Engine and the Compose plugin first (see DOCKER.md).'
docker compose version >/dev/null
if [[ ! -f .env ]]; then
    cp .env.example .env
    die 'Created .env. Fill XGIFT_DOMAIN and ACME_EMAIL, then rerun this command.'
fi
if grep -Eq '^(XGIFT_DOMAIN=gift.example.com|ACME_EMAIL=you@example.com)' .env; then
    die 'Replace the example domain and email in .env first.'
fi
dc config --quiet

start() {
    # Recreate both together: app joins the network namespace of this Caddy.
    dc up -d --force-recreate --wait --wait-timeout 180 caddy app
}

backup() {
    # Pull before stopping anything. Never copy a live SQLite database.
    dc pull backup
    local destination
    destination="backups/$(date -u +%Y%m%dT%H%M%SZ)-$$"
    mkdir -p "$destination"
    # Keep this outside local scope: Bash unwinds locals before an EXIT trap.
    backup_running="$(dc ps --status running --services | grep -E '^(app|caddy)$' || true)"
    # Restore the prior running set even if tar or the local disk fails.
    resume() {
        if [[ -n "$backup_running" ]]; then
            # Intentional splitting: only fixed service names selected above.
            dc start $backup_running
        fi
    }
    trap resume EXIT
    dc stop app caddy
    dc run --rm --no-deps -T backup -C /snapshot -czf - app caddy-data caddy-config > "$destination/volumes.tar.gz"
    cp .env "$destination/env"
    git rev-parse HEAD > "$destination/revision.txt" 2>/dev/null || printf 'source archive deployment\n' > "$destination/revision.txt"
    # Includes local deployment additions, but excludes credentials and caches.
    tar --exclude=.git --exclude=.env --exclude='./.env.*' --exclude=backups \
        --exclude=bootstrap.json --exclude=node_modules --exclude=sqlite --exclude=.private --exclude=.artifacts \
        -czf "$destination/source.tar.gz" .
    resume
    trap - EXIT
    echo "Backup complete: $destination (contains secrets; keep a secure off-server copy)."
}

case "${1:-help}" in
    init)
        dc pull cli
        [ -f bootstrap.json ] || printf '{}\n' > bootstrap.json
        echo 'Wizard: keep /data/vault-password; generate site config: y; use your https://domain; keep 127.0.0.1:8787.'
        echo 'Payment switch is controlled by .env, not the generated /data/site.env.'
        dc run --rm --no-deps cli setup
        dc run --rm --no-deps -T cli status
        start
        ;;
    up) start ;;
    update)
        # Download before downtime. This does not silently merge upstream code.
        dc pull app
        backup
        start
        ;;
    backup) backup ;;
    stop) dc stop app caddy ;;
    status) dc ps; dc run --rm --no-deps -T cli status ;;
    logs) dc logs --tail=100 -f app caddy ;;
    *) echo 'Usage: bash deploy/docker/manage.sh {init|up|update|backup|stop|status|logs}' ;;
esac
