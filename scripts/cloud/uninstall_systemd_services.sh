#!/bin/bash
set -euo pipefail

SYSTEMD_DIR="/etc/systemd/system"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run with sudo: sudo ./scripts/cloud/uninstall_systemd_services.sh"
  exit 1
fi

APP_SERVICES=$(systemctl list-units --plain --no-legend 'pingpong-app@*' | awk '{print $1}')

if [[ -n "$APP_SERVICES" ]]; then
  echo "Stopping app services..."
  # shellcheck disable=SC2086
  systemctl disable --now $APP_SERVICES 2>/dev/null || true
fi

if systemctl is-active --quiet pingpong-etcd 2>/dev/null; then
  echo "Stopping pingpong-etcd..."
  systemctl disable --now pingpong-etcd 2>/dev/null || true
fi

echo "Removing unit files..."
rm -f "${SYSTEMD_DIR}/pingpong-etcd.service"
rm -f "${SYSTEMD_DIR}/pingpong-app@.service"

systemctl daemon-reload

echo "Done. All pingpong systemd services have been uninstalled."
