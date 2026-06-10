# Clash Node Pipeline

> Aggregate, clean, and speed-test Clash/Mihomo subscriptions, then export a Clash config and a v2rayN subscription.

[![Release](https://img.shields.io/github/v/release/use632/clash-node-pipeline)](https://github.com/use632/clash-node-pipeline/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
![Platform](https://img.shields.io/badge/platform-Windows-0078D6)

**English** · [简体中文](README.zh-CN.md)

Pull multiple Clash/Mihomo or v2rayN subscriptions, parse and de-duplicate the nodes, screen them with TCP plus the mihomo core, and write a clean Clash config and a v2rayN subscription. Comes with a double-click GUI.

## Features

- **Multi-source merge** — many subscriptions in, one file out; de-duplicated by fingerprint (type / server / port / credentials), not just name.
- **Tolerant parsing** — Clash YAML, base64 subscriptions, raw share links (`ss` / `vmess` / `vless` / `trojan` / ...), and v2rayN-style txt. Broken entries are skipped, never fatal.
- **Two-stage speed test** — fast TCP screening, then mihomo-core delay testing across all protocols, which drops nodes that connect but don't actually work.
- **Multiple outputs** — Clash/Mihomo YAML, a v2rayN subscription, and a built-in local subscription server.
- **Date-aware URLs** — links with an embedded date (`.../2026/06/0-20260607.txt`) are shifted to today on every run.
- **Proxy fetching** — pull blocked subscriptions through a local proxy (proxy first, direct fallback).
- **One click** — double-click the exe to open a local web GUI. No commands required.

## Install

**Download (Windows, no Go needed)** — get `clash-node-pipeline.exe` from the [latest release](https://github.com/use632/clash-node-pipeline/releases/latest) and double-click it.

**Build from source** — requires Go 1.23+:

```bash
go build ./cmd/clash-node-pipeline
# or install to GOBIN
go install github.com/use632/clash-node-pipeline/cmd/clash-node-pipeline@latest
```

## Usage

### GUI

Double-click the exe, or run:

```bash
clash-node-pipeline gui
```

A browser opens at `http://127.0.0.1:8787`. Paste subscription URLs, click run. Output lands in `output/` next to the exe, and the result page shows local subscription URLs (`/clash.yaml`, `/v2rayn.txt`) you can paste straight into Clash Verge or v2rayN.

### CLI

```bash
# fetch, clean, speed-test, write output/clash.yaml
clash-node-pipeline run --config configs/sources.yaml

# pull blocked subscriptions through a local proxy
clash-node-pipeline run --proxy http://127.0.0.1:7897

# parse and de-duplicate only, no speed test
clash-node-pipeline run --skip-speedtest
```

Useful flags: `--max-nodes`, `--proxy`, `--no-mihomo`, `--mihomo-core <path>`, `--test-url`, `--skip-speedtest`, `--v2rayn-out`.

### Local subscription server

```bash
clash-node-pipeline serve
```

Serves `/clash` and `/v2rayn` on `127.0.0.1:8899`, so clients can subscribe by URL instead of importing a file.

## Configuration

Edit `configs/sources.yaml`:

```yaml
sources:
  - name: my-sub
    url: "https://example.com/sub.yaml"
    enabled: true
  - name: dated            # date in the URL is auto-shifted to today
    url: "https://node.example.com/uploads/2026/06/0-20260607.txt"
    enabled: true

fetch:
  proxy: ""                # e.g. http://127.0.0.1:7897 — tried before direct

output:
  max_nodes: 1000
```

The default pipeline is: **fetch → parse → de-duplicate → TCP screen → mihomo delay test → sort → output**. mihomo testing uses the core bundled with [Clash Verge](https://github.com/clash-verge-rev/clash-verge-rev) (`verge-mihomo.exe`), auto-detected; if no core is found, it falls back to the TCP results.

## Develop

```bash
gofmt -l ./cmd ./internal
go vet ./...
go test ./...
```

## Notes

- The GUI and subscription server bind to `127.0.0.1` only.
- mihomo delay testing needs a mihomo core on the machine; Clash Verge provides one.
- The v2rayN subscription only includes `vmess` / `vless` / `trojan` / `ss` nodes — v2rayN doesn't import plain `http`/`socks` proxies. The Clash output keeps every node (Clash supports those protocols).
- This is a personal tool for organizing and testing your own subscriptions.

## License

[MIT](LICENSE)
