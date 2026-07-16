# Dockerfile for OGame Open Source
# Some points were borrowed from the deployment by Noli: https://gitlab.com/nolialsea/ogame-opensource-docker

# This deployment only implements a single-domain configuration (the lobby and Universe 1 on the same domain). If you need to separate the universe into a subdomain like uni1.mygame.com, you'll need to come up with your own solution.

FROM php:8.2-apache AS legacy

# MailHog configuration
# Copy the msmtp configuration file into the container
COPY msmtprc /etc/msmtprc
# Set permissions so it's readable by all users
RUN chmod 644 /etc/msmtprc

# Lobby files (start page)
COPY ./wwwroot /var/www/html
COPY ./download /var/www/html/download
# Universe (game) files
COPY ./game /var/www/html/game

# ---- Apache modules (rewrite + remoteip) ----
RUN a2enmod rewrite remoteip

# RemoteIP configuration (trust only your proxy networks)
# Create this file in your repo root: apache-remoteip.conf
COPY apache-remoteip.conf /etc/apache2/conf-available/remoteip.conf
RUN a2enconf remoteip

# (Optional but handy) log the rewritten client IP rather than the proxy IP
# `%a` is the client IP after RemoteIP processing; `%h` is the immediate peer.
# This replaces `%h` -> `%a` in the default log format.
RUN sed -i 's/%h/%a/g' /etc/apache2/apache2.conf

# PHP extensions
COPY php.ini /usr/local/etc/php/conf.d/custom.ini
RUN a2enmod rewrite
RUN apt-get update && apt-get install -y --no-install-recommends \
    msmtp-mta \
    libfreetype6-dev \
    libjpeg62-turbo-dev \
    libpng-dev \
    libwebp-dev \
    libzip-dev \
    zlib1g-dev \
    libonig-dev \
    msmtp \
    && rm -rf /var/lib/apt/lists/*
RUN docker-php-ext-configure gd --with-freetype --with-jpeg --with-webp
RUN docker-php-ext-install gd
RUN docker-php-ext-install mbstring mysqli pdo pdo_mysql

# To prevent configuration files from being destroyed after redeployment, you need to make them symbolic links, and drag the configs themselves into the volume
# Create a directory that will be managed by a Docker volume
RUN mkdir -p /var/www/html/persistent_configs
# Change ownership of this directory so the web server can write to it
RUN chown -R www-data:www-data /var/www/html/persistent_configs
# Create two SEPARATE symbolic links for the two config files
RUN ln -s /var/www/html/persistent_configs/root_config.php /var/www/html/config.php
RUN ln -s /var/www/html/persistent_configs/game_config.php /var/www/html/game/config.php

RUN chown -R www-data:www-data /var/www/html

# RUN apt-get update && apt-get install -y cron
# COPY cronfile /etc/cron.d/cronfile
# RUN chmod 0644 /etc/cron.d/cronfile
# RUN crontab /etc/cron.d/cronfile
# CMD ["cron", "-f"]

# C battle engine
RUN g++ /var/www/html/game/battle/*.cpp -lm -o /usr/lib/cgi-bin/battle
RUN chmod 755 /usr/lib/cgi-bin/battle

# Docker startup helpers
COPY docker/auto-install.php /usr/local/bin/ogame-auto-install.php
COPY docker/entrypoint.sh /usr/local/bin/ogame-entrypoint
RUN chmod 755 /usr/local/bin/ogame-entrypoint

ENTRYPOINT ["ogame-entrypoint"]
CMD ["apache2-foreground"]

FROM golang:1.25 AS backend-builder
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
RUN CGO_ENABLED=0 go build -o /out/ogame-server ./cmd/ogame-server

FROM oven/bun:1.3 AS frontend-builder
WORKDIR /src/frontend
COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile
COPY frontend ./
COPY wwwroot/img ../wwwroot/img
COPY wwwroot/css ../wwwroot/css
COPY wwwroot/evolution ../wwwroot/evolution
COPY wwwroot/favicon.ico ../wwwroot/favicon.ico
COPY game/css ../game/css
COPY game/img ../game/img
COPY game/js ../game/js
COPY game/mods ../game/mods
RUN bun run build

FROM alpine:3.22 AS golang-runtime
RUN adduser -D -H -u 10001 ogame
WORKDIR /srv/ogame
COPY --from=backend-builder /out/ogame-server /usr/local/bin/ogame-server
COPY --from=frontend-builder /src/frontend/dist /srv/ogame/frontend
COPY download /srv/ogame/download
COPY game /srv/ogame/game
RUN mkdir -p /srv/ogame/game/temp /srv/ogame/data && chown -R ogame:ogame /srv/ogame/game/temp /srv/ogame/data
ENV OGAME_ENV=production
ENV OGAME_HTTP_ADDR=:8080
ENV OGAME_STATIC_DIR=/srv/ogame/frontend
ENV OGAME_LEGACY_ASSET_DIR=/srv/ogame/download
ENV OGAME_LEGACY_GAME_DIR=/srv/ogame/game
EXPOSE 8080
USER ogame
ENTRYPOINT ["ogame-server"]
