#!/usr/bin/env bash
# Usage: ./deploy/deploy.sh user@your-vps-ip
set -euo pipefail

HOST=${1:?usage: deploy.sh user@host}
REMOTE_DIR=/opt/linkbounty

echo "→ building binary..."
GOOS=linux GOARCH=amd64 go build -trimpath -o linkbounty ./cmd/web

echo "→ syncing files to $HOST..."
ssh "$HOST" "sudo mkdir -p $REMOTE_DIR && sudo chown \$(whoami): $REMOTE_DIR"
rsync -az --delete linkbounty ui/ "$HOST:$REMOTE_DIR/"
rsync -az deploy/linkbounty.service "$HOST:/tmp/linkbounty.service"

echo "→ installing service..."
ssh "$HOST" bash << 'REMOTE'
  sudo mv /tmp/linkbounty.service /etc/systemd/system/linkbounty.service
  id linkbounty &>/dev/null || sudo useradd --system --no-create-home linkbounty
  sudo chown -R linkbounty: /opt/linkbounty
  sudo systemctl daemon-reload
  sudo systemctl enable --now linkbounty
  sudo systemctl restart linkbounty
  sleep 1
  sudo systemctl status linkbounty --no-pager
REMOTE

echo "✓ deployed"
