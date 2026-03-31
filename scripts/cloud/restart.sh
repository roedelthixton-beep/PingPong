#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")"; pwd)"
# shellcheck source=services.sh
source "${SCRIPT_DIR}/services.sh"

usage() {
  cat <<EOF
Usage: ./scripts/cloud/restart.sh <module>

Modules:
$(print_modules)

Examples:
  ./scripts/cloud/restart.sh pipeline
  ./scripts/cloud/restart.sh api
  ./scripts/cloud/restart.sh etcd
EOF
}

if [[ $# -ne 1 ]]; then
  usage
  exit 1
fi

module="$1"

if ! is_valid_module "$module"; then
  echo "Unknown module: ${module}" >&2
  usage
  exit 1
fi

service="$(module_to_unit "$module")"

cmd=(systemctl restart "${service}")
if [[ "${EUID}" -ne 0 ]]; then
  sudo "${cmd[@]}"
  sudo systemctl is-active --quiet "${service}"
else
  "${cmd[@]}"
  systemctl is-active --quiet "${service}"
fi

echo "${service} restarted and active."
