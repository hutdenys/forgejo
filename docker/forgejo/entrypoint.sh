#!/bin/bash
set -e

APP_DATA_PATH="/data"
APP_INI="${APP_DATA_PATH}/gitea/conf/app.ini"

# Create directories with rigths
mkdir -p "$APP_DATA_PATH/gitea/conf"
chown -R git:git "$APP_DATA_PATH"

# If app.ini doesn't exist — generate from template
if [ ! -f "$APP_INI" ]; then
  echo "Generating app.ini..."
  envsubst < /app/templates/app.ini.tpl > "$APP_INI"
  chown git:git "$APP_INI"
fi

# Check if custom app.ini exists and add metrics section if needed
CUSTOM_APP_INI="/data/custom/conf/app.ini"
if [ -f "$CUSTOM_APP_INI" ]; then
  if ! grep -q "^\[metrics\]" "$CUSTOM_APP_INI"; then
    echo "Adding [metrics] section to custom app.ini..."
    echo "" >> "$CUSTOM_APP_INI"
    echo "[metrics]" >> "$CUSTOM_APP_INI"
    echo "ENABLED = true" >> "$CUSTOM_APP_INI"
  fi
fi

# Launch Forgejo
exec /usr/local/bin/forgejo --work-path "$APP_DATA_PATH" web
