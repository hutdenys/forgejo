### Stage 1: Forgejo & OTEL Downloader
FROM alpine:latest AS downloader

# Download and extract Splunk OTEL Collector in single layer
RUN apk add --no-cache curl tar && \
    curl -L https://github.com/signalfx/splunk-otel-collector/releases/download/v0.130.0/splunk-otel-collector_0.130.0_amd64.tar.gz \
    -o /otelcol.tar.gz && \
    mkdir -p /otel && \
    tar -xzf /otelcol.tar.gz -C /otel && \
    chmod +x /otel/splunk-otel-collector/bin/otelcol && \
    rm /otelcol.tar.gz && \
    apk del curl tar

### Stage 2: Final image
FROM alpine:latest

# Build arguments for environment variables (combined for clarity)
ARG SPLUNK_ACCESS_TOKEN \
    DB_HOST \
    DB_PORT=3306 \
    DB_USER \
    DB_PASS \
    DB_NAME=forgejo \
    FORGEJO_DOMAIN \
    FORGEJO_PORT=3000 \
    FORGEJO_LFS_JWT_SECRET \
    FORGEJO_INTERNAL_TOKEN \
    FORGEJO_JWT_SECRET

# Add dependencies (combined in single layer)
RUN apk add --no-cache --virtual .runtime-deps \
    git \
    openssh \
    bash \
    curl \
    mariadb-client \
    ca-certificates \
    gettext

# Create git user and setup directories
RUN addgroup -g 1000 git && \
    adduser -D -u 1000 -G git -s /bin/bash git && \
    mkdir -p /data /app/gitea && \
    chown -R git:git /data /app

# Copy and setup application files
COPY ./gitea /usr/local/bin/forgejo
COPY --from=downloader /otel /otel
COPY ./docker/forgejo/otel-config.yaml /otel/config.yaml
COPY ./docker/forgejo/entrypoint.sh /app/entrypoint.sh
COPY ./docker/forgejo/templates /app/templates

# Set all permissions in one layer
RUN chmod +x /usr/local/bin/forgejo /app/entrypoint.sh /otel/splunk-otel-collector/bin/otelcol && \
    chown -R git:git /otel

# Environment variables for runtime (combined in single layer)
ENV SPLUNK_ACCESS_TOKEN=${SPLUNK_ACCESS_TOKEN} \
    DB_HOST=${DB_HOST} \
    DB_PORT=${DB_PORT} \
    DB_USER=${DB_USER} \
    DB_PASS=${DB_PASS} \
    DB_NAME=${DB_NAME} \
    FORGEJO_DOMAIN=${FORGEJO_DOMAIN} \
    FORGEJO_PORT=${FORGEJO_PORT} \
    FORGEJO_LFS_JWT_SECRET=${FORGEJO_LFS_JWT_SECRET} \
    FORGEJO_INTERNAL_TOKEN=${FORGEJO_INTERNAL_TOKEN} \
    FORGEJO_JWT_SECRET=${FORGEJO_JWT_SECRET}

# Runtime config
USER git
WORKDIR /app/gitea
VOLUME ["/data"]
EXPOSE 3000

ENTRYPOINT ["/app/entrypoint.sh"]