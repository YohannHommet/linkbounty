#!/usr/bin/env bash
# Usage: ./deploy/deploy.sh user@your-vps-ip
# Prerequisites on the VPS: Caddy installed, ports 80/443 open.
set -euo pipefail

HOST=${1:?usage: deploy.sh user@host}
REMOTE_DIR=/opt/linkbounty

echo "→ building binary (pure-Go, no CGO required)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o linkbounty ./cmd/web

echo "→ preparing remote directory structure on $HOST..."
ssh "$HOST" "
  sudo mkdir -p $REMOTE_DIR/data $REMOTE_DIR/ui/html
  id linkbounty &>/dev/null || sudo useradd --system --no-create-home linkbounty
  sudo chown -R linkbounty: $REMOTE_DIR
"

echo "→ syncing binary and UI assets to $HOST..."
rsync -az --delete linkbounty "$HOST:$REMOTE_DIR/linkbounty"
rsync -az --delete ui/html/    "$HOST:$REMOTE_DIR/ui/html/"

echo "→ installing systemd service..."
rsync -az deploy/linkbounty.service "$HOST:/tmp/linkbounty.service"
ssh "$HOST" "
  sudo mv /tmp/linkbounty.service /etc/systemd/system/linkbounty.service
  sudo chown -R linkbounty: $REMOTE_DIR
  sudo systemctl daemon-reload
  sudo systemctl enable --now linkbounty
  sudo systemctl restart linkbounty
  sleep 1
  sudo systemctl status linkbounty --no-pager
"

echo "✓ deployed to $HOST"
echo ""
echo "Next: copy deploy/Caddyfile to /etc/caddy/Caddyfile on the VPS (edit the domain first),"
echo "then run: ssh $HOST sudo systemctl reload caddy"
