#!/usr/bin/env bash
set -euo pipefail

# Usage: scripts/deploy.sh <ssh-host>
# Pulls latest main, rebuilds the server binary, and restarts the systemd service.
# Build happens on the remote host per CLAUDE.md — this repo does not cross-compile.

HOST="${1:?usage: scripts/deploy.sh <ssh-host>}"

ssh "$HOST" 'cd /opt/clinicapp && git pull && cd backend && go build -o bin/clinicapp-server ./cmd/server && systemctl restart clinicapp'
