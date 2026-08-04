#!/usr/bin/env bash
set -euo pipefail

# Usage: scripts/deploy.sh <ssh-host>
# Pulls latest main, rebuilds the server binary, and restarts the systemd service.
# Build happens on the remote host per CLAUDE.md — this repo does not cross-compile.

HOST="${1:?usage: scripts/deploy.sh <ssh-host>}"

# git pull and the build run as the clinicapp system user, which owns
# /opt/clinicapp — running them as root (the ssh user) fails with git's
# "detected dubious ownership" safe.directory check, since root != the
# repo's owner. Only the systemd restart needs root.
ssh "$HOST" 'su clinicapp -s /bin/bash -c "cd /opt/clinicapp && git pull && cd backend && go build -o bin/clinicapp-server ./cmd/server" && systemctl restart clinicapp'
