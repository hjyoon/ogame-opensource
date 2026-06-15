#!/bin/sh
set -e

case "${OGAME_AUTO_INSTALL:-}" in
  1|true|TRUE|yes|YES|on|ON)
    php /usr/local/bin/ogame-auto-install.php
    chown -R www-data:www-data /var/www/html/persistent_configs
    ;;
esac

exec docker-php-entrypoint "$@"
