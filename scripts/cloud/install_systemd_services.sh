#!/bin/bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.."; pwd)"
SYSTEMD_DIR="/etc/systemd/system"
RUN_USER="${RUN_USER:-${SUDO_USER:-$(id -un)}}"
RUN_GROUP="${RUN_GROUP:-$(id -gn "$RUN_USER")}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run with sudo: sudo ./scripts/cloud/install_systemd_services.sh"
  exit 1
fi

render_unit() {
  local src=$1
  local dst=$2

  sed \
    -e "s#{{PROJECT_ROOT}}#${PROJECT_ROOT}#g" \
    -e "s#{{RUN_USER}}#${RUN_USER}#g" \
    -e "s#{{RUN_GROUP}}#${RUN_GROUP}#g" \
    "${src}" > "${dst}"
}

render_unit \
  "${PROJECT_ROOT}/cloud/systemd/pingpong-etcd.service.tpl" \
  "${SYSTEMD_DIR}/pingpong-etcd.service"

render_unit \
  "${PROJECT_ROOT}/cloud/systemd/pingpong-app@.service.tpl" \
  "${SYSTEMD_DIR}/pingpong-app@.service"

systemctl daemon-reload

echo "Installed systemd units:"
echo "  ${SYSTEMD_DIR}/pingpong-etcd.service"
echo "  ${SYSTEMD_DIR}/pingpong-app@.service"
echo ""
echo "Service user: ${RUN_USER}:${RUN_GROUP}"
echo ""
echo "Next steps:"
echo "  1. Build binaries: bash ${PROJECT_ROOT}/scripts/common/build.sh"
echo "  2. Start all services: sudo ${PROJECT_ROOT}/scripts/cloud/restart_all_services.sh"
