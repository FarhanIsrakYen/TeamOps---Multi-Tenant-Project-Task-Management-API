#!/bin/sh
set -eu

envsubst '${API_BASE_URL}' \
  < /etc/nginx/default.conf.template \
  > /tmp/nginx/default.conf

nginx -t
exec "$@"
