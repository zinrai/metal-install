#!/bin/sh
# Authorize SSH keys for root on 02-00-00-00-00-00.
set -eu

mkdir -p /root/.ssh
chmod 700 /root/.ssh
cat > /root/.ssh/authorized_keys <<'KEYS'
ssh-ed25519 AAAAAA snapshot@example
KEYS
chmod 600 /root/.ssh/authorized_keys
