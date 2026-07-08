#!/bin/sh
# Authorize SSH keys for root on 02-00-00-00-00-00.
#
# This is the one post-install hook the examples ship, to show the pattern:
# each installer fetches a per-node script from
#   http://192.0.2.1/configs/02-00-00-00-00-00/post/<name>.sh
# and runs it in the target. List more scripts under setup.post in the OS YAML
# and the installer runs each in order. Keep them small and single-purpose.
set -eu

mkdir -p /root/.ssh
chmod 700 /root/.ssh
cat > /root/.ssh/authorized_keys <<'KEYS'
ssh-ed25519 AAAAAA snapshot@example
KEYS
chmod 600 /root/.ssh/authorized_keys
