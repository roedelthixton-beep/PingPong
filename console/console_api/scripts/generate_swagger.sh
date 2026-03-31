#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")"; pwd)"
MODULE_DIR="$(cd "$SCRIPT_DIR/.."; pwd)"

echo "Regenerating console Swagger docs..."

if command -v mise >/dev/null 2>&1; then
  if mise exec -- swag --version >/dev/null 2>&1; then
    mise exec -- swag init -g main.go -o "$MODULE_DIR/docs" --parseDependency -d "$MODULE_DIR"
    echo "Done."
    exit 0
  fi
fi

if command -v swag >/dev/null 2>&1; then
  swag init -g main.go -o "$MODULE_DIR/docs" --parseDependency -d "$MODULE_DIR"
  echo "Done."
  exit 0
fi

echo "swag not installed, skipping console Swagger generation."
echo "Done."
