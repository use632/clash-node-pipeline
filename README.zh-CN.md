# Clash Node Pipeline

> 聚合、清洗、测速 Clash/Mihomo 订阅，输出标准 Clash 配置和 v2rayN 订阅。

[![Release](https://img.shields.io/github/v/release/use632/clash-node-pipeline)](https://github.com/use632/clash-node-pipeline/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
![Platform](https://img.shields.io/badge/platform-Windows-0078D6)

[English](README.md) · **简体中文**

拉取多个 Clash/Mihomo 或 v2rayN 订阅，解析去重，用 TCP 加 mihomo 内核测速，输出干净的 Clash 配置和 v2rayN 订阅。自带双击即用的图形界面。

## 功能

- **多源合并**：多个订阅进，一个文件出；按指纹去重（类型 / 服务器 / 端口 / 凭据），而不只看名字。
- **宽松解析**：支持 Clash YAML、base64 订阅、分享链接（`ss` / `vmess` / `vless` / `trojan` / ...）、v2rayN txt，以及把节点链接直接写在正文里的网页（如博客文章，会自动从 HTML 里抓取链接）。坏数据跳过，不会中断。
- **两阶段测速**：先 TCP 粗筛，再用 mihomo 内核对全协议做真实延迟测速，剔除「能连但不可用」的节点。
- **多种输出**：Clash/Mihomo YAML、v2rayN 订阅，外加内置的本地订阅服务。
- **日期自适应**：链接里带日期（`.../2026/06/0-20260607.txt`）会在每次运行时自动校正到当天。
- **代理拉取**：被墙的订阅可走本地代理拉取（优先代理，失败回退直连）。
- **一键运行**：双击 exe 打开本地网页界面，无需敲命令。

## 安装

**下载（Windows，免装 Go）**：到 [Releases](https://github.com/use632/clash-node-pipeline/releases/latest) 下载 `clash-node-pipeline.exe`，双击即用。

**从源码编译**：需要 Go 1.23+：

```bash
go build ./cmd/clash-node-pipeline
# 或安装到 GOBIN
go install github.com/use632/clash-node-pipeline/cmd/clash-node-pipeline@latest
```

Windows 也可双击 `build.bat` 一键编译。

## 用法

### 图形界面

双击 exe，或运行：

```bash
clash-node-pipeline gui
```

浏览器会打开 `http://127.0.0.1:8787`。粘贴订阅链接，点「开始运行」。输出在 exe 旁边的 `output/`，结果页会给出本地订阅地址（`/clash.yaml`、`/v2rayn.txt`），可直接填进 Clash Verge 或 v2rayN。

### 命令行

```bash
# 拉取、清洗、测速，写入 output/clash.yaml
clash-node-pipeline run --config configs/sources.yaml

# 走本地代理拉取被墙的订阅
clash-node-pipeline run --proxy http://127.0.0.1:7897

# 只解析去重，不测速
clash-node-pipeline run --skip-speedtest
```

常用参数：`--max-nodes`、`--proxy`、`--no-mihomo`、`--mihomo-core <路径>`、`--test-url`、`--skip-speedtest`、`--v2rayn-out`。

### 本地订阅服务

```bash
clash-node-pipeline serve
```

在 `127.0.0.1:8899` 提供 `/clash` 和 `/v2rayn`，客户端可用订阅地址更新，而不必导入文件。

## 配置

编辑 `configs/sources.yaml`：

```yaml
sources:
  - name: my-sub
    url: "https://example.com/sub.yaml"
    enabled: true
  - name: dated            # 链接里的日期会自动校正到当天
    url: "https://node.example.com/uploads/2026/06/0-20260607.txt"
    enabled: true

fetch:
  proxy: ""                # 例如 http://127.0.0.1:7897，优先于直连

output:
  max_nodes: 1000
```

默认流程：**拉取 → 解析 → 去重 → TCP 粗筛 → mihomo 测速 → 排序 → 输出**。mihomo 测速使用 [Clash Verge](https://github.com/clash-verge-rev/clash-verge-rev) 自带的内核（`verge-mihomo.exe`，自动检测）；找不到内核时自动退回 TCP 结果。

## 开发

```bash
gofmt -l ./cmd ./internal
go vet ./...
go test ./...
```

## 说明

- 图形界面和订阅服务只监听 `127.0.0.1`。
- mihomo 测速需要本机有 mihomo 内核，Clash Verge 自带。
- v2rayN 订阅只包含 `vmess` / `vless` / `trojan` / `ss` 节点 —— v2rayN 不导入纯 `http`/`socks` 代理。Clash 输出则保留全部节点（Clash 支持这些协议）。
- 本工具用于整理和测试你自己的订阅。

## 许可证

[MIT](LICENSE)
