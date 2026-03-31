#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")"; pwd)"
# shellcheck source=services.sh
source "${SCRIPT_DIR}/services.sh"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run with sudo: sudo ./scripts/cloud/restart_all_services.sh"
  exit 1
fi

echo "Restarting pingpong services..."

for mod in "${ALL_MODULES[@]}"; do
  svc="$(module_to_unit "$mod")"
  echo "==> restarting ${svc}"
  systemctl restart "${svc}"
  systemctl is-active --quiet "${svc}"
  echo "    ${svc} is active"
done

echo ""
echo "All services restarted successfully."
