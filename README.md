# Subpage

A Go + Fiber rewrite of the [Remnawave Subscription Page](https://github.com/remnawave/subscription-page),
built to cut down the memory and CPU footprint of the original NestJS
service. Same behavior — renders the subscription page for browsers,
proxies raw subscription payloads for VPN clients — as a single static
Go binary with the React frontend embedded.

Learn more about [Remnawave](https://remna.st/).

## Configuration

Copy `.env.example` to `.env` and fill in:

| Variable | Required | Description |
| - | - | - |
| `REMNAWAVE_PANEL_URL` | yes | Base URL of your Remnawave panel |
| `REMNAWAVE_API_TOKEN` | yes | API token for the panel |
| `INTERNAL_JWT_SECRET` | yes | Signs the session cookie and encrypts subpage-config uuids; keep it stable |
| `SUBPAGE_CONFIG_UUID` | no | Default subpage config uuid for single-tenant setups |
| `CUSTOM_SUB_PREFIX` | no | Global route prefix, if your panel uses one |
| `ADDONS_CONFIG` | no | Path to the add-ons file (default `addons.yml`; `/etc/subpage/addons.yml` in `compose.yml`) |

### Add-ons

An add-on is a second Remnawave user named after the main one with a fixed
prefix and/or suffix, e.g. `premium_<username>`, typically created by billing
for a paid option with its own squad and traffic limit. When a client
fetches the main subscription, the configs of every existing add-on user
are appended to it, renamed to show the add-on's remaining traffic.
Non-active add-ons can be shown as an unusable placeholder config with an
explanatory name (e.g. "traffic limit reached, top up to continue").

Copy `addons.example.yml` to `addons.yml` next to `compose.yml` and edit it.
Without the file nothing changes. Add-on users are looked up only by the
username the panel returns for the requested subscription; the response
headers (`subscription-userinfo`, etc.) always come from the main user.

## Run

```bash
task web:install   # once, installs frontend deps
task dev:web       # bun --hot dev server for the frontend
task dev:api        # go run . --no-web --debug, API only
```

## Build & deploy

```bash
task build          # bun run build, then embeds web/dist into the Go binary
./bin/subpage        # single binary: serves the SPA + /api/*
```

A ready-to-use image is published at `ghcr.io/lentryd/subpage`; see
`compose.yml` for an example deployment behind Traefik.

## License

Derivative work of [remnawave/subscription-page](https://github.com/remnawave/subscription-page),
licensed under AGPL-3.0 — see [LICENSE](LICENSE) and [NOTICE](NOTICE)
for what's ported from the original.
