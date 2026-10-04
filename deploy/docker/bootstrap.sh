#!/bin/sh
# First boot only. Existing vaults and keys are never overwritten.
set -eu
umask 077
seed=/run/secrets/bootstrap
fail() { echo "$*" >&2; exit 1; }
[ -r "$seed" ] || fail 'Create bootstrap.json from bootstrap.example.json; make it readable by UID 10001.'
for file in vault.db vault-password admin-password; do
    [ ! -e "/data/$file" ] || fail 'Partial/existing initialization found. Restore or repair it; refusing to overwrite secrets.'
done
# Print only generic errors, never user credentials or jq input snippets.
jq -e '
  type == "object" and
  has("cookies") and has("card") and has("api-auth") and has("proxy") and has("stripe-key") and has("catalog") and
  ([.. | strings | select(contains("CHANGE_ME"))] | length == 0) and
  (.cookies.cookies | map(select((.name == "auth_token" or .name == "ct0") and (.value | length > 0))) | length == 2) and
  (."stripe-key" | test("^pk_live_[A-Za-z0-9]+$")) and
  (.card.number | test("^[0-9]{12,19}$")) and
  (.card.cvc | test("^[0-9]{3,4}$")) and
  (.proxy.outbounds | length > 0) and
  (.catalog.plans | length > 0) and
  (all(.catalog.plans[]; .amount > 0))
' "$seed" >/dev/null 2>&1 || fail 'bootstrap.json is incomplete/invalid. Replace CHANGE_ME values and set positive catalog amounts.'
stage=$(mktemp -d /data/.bootstrap.XXXXXX)
# Failed initialization is confined to this new staging directory.
cleanup() { rm -rf "$stage"; }
trap cleanup EXIT HUP INT TERM
head -c 32 /dev/urandom | base64 > "$stage/vault-password"
head -c 32 /dev/urandom | base64 > "$stage/admin-password"
jq 'del(."stripe-key")' "$seed" | xgift init --db "$stage/vault.db" --password-file "$stage/vault-password" >/dev/null
jq -r '."stripe-key"' "$seed" | xgift put --db "$stage/vault.db" --password-file "$stage/vault-password" --name stripe-key >/dev/null
xgift status --db "$stage/vault.db" --password-file "$stage/vault-password" >/dev/null
printf '/data/vault-password\n' > "$stage/password-path"
for file in vault-password admin-password password-path vault.db; do
    mv "$stage/$file" "/data/$file"
done
echo 'Vault initialized. Read the administrator password using: docker compose exec app cat /data/admin-password'
