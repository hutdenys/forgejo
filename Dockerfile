### Stage 1: Forgejo & OTEL Downloader
FROM alpine:latest AS downloader

RUN apk add --no-cache curl tar

# ENV for versions
ENV FORGEJO_VERSION=1.21.11-0 \
    OTEL_VERSION=

# Download and extract Splunk OTEL Collector
RUN curl -L https://github.com/signalfx/splunk-otel-collector/releases/download/v0.130.0/splunk-otel-collector_0.130.0_amd64.tar.gz \
    -o /otelcol.tar.gz && \
    mkdir -p /otel && \
    tar -xzf /otelcol.tar.gz -C /otel && \
    chmod +x /otel/splunk-otel-collector/bin/otelcol && \
    rm /otelcol.tar.gz

### Stage 2: Final image
FROM alpine:latest

# Add dependencies
RUN apk add --no-cache git openssh bash curl mariadb-client ca-certificates gettext

# Create git user
RUN addgroup -g 1000 git && adduser -D -u 1000 -G git -s /bin/bash git

# Copy builded application
COPY ./gitea /usr/local/bin/forgejo
RUN chmod +x /usr/local/bin/forgejo

# Copy Splunk OTEL
COPY --from=downloader /otel /otel

# Add entrypoint and templates
COPY ./docker/forgejo/otel-config.yaml /otel/config.yaml
COPY ./docker/forgejo/entrypoint.sh /app/entrypoint.sh
COPY ./docker/forgejo/templates /app/templates
RUN chmod +x /app/entrypoint.sh

# Set permissions
RUN chmod +x /usr/local/bin/forgejo /app/entrypoint.sh && \
    mkdir -p /data /app/gitea && \
    chown -R git:git /data /app /otel

# Splunk env
ENV SPLUNK_ACCESS_TOKEN=${SPLUNK_ACCESS_TOKEN}

# Runtime config
USER git
WORKDIR /app/gitea
VOLUME ["/data"]
EXPOSE 3000

ENTRYPOINT ["/app/entrypoint.sh"]