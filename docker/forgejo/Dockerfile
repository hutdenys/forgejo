FROM alpine:latest

ENV FORGEJO_DOMAIN=localhost \
    FORGEJO_PORT=3000 \
    FORGEJO_SSH_PORT=22

# Add dependencies
RUN apk add --no-cache git openssh bash curl mariadb-client ca-certificates gettext

# Create user git
RUN addgroup -g 1000 git && adduser -D -u 1000 -G git -s /bin/bash git

# Download Forgejo
ENV FORGEJO_VERSION=1.21.11-0
RUN curl -L -o /usr/local/bin/forgejo \
    https://codeberg.org/forgejo/forgejo/releases/download/v${FORGEJO_VERSION}/forgejo-${FORGEJO_VERSION}-linux-amd64 && \
    chmod +x /usr/local/bin/forgejo

# Add entrypoint and template
COPY entrypoint.sh /app/entrypoint.sh
COPY templates /app/templates
RUN chmod +x /app/entrypoint.sh

RUN mkdir -p /data /app/gitea && chown -R git:git /data /app

USER git
WORKDIR /app/gitea
VOLUME ["/data"]
EXPOSE 3000 22

ENTRYPOINT ["/app/entrypoint.sh"]
