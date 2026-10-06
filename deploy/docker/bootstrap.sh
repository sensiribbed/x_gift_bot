#!/bin/sh
# First boot only. Existing vaults and keys are never overwritten.
set -eu
umask 077
seed=/run/secrets/bootstrap
fail() { echo "$*" >&2; exit 1; }
[ -r "$seed" ] || fail 'Create bootstrap.json from bootstrap.example.json; make it readable by UID 10001.'
for file in vault.db vault-password; do
    [ ! -e "/data/$file" ] || fail 'Partial/existing initialization found. Restore or repair it; refusing to overwrite secrets.'
done
# Print only generic errors, never user credentials or jq input snippets.
jq -e '
  type == "object" and
  (keys | sort) == (["cookies", "api-auth", "proxy", "stripe-key", "catalog"] | sort) and
  ([.. | strings | select(contains("CHANGE_ME"))] | length == 0) and
  (.cookies.cookies | map(select((.name == "auth_token" or .name == "ct0") and (.value | length > 0))) | length == 2) and
  (."stripe-key" | test("^pk_live_[A-Za-z0-9]+$")) and
  (."api-auth".Authorization | startswith("Bearer ")) and
  (.proxy.outbounds | length > 0) and
  (.catalog.plans | length > 0) and
  (all(.catalog.plans[]; .amount > 0))
' "$seed" >/dev/null 2>&1 || fail 'bootstrap.json is incomplete/invalid. Replace CHANGE_ME values and set positive catalog amounts.'
stage=$(mktemp -d /data/.bootstrap.XXXXXX)
# Failed initialization is confined to this new staging directory.
cleanup() { rm -rf "$stage"; }
trap cleanup EXIT HUP INT TERM
head -c 32 /dev/urandom | base64 > "$stage/vault-password"
xgift-config --db "$stage/vault.db" --password-file "$stage/vault-password" init < "$seed" >/dev/null
printf '/data/vault-password\n' > "$stage/password-path"
for file in vault-password password-path vault.db; do
    mv "$stage/$file" "/data/$file"
done
echo 'Lite vault initialized: eligibility and manual links only.'
