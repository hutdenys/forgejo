#!/bin/bash
set -e

APP_DATA_PATH="/data"
APP_INI="${APP_DATA_PATH}/gitea/conf/app.ini"

# Create directories with rights
mkdir -p "$APP_DATA_PATH/gitea/conf"
chown -R git:git "$APP_DATA_PATH"

# If app.ini doesn't exist — generate from template
if [ ! -f "$APP_INI" ]; then
  echo "Generating app.ini..."
  envsubst < /app/templates/app.ini.tpl > "$APP_INI"
  chown git:git "$APP_INI"
fi

# Launch otel
/otel/splunk-otel-collector/bin/otelcol --config /otel/config.yaml &

# Launch Forgejo
exec /usr/local/bin/forgejo --config "$APP_INI" web
