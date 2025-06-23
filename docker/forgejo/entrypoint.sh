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

# Launch Forgejo
exec /usr/local/bin/forgejo --work-path "$APP_DATA_PATH" web
