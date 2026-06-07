# Clash Node Pipeline

本工具是一个本地 Go CLI，用于聚合 Clash/Mihomo 订阅源、宽松解析节点、自动修复重名、过滤坏节点、执行 TCP 延迟测试，并生成标准 Clash YAML 文件。

> 范围说明：这里的“清洗/洗白”只表示配置清理与标准化，包括去重、安全重命名、字段校验和坏节点过滤。

## 零、双击即用（最简单，推荐小白）

不想敲任何命令？

**方式一：直接下载（最省事，免装 Go）**

到 [Releases](https://github.com/use632/clash-node-pipeline/releases/latest) 下载 `clash-node-pipeline.exe`，双击即用。

**方式二：自己编译**

1. **生成 exe（只需一次）**：双击 `build.bat`（需要先装好 Go），它会自动编译出 `clash节点工具.exe`。
   - 如果已经有现成的 exe，可跳过这步。

---

下载或编译好后，**双击那个 exe**：会自动弹出一个黑色窗口并打开浏览器界面，在网页里填订阅、点「开始运行」即可。

- 生成的文件都在 exe **旁边的 `output` 文件夹**里（`output/gui.clash.yaml` 可直接导入 Clash/Mihomo）。
- 退出：关闭那个黑色窗口，或在窗口里按 `Ctrl+C`。
- 浏览器没自动打开时，手动访问窗口里显示的地址（默认 `http://127.0.0.1:8787`）。

> 这个 exe 是**单文件**的，拷到别的 Windows 电脑也能直接双击用，不需要对方装 Go。

### 用「本地订阅地址」替代手动导文件（推荐）

GUI 里点过一次「开始运行」后，**同一个程序、同一个端口**就自带订阅服务，运行完成页面上会显示两个地址（直接指向 `.yaml` / `.txt` 文件，和常见订阅链接格式一致）：

- Clash / Mihomo： `http://127.0.0.1:8787/clash.yaml`
- v2rayN： `http://127.0.0.1:8787/v2rayn.txt`

把对应地址**填进客户端的「订阅」里**（Clash Verge → 订阅 → 新建；v2rayN → 订阅 → 添加），以后想换最新节点，只要回到本页面再点一次「开始运行」，客户端更新订阅就会自动拿到新结果，**不用每次手动导文件**。（不带后缀的 `/clash`、`/v2rayn` 仍可用，是别名。）

> 该订阅地址只在本程序运行期间有效，且仅限本机（`127.0.0.1`）使用。关掉黑窗口后地址即失效。如果端口被自动改成了 8788 等，按窗口里实际显示的地址为准。

### 黑窗口一闪而过 / 没打开界面怎么办？

绝大多数情况是**上一次开的程序还在后台运行、占着端口**。现在的版本已经会自动换端口（窗口里会提示「端口 8787 被占用，已自动改用 8788」），所以直接看**窗口里显示的地址**访问即可。如果仍打不开：

- 关掉所有正在运行的 `clash节点工具`（任务管理器里结束，或重启电脑），再双击一次。
- 出错时窗口不会再瞬间消失，会停在「按回车键关闭本窗口...」，把上面的报错发出来即可定位。
- 仍想固定端口时，用命令行 `clash节点工具.exe gui --addr 127.0.0.1:9000`。


## 一、小白快速开始

### 1. 安装 Go

先安装 Go 1.23 或更新版本：

- 官网：https://go.dev/dl/
- Windows 安装后重新打开终端。

检查是否安装成功：

```bash
go version
```

### 2. 进入项目目录

```bash
cd clash-node-pipeline
```

如果你当前在父目录，可以执行：

```bash
cd "C:/Users/feng/Downloads/新建文件夹/clash-node-pipeline"
```

### 3. 下载依赖

```bash
go mod tidy
```

### 4. 先跳过测速跑通示例

```bash
go run ./cmd/clash-node-pipeline run --config configs/sources.yaml --skip-speedtest
```

生成文件：

- `output/clash.yaml`：可导入 Clash/Mihomo 的配置。
- `output/report.json`：运行报告。
- `output/bad_nodes.log`：坏节点/坏数据日志。

### 5. 执行真实 TCP 测速

```bash
go run ./cmd/clash-node-pipeline run --config configs/sources.yaml
```

## 二、GUI 图形界面用法

启动本地 Web GUI：

```bash
go run ./cmd/clash-node-pipeline gui
```

程序会自动打开浏览器。如果没有自动打开，请手动访问：

```text
http://127.0.0.1:8787
```

GUI 页面可以直接填写：

- 订阅 URL（多行，每行一个，自动跨源去重合并为一个文件）
- 拉取代理（订阅拉不到时填，留空=直连）
- 最终输出节点数
- 测速候选上限
- 第一阶段并发
- 第二阶段并发
- 测速 URL
- mihomo 内核路径（留空自动检测，默认开启 mihomo 精测）
- 是否关闭 mihomo（只要 TCP 结果）
- 是否跳过测速

点击“开始运行”后会生成：

```text
output/gui.clash.yaml
output/gui.report.json
output/gui.bad_nodes.log
```

其中 `output/gui.clash.yaml` 可以导入 Clash/Mihomo。

如果默认端口被占用，可以换端口：

```bash
go run ./cmd/clash-node-pipeline gui --addr 127.0.0.1:8899
```

## 三、本地订阅服务（serve）

运行 `serve` 子命令，把已生成的订阅文件通过本地 HTTP 暴露出来，客户端就能用「订阅地址」自动更新，而不用手动导文件：

```bash
go run ./cmd/clash-node-pipeline serve --config configs/sources.yaml
```

启动后会提供：

```text
Clash / Mihomo 订阅: http://127.0.0.1:8899/clash
v2rayN 订阅:        http://127.0.0.1:8899/v2rayn
首页（含说明）:      http://127.0.0.1:8899/
```

把对应地址填进客户端的「订阅」里更新即可。换端口用 `--addr 127.0.0.1:9000`，不自动开浏览器加 `--open=false`。

> 典型用法：先 `run` 生成订阅文件，再 `serve` 提供订阅地址。该服务只监听本机。

## 四、把 v2rayN/Base64 订阅作为输入

`sources` 里的 URL 不仅支持 Clash YAML，也支持 **v2rayN 风格的 txt（整体 Base64 的分享链接列表）** 或纯分享链接文本：

```yaml
sources:
  - name: my-v2rayn-sub
    url: "https://example.com/my.v2rayn.txt"
    enabled: true
  - name: local-v2rayn
    url: "output/v2rayn.txt"
    enabled: true
```

程序会自动识别 Base64、逐行解析 `vmess/vless/trojan/ss/...` 链接，与 Clash 订阅一起跨源去重合并。

## 五、按日期自动调整订阅链接

网络上很多免费订阅把「发布日期」写进了链接里，例如：

```text
https://node.example.com/uploads/2026/06/0-20260607.txt
```

其中 `2026/06` 和 `20260607` 就是日期。本工具会在**每次运行时自动把链接里的日期改成当天**，所以你只要写入一次链接，之后不用手动改日期，它也能继续拉到当天的订阅。

- 支持的日期形态：`YYYY/MM`、`YYYY-MM`、`YYYY/MM/DD`、`YYYY-MM-DD`、`YYYYMMDD`（分隔符 `/ - _ .` 都识别，会原样保留）。
- 只改真正的日期：被更长数字串包住的数字（比如 14 位时间戳里的 8 位）不会被误改。
- 已经是当天的链接不会变化（幂等）。
- 这和显式的 `{date}` 占位符是两套机制：`{date}` 是你主动留的空，本功能则是自动识别**已经带日期**的真实链接。

直接把带日期的链接写进 `configs/sources.yaml` 的 `sources` 即可：

```yaml
sources:
  - name: clashnode
    url: "https://node.example.com/uploads/2026/06/0-20260607.txt"
    enabled: true
```

### GUI 自动保存订阅

在 GUI 里运行成功后，工具会把你填写的订阅链接自动保存到 `output/saved_sources.txt`。**下次打开 GUI 时会自动填好这些链接，并把其中的日期校正到当天**——写一次，以后每天打开直接用。

> 保存到磁盘的链接保持你写入时的原样（便于阅读），日期调整发生在运行/读取时。

## 六、常用命令

检查配置文件是否能正常解析：

```bash
go run ./cmd/clash-node-pipeline check-config --config configs/sources.yaml
```

限制最终输出节点数量，并控制测速前候选裁剪：

```bash
go run ./cmd/clash-node-pipeline run \
  --config configs/sources.yaml \
  --output output/clash.yaml \
  --max-nodes 1000 \
  --speedtest-candidate-limit 5000 \
  --stage1-concurrency 500 \
  --stage2-concurrency 200
```

`--speedtest-candidate-limit 5000` 表示先从去重后的节点中稳定抽取 5000 个候选做 TCP 测速，再从存活节点中按延迟输出前 `--max-nodes` 个。对于两万级订阅，这会明显缩短测速时间。

### 多订阅合并

`run` 命令读取 `configs/sources.yaml` 里的 `sources` 列表，可以填多个订阅，程序会跨源解析、去重、合并，最终只输出一个文件：

```yaml
sources:
  - name: sub-a
    url: "https://example.com/a.yaml"
    enabled: true
  - name: sub-b
    url: "https://example.com/b.yaml"
    enabled: true
```

GUI 里则直接在订阅框内多行粘贴即可。

### 测速流程（默认）

默认流程是：

```text
拉取 → 解析 → 去重 → 候选裁剪 → TCP 两阶段粗筛(删除连不通的) → mihomo 内核精测(全协议) → 按延迟排序 → 截断 max_nodes → 输出
```

也就是说，**默认就会先用 TCP 删掉无效节点，再把存活节点交给 mihomo 内核做真实测速**。如果电脑上没找到 mihomo 内核，会自动退回 TCP 结果，不会报错。

直接运行即可（无需任何额外参数）：

```bash
go run ./cmd/clash-node-pipeline run --config configs/sources.yaml
```

### mihomo 内核测速（默认开启，最准）

只要装了 Clash Verge（自带 mihomo 内核），上面的默认流程就会自动调用它对**所有协议**做真实延迟测速：

- 不指定 `--mihomo-core` 时会自动在 `C:\Program Files\Clash Verge\` 等位置查找 `verge-mihomo.exe`。
- 也可手动指定：`--mihomo-core "C:\Program Files\Clash Verge\verge-mihomo.exe"`。
- 工具会临时拉起一个 mihomo 进程，通过其 RESTful API（`/proxies/{name}/delay`）逐节点测速，结束后自动关闭进程，不影响你正在用的 Clash Verge。
- vless/trojan/vmess/hysteria/tuic 等全协议都能精确测速，这是最准的方式。
- 不想用 mihomo（只要 TCP 结果）时加 `--no-mihomo`；GUI 里勾选“关闭 mihomo 内核测速”。

```bash
# 推荐：TCP 粗筛 + mihomo 精测（默认行为）
go run ./cmd/clash-node-pipeline run \
  --config configs/sources.yaml \
  --speedtest-candidate-limit 2000 \
  --max-nodes 300

# 只要 TCP 结果，不调内核
go run ./cmd/clash-node-pipeline run --config configs/sources.yaml --no-mihomo
```

> 实测：某 2 万节点订阅，TCP 初筛“存活”92 个，经 mihomo 真实测速后只有 4 个真正连通——可见 TCP 能连 ≠ 节点可用，mihomo 测速能筛掉大量假存活节点。

## 七、配置订阅源

编辑 `configs/sources.yaml`，支持 HTTP(S)、`file://` 或本地相对路径。URL 中的 `{date}` 会按 `date_formats` 自动展开。

```yaml
sources:
  - name: daily
    url: "https://example.com/sub/{date}"
    enabled: true
```

本地文件示例：

```yaml
sources:
  - name: local-test
    url: "testdata/mixed-sources.txt"
    enabled: true
```

### 用代理拉取订阅（直连拉不到时）

有些免费订阅在墙外，直连会超时拉不到。这时可以让程序**走本地代理（如 Clash Verge）去拉取**：填了代理后，每个订阅会**优先走代理拉取，失败再自动回退直连**——既能拉到被墙的订阅，正常的也不受影响。

GUI 里直接在「拉取代理」框填写即可；命令行用 `--proxy`；配置文件用 `fetch.proxy`：

```yaml
fetch:
  proxy: "http://127.0.0.1:7890"   # 也可写 socks5://127.0.0.1:7891，或只写端口 7890
```

```bash
# 命令行：经代理拉取（端口按你的客户端实际为准）
go run ./cmd/clash-node-pipeline run --config configs/sources.yaml --proxy http://127.0.0.1:7890
```

> 代理端口看你的客户端：Clash Verge Rev 的混合端口常见是 **7897**，旧版 Clash 常见 **7890**，可在客户端设置里查到。只填端口（如 `7897`）会自动补成 `http://127.0.0.1:7897`。运行报告里会显示「经代理拉取」的订阅数量，确认代理确实生效。

## 八、测试命令

格式化代码：

```bash
gofmt -w cmd internal
```

运行单元测试：

```bash
go test ./...
```

运行极端坏数据测试：

```bash
go run ./cmd/clash-node-pipeline run --config configs/sources.yaml --skip-speedtest
```

当前项目已在 Windows + Go 1.26.4 下通过 `gofmt`、`go test ./...` 和真实订阅运行验证。

## 九、稳定性设计

- 单个订阅源失败不会中断整体流程。
- YAML 损坏时会记录错误，并尝试从文本行中继续解析 URI 节点。
- 重名节点会自动追加序号，避免 Clash 配置冲突。
- 节点去重不只看名字，而是看 `type/server/port/password/uuid/sni` 等指纹字段。
- TCP 测试使用 timeout 和 semaphore 限流，避免 1.5 万节点时堵塞网络。
- 带日期的订阅链接每次运行自动校正到当天，且只改真正的日期、对已是当天的链接幂等。
- 输出目录会自动创建。
