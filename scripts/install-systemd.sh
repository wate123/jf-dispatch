#!/bin/sh
set -eu

ROLE=${1:-}
CONFIG=${2:-}
if [ "$(id -u)" -ne 0 ]; then echo "run as root" >&2; exit 1; fi
if [ "$ROLE" != scheduler ] && [ "$ROLE" != worker ]; then echo "usage: $0 scheduler|worker CONFIG" >&2; exit 2; fi
if [ ! -f "$CONFIG" ]; then echo "config not found: $CONFIG" >&2; exit 2; fi
if [ ! -x ./jf-dispatch ]; then echo "build ./jf-dispatch first" >&2; exit 2; fi

if ! id jf-dispatch >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/jf-dispatch --create-home --shell /usr/sbin/nologin jf-dispatch
fi
for group in video render; do
  if getent group "$group" >/dev/null 2>&1; then usermod -a -G "$group" jf-dispatch; fi
done

install -m 0755 ./jf-dispatch /usr/local/bin/jf-dispatch
install -d -o jf-dispatch -g jf-dispatch -m 0750 /etc/jf-dispatch
install -o jf-dispatch -g jf-dispatch -m 0640 "$CONFIG" "/etc/jf-dispatch/$ROLE.yaml"
if [ ! -f /etc/jf-dispatch/cluster-token ]; then
  umask 077
  openssl rand -hex 32 > /etc/jf-dispatch/cluster-token
fi
chown jf-dispatch:jf-dispatch /etc/jf-dispatch/cluster-token
chmod 0600 /etc/jf-dispatch/cluster-token
install -m 0644 configs/systemd/jf-dispatch@.service /etc/systemd/system/jf-dispatch@.service
systemctl daemon-reload
systemctl enable --now "jf-dispatch@$ROLE"
echo "installed jf-dispatch $ROLE"
