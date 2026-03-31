#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")"; pwd)"
# shellcheck source=services.sh
source "${SCRIPT_DIR}/services.sh"

usage() {
  cat <<EOF
Usage: ./scripts/cloud/logs.sh <module> [journalctl args...]

Modules:
$(print_modules)

Examples:
  ./scripts/cloud/logs.sh pipeline
  ./scripts/cloud/logs.sh api -n 100 --no-pager
  ./scripts/cloud/logs.sh etcd --since "10 minutes ago"
EOF
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

module="$1"
shift

if ! is_valid_module "$module"; then
  echo "Unknown module: ${module}" >&2
  usage
  exit 1
fi

service="$(module_to_unit "$module")"

if [[ $# -eq 0 ]]; then
  set -- -f
fi

cmd=(journalctl -u "${service}" "$@")
if [[ "${EUID}" -ne 0 ]]; then
  exec sudo "${cmd[@]}"
fi
exec "${cmd[@]}"
