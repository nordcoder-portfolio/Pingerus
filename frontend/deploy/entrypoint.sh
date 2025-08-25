#!/bin/sh
set -e

TEMPLATE="/usr/share/nginx/html/app-config.template.js"
TARGET="/usr/share/nginx/html/app-config.js"

if [ -f "$TEMPLATE" ]; then

  API_BASE="${API_BASE:-}"
  export API_BASE
  envsubst '${API_BASE}' < "$TEMPLATE" > "$TARGET"
else
  echo "window.__APP_CONFIG__ = { API_BASE: \"${API_BASE:-}\" };" > "$TARGET"
fi