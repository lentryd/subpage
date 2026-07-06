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
