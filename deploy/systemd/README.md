# OpenTunnel systemd Relay

This directory contains an example native Linux systemd deployment for the OpenTunnel relay.

## Files

- `opentunnel-relay.service`: example systemd unit.
- `opentunnel-relay.env.example`: environment file template for relay flags.

## Install Example

```bash
id -u opentunnel >/dev/null 2>&1 || sudo useradd --system --home /var/lib/opentunnel --shell /usr/sbin/nologin opentunnel
sudo install -d -m 0755 /etc/opentunnel /opt/opentunnel /usr/local/bin
sudo install -d -m 0750 -o opentunnel -g opentunnel /var/lib/opentunnel
sudo install -m 0755 ./opentunnel /usr/local/bin/opentunnel
sudo install -m 0644 ./opentunnel /opt/opentunnel/opentunnel
sudo install -m 0644 deploy/systemd/opentunnel-relay.env.example /etc/opentunnel/relay.env
sudo install -m 0644 deploy/systemd/opentunnel-relay.service /etc/systemd/system/opentunnel-relay.service
sudo editor /etc/opentunnel/relay.env
sudo systemctl daemon-reload
sudo systemctl enable --now opentunnel-relay.service
```

The service runs as the unprivileged `opentunnel` system user. Edit /etc/opentunnel/relay.env before starting the service. Set `OPENTUNNEL_PUBLIC_URL` to the HTTPS origin users will fetch. The default `/opt/opentunnel/opentunnel` artifact path is the release artifact served by `/cli`, separate from the `/usr/local/bin/opentunnel` service binary.

TLS is normally terminated by a reverse proxy or load balancer in front of the relay. The hardening settings in the example unit are useful defaults, not a complete security boundary.
