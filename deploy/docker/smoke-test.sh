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
  .card={number:"4242424242424242",exp_month:"12",exp_year:"2099",cvc:"123",billing_name:"Test",email:"test@example.com",billing_country:"US"} |
  ."stripe-key"="pk_live_smoketest"' bootstrap.example.json > "$tmp/bootstrap.json"
chmod 755 "$tmp"
chmod 644 "$tmp/bootstrap.json"
docker run -d --name "$name" --network none --read-only --tmpfs /tmp \
    --cap-drop ALL --security-opt no-new-privileges \
    -v "$name:/data" -v "$tmp/bootstrap.json:/run/secrets/bootstrap:ro" \
    -e XGIFT_ORIGIN=https://test.example.com -e XGIFT_DATA_DIR=/data \
    -e XGIFT_PASSWORD_FILE=/data/vault-password \
    -e XGIFT_ADMIN_PASSWORD_FILE=/data/admin-password \
    -e XGIFT_PAYMENTS_ENABLED=false "$image"
healthy() {
    for attempt in $(seq 1 30); do
        if docker exec "$name" curl -fsS http://127.0.0.1:8787/healthz > "$tmp/health.json"; then
            jq -e '.ok == true and .payments_enabled == false' "$tmp/health.json"
            return
        fi
        sleep 2
    done
    docker logs "$name"
    return 1
}
healthy
if docker exec "$name" xgift-diagnose 00000000000000000000000000000000 > "$tmp/diagnose.txt" 2>&1; then
    echo 'Diagnostics should report that no failure record exists' >&2
    exit 1
fi
grep -q 'no saved failure found for this order ID' "$tmp/diagnose.txt"
before="$(docker exec "$name" sha256sum /data/vault-password /data/admin-password)"
docker restart "$name"
healthy
after="$(docker exec "$name" sha256sum /data/vault-password /data/admin-password)"
test "$before" = "$after"
docker exec "$name" sh -c 'test "$(stat -c %a /data/admin-password)" = 600'
echo 'PASS: offline bootstrap, HTTP health, secret permissions, restart persistence'
