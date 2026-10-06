#!/usr/bin/env bash
set -Eeuo pipefail
image="${1:?image required}"
tmp="$(mktemp -d)"
name="xgift-smoke-$$"
cleanup() {
    docker rm -f "$name" >/dev/null 2>&1 || true
    docker volume rm "$name" >/dev/null 2>&1 || true
    rm -rf "$tmp"
}
trap cleanup EXIT
# Synthetic offline fixture. The app is never allowed to submit payments.
jq '.cookies.cookies[].value="offline-smoke" | ."api-auth".Authorization="Bearer offline-smoke" |
  ."stripe-key"="pk_live_smoketest"' bootstrap.example.json > "$tmp/bootstrap.json"
chmod 755 "$tmp"
chmod 644 "$tmp/bootstrap.json"
docker run -d --name "$name" --network none --read-only --tmpfs /tmp \
    --cap-drop ALL --security-opt no-new-privileges \
    -v "$name:/data" -v "$tmp/bootstrap.json:/run/secrets/bootstrap:ro" \
    -e XGIFT_ORIGIN=https://test.example.com -e XGIFT_DATA_DIR=/data \
    -e XGIFT_PASSWORD_FILE=/data/vault-password \
    -e XGIFT_PAYMENTS_ENABLED=true "$image"
healthy() {
    for attempt in $(seq 1 30); do
        if docker exec "$name" curl -fsS http://127.0.0.1:8787/healthz > "$tmp/health.json"; then
            jq -e '.ok == true and .mode == "lite" and .payments_enabled == false' "$tmp/health.json"
            return
        fi
        sleep 2
    done
    docker logs "$name"
    return 1
}
healthy
for path in admin admin.js app.js api/redeem api/admin/codes api/admin/recovery/start; do
    code="$(docker exec "$name" curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:8787/$path")"
    test "$code" = 404
done
docker exec "$name" curl -fsS http://127.0.0.1:8787/ | grep -q '/lite.js'
docker exec "$name" curl -fsS http://127.0.0.1:8787/api/manual-link/plans | jq -e '.plans | length > 0'
docker exec "$name" sh -c 'test ! -e /data/site.db && test ! -e /data/admin-password && ! command -v xgift && ! command -v xgift-web'
before="$(docker exec "$name" sha256sum /data/vault-password)"
docker restart "$name"
healthy
after="$(docker exec "$name" sha256sum /data/vault-password)"
test "$before" = "$after"
docker exec "$name" sh -c 'test "$(stat -c %a /data/vault-password)" = 600'
echo 'PASS: offline bootstrap, HTTP health, secret permissions, restart persistence'
