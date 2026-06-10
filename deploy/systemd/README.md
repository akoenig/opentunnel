# OpenTunnel systemd Relay

This directory contains an example native Linux systemd deployment for the OpenTunnel relay.

## Files

- `opentunnel-relay.service`: example systemd unit.
- `opentunnel-relay.env.example`: environment file template for relay flags.

## Install Example

```bash
sudo install -d -m 0755 /etc/opentunnel /opt/opentunnel
sudo install -m 0755 ./opentunnel /opt/opentunnel/opentunnel
sudo install -m 0644 deploy/systemd/opentunnel-relay.env.example /etc/opentunnel/relay.env
sudo install -m 0644 deploy/systemd/opentunnel-relay.service /etc/systemd/system/opentunnel-relay.service
sudo systemctl daemon-reload
sudo systemctl enable --now opentunnel-relay.service
```

Edit `/etc/opentunnel/relay.env` before starting the service. Set `OPENTUNNEL_PUBLIC_URL` to the HTTPS origin users will fetch.

TLS is normally terminated by a reverse proxy or load balancer in front of the relay. The hardening settings in the example unit are useful defaults, not a complete security boundary.
