#!/bin/sh
set -eu
umask 077
if [ "${1:-}" = xgift-lite ]; then
    if [ ! -e /data/vault.db ]; then
        xgift-bootstrap
    fi
    for file in vault.db vault-password; do
        if [ ! -s "/data/$file" ]; then
            echo "Missing /data/$file. Complete: bash deploy/docker/manage.sh init" >&2
            exit 1
        fi
    done
fi
exec "$@"
