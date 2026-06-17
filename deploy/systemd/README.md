# OpenTunnel systemd Relay

This directory contains an example native Linux systemd deployment for the OpenTunnel relay.

## Files

- `opentunnel-relay.service`: example systemd unit.
- `opentunnel-relay.env.example`: environment file template for relay flags.

## Install Example

Native systemd deployment needs an artifact directory containing every supported temporary CLI artifact, named like `opentunnel-1.0.0-linux-amd64`, `opentunnel-1.0.0-linux-arm64`, `opentunnel-1.0.0-darwin-amd64`, and `opentunnel-1.0.0-darwin-arm64`. Copy these files from the release artifact build or CI output, or create them with a local cross-build using the same `VERSION` value. `OPENTUNNEL_VERSION` must match the version segment in those filenames.

```bash
id -u opentunnel >/dev/null 2>&1 || sudo useradd --system --home /var/lib/opentunnel --shell /usr/sbin/nologin opentunnel
sudo install -d -m 0755 /etc/opentunnel /opt/opentunnel /usr/local/bin
sudo install -d -m 0750 -o opentunnel -g opentunnel /var/lib/opentunnel /opt/opentunnel/artifacts
sudo install -m 0755 ./opentunnel /usr/local/bin/opentunnel
sudo install -m 0644 ./opentunnel-* /opt/opentunnel/artifacts/
sudo install -m 0644 deploy/systemd/opentunnel-relay.env.example /etc/opentunnel/relay.env
sudo install -m 0644 deploy/systemd/opentunnel-relay.service /etc/systemd/system/opentunnel-relay.service
sudo editor /etc/opentunnel/relay.env
sudo systemctl daemon-reload
sudo systemctl enable --now opentunnel-relay.service
```

The service runs as the unprivileged `opentunnel` system user. Edit `/etc/opentunnel/relay.env` before starting the service. Set `OPENTUNNEL_PUBLIC_URL` to the HTTPS origin users will fetch. `OPENTUNNEL_ARTIFACT_DIR` points to the directory of release artifacts served by `/cli/bin/...`, separate from the `/usr/local/bin/opentunnel` service binary. `OPENTUNNEL_ACTIVITY_LOG_INTERVAL` controls active tunnel logging and accepts a positive Go duration, such as `30s` or `10m`.

TLS is normally terminated by a reverse proxy or load balancer in front of the relay. The hardening settings in the example unit are useful defaults, not a complete security boundary.
