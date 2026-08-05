#!/bin/sh
set -e

timezone="UTC"
printf 'date.timezone="%s"\n' "$timezone" > /usr/local/etc/php/conf.d/zz-ogame-timezone.ini
sed -i "s|^php_value date.timezone .*|php_value date.timezone '$timezone'|" /var/www/html/game/.htaccess
export TZ="$timezone"

case "${OGAME_AUTO_INSTALL:-}" in
  1|true|TRUE|yes|YES|on|ON)
    php /usr/local/bin/ogame-auto-install.php
    chown -R www-data:www-data /var/www/html/persistent_configs
    ;;
esac

exec docker-php-entrypoint "$@"
