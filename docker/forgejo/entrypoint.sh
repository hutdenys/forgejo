#!/bin/bash
set -e

APP_DATA_PATH="/data"
APP_INI="${APP_DATA_PATH}/gitea/conf/app.ini"

# Create directories with rights
mkdir -p "$APP_DATA_PATH/gitea/conf"
chown -R git:git "$APP_DATA_PATH"

# Always regenerate app.ini from template with current environment variables
echo "Generating app.ini from template..."
envsubst < /app/templates/app.ini.tpl > "$APP_INI"
chown git:git "$APP_INI"

# Launch otel
/otel/splunk-otel-collector/bin/otelcol --config /otel/config.yaml &

# Launch Forgejo
exec /usr/local/bin/forgejo --config "$APP_INI" web
