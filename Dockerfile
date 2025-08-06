FROM alpine:latest


# Add dependencies
RUN apk add --no-cache git openssh bash curl mariadb-client ca-certificates gettext

# Create user git
RUN addgroup -g 1000 git && adduser -D -u 1000 -G git -s /bin/bash git

# Copy builded application
COPY ./gitea /usr/local/bin/forgejo
RUN chmod +x /usr/local/bin/forgejo

ENV SPLUNK_ACCESS_TOKEN=${SPLUNK_ACCESS_TOKEN}

# Add splunk-otel-collector    
RUN mkdir -p /otel
COPY ./otel /otel
RUN chmod +x /otel/splunk-otel-collector/bin/otelcol

COPY ./docker/forgejo/otel-config.yaml /otel/config.yaml

# Add entrypoint and template
COPY ./docker/forgejo/entrypoint.sh /app/entrypoint.sh
COPY ./docker/forgejo/templates /app/templates
RUN chmod +x /app/entrypoint.sh

RUN mkdir -p /data /app/gitea && chown -R git:git /data /app /otel

USER git
WORKDIR /app/gitea
EXPOSE 3000

ENTRYPOINT ["/app/entrypoint.sh"]
