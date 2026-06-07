package webui

import "html/template"

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Clash Node Pipeline GUI</title>
  <style>
    :root { color-scheme: light dark; }
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; background: #0f172a; color: #e2e8f0; }
    .wrap { max-width: 980px; margin: 0 auto; padding: 32px 18px 60px; }
    .card { background: #111827; border: 1px solid #334155; border-radius: 18px; padding: 22px; box-shadow: 0 20px 45px rgba(0,0,0,.25); margin-bottom: 18px; }
    h1 { margin: 0 0 8px; font-size: 30px; }
    p { color: #94a3b8; line-height: 1.6; }
    label { display: block; margin: 14px 0 6px; font-weight: 650; }
    input[type="text"], input[type="number"], textarea { width: 100%; box-sizing: border-box; padding: 12px 14px; border-radius: 12px; border: 1px solid #475569; background: #020617; color: #e2e8f0; font-size: 15px; font-family: inherit; }
    textarea { min-height: 120px; resize: vertical; line-height: 1.5; }
    .grid { display: grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap: 14px; }
    .check { display: flex; gap: 10px; align-items: center; margin-top: 16px; }
    button { margin-top: 20px; width: 100%; border: 0; border-radius: 14px; padding: 14px 18px; font-size: 16px; font-weight: 800; color: #04111f; background: linear-gradient(135deg,#67e8f9,#a7f3d0); cursor: pointer; }
    button:hover { filter: brightness(1.08); }
    .hint { font-size: 13px; color: #94a3b8; }
    .ok { border-color: #22c55e; }
    .err { border-color: #ef4444; color: #fecaca; }
    table { width: 100%; border-collapse: collapse; margin-top: 8px; }
    td { padding: 9px 8px; border-bottom: 1px solid #334155; }
    td:first-child { color: #94a3b8; width: 220px; }
    code { background: #020617; border: 1px solid #334155; border-radius: 8px; padding: 2px 6px; }
    @media (max-width: 720px) { .grid { grid-template-columns: 1fr; } }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>Clash Node Pipeline GUI</h1>
    <p>本地浏览器界面。支持同时输入多个订阅，跨源去重合并后只输出一个 Clash/Mihomo 配置文件。</p>

    <div class="card">
      <form method="post">
        <label>订阅 URL（每行一个，可粘贴多个）</label>
        <textarea name="source_urls" placeholder="https://example.com/a.yaml&#10;https://example.com/b.yaml" required>{{.Form.SourceURLs}}</textarea>
        <div class="hint">支持换行、逗号或空格分隔；重复的 URL 会自动忽略。链接里的日期（如 <code>2026/06</code>、<code>20260607</code>）会在每次运行时自动校正到当天；运行成功后会自动记住这些链接，下次打开自动填好。</div>

        <label>拉取代理（订阅拉不到时填，留空=直连）</label>
        <input type="text" name="proxy" value="{{.Form.Proxy}}" placeholder="http://127.0.0.1:7890">
        <div class="hint">有些免费订阅在墙外，直连拉不到。填了代理会<b>优先走代理拉取、失败再回退直连</b>。支持 <code>http://127.0.0.1:7890</code>、<code>socks5://127.0.0.1:7891</code>，也可只填端口 <code>7890</code>。Clash Verge 默认混合端口一般是 7890。</div>

        <div class="grid">
          <div>
            <label>最终输出节点数</label>
            <input type="number" name="max_nodes" value="{{.Form.MaxNodes}}" min="1">
          </div>
          <div>
            <label>测速候选上限</label>
            <input type="number" name="speedtest_candidate_limit" value="{{.Form.SpeedtestCandidateLimit}}" min="0">
            <div class="hint">大订阅建议 3000-8000；0 表示按倍数自动。</div>
          </div>
          <div>
            <label>第一阶段并发</label>
            <input type="number" name="stage1_concurrency" value="{{.Form.Stage1Concurrency}}" min="1">
          </div>
          <div>
            <label>第二阶段并发</label>
            <input type="number" name="stage2_concurrency" value="{{.Form.Stage2Concurrency}}" min="1">
          </div>
        </div>

        <label>测速 URL（mihomo 内核精测用）</label>
        <input type="text" name="test_url" value="{{.Form.TestURL}}" placeholder="http://www.gstatic.com/generate_204">

        <label class="check">
          <input type="checkbox" name="disable_mihomo" {{if .Form.DisableMihomo}}checked{{end}}>
          <span>关闭 mihomo 内核测速（默认开启：TCP 粗筛后用 mihomo 精测全部协议）</span>
        </label>

        <label>mihomo 内核路径（留空自动检测 Clash Verge）</label>
        <input type="text" name="mihomo_core" value="{{.Form.MihomoCore}}" placeholder="C:\Program Files\Clash Verge\verge-mihomo.exe">

        <label class="check">
          <input type="checkbox" name="skip_speedtest" {{if .Form.SkipSpeedtest}}checked{{end}}>
          <span>跳过全部测速，只做拉取/解析/去重</span>
        </label>

        <button type="submit">开始运行</button>
        <p class="hint">真实测速更准但更慢，可能需要几十秒到几分钟，等待期间请不要关闭页面。</p>
      </form>
    </div>

    {{if .Error}}
    <div class="card err">
      <h2>运行失败</h2>
      <p>{{.Error}}</p>
    </div>
    {{end}}

    {{if .Report}}
    <div class="card ok">
      <h2>运行完成</h2>
      <table>
        {{if .Report.FetchedViaProxy}}
        <tr><td>经代理拉取的订阅</td><td>{{.Report.FetchedViaProxy}}</td></tr>
        {{end}}
        <tr><td>原始解析节点</td><td>{{.Report.RawParsedNodes}}</td></tr>
        <tr><td>解析/清洗问题</td><td>{{.Report.ParseIssues}}</td></tr>
        <tr><td>清洗后节点</td><td>{{.Report.NormalizedNodes}}</td></tr>
        <tr><td>去重后节点</td><td>{{.Report.DedupedNodes}}</td></tr>
        <tr><td>测速候选节点</td><td>{{.Report.SpeedtestCandidates}}</td></tr>
        <tr><td>第一阶段 TCP 存活</td><td>{{.Report.Stage1Alive}}</td></tr>
        <tr><td>第二阶段测速完成</td><td>{{.Report.Stage2Tested}}</td></tr>
        {{if .Report.MihomoTested}}
        <tr><td>mihomo 测速完成</td><td>{{.Report.MihomoTested}}</td></tr>
        <tr><td>mihomo 测速存活</td><td>{{.Report.MihomoAlive}}</td></tr>
        {{end}}
        {{if .Report.MihomoSkipped}}
        <tr><td>mihomo 提示</td><td>{{.Report.MihomoSkipped}}</td></tr>
        {{end}}
        <tr><td>最终输出节点</td><td>{{.Report.FinalNodes}}</td></tr>
        <tr><td>耗时</td><td>{{.Report.DurationMS}} ms</td></tr>
        <tr><td>Clash 配置</td><td><code>{{.Report.OutputFile}}</code></td></tr>
        {{if .Report.V2rayNFile}}
        <tr><td>v2rayN 订阅</td><td><code>{{.Report.V2rayNFile}}</code>（链接 {{.Report.V2rayNLinks}}，跳过 {{.Report.V2rayNSkipped}}）</td></tr>
        {{end}}
        <tr><td>坏节点日志</td><td><code>{{.Report.BadNodesFile}}</code></td></tr>
      </table>
      <p>把 <code>{{.Report.OutputFile}}</code> 导入 Clash/Mihomo 即可使用。{{if .Report.V2rayNFile}}<br>v2rayN 用户可导入 <code>{{.Report.V2rayNFile}}</code>（已 Base64 编码的订阅文件）。{{end}}</p>
    </div>

    <div class="card">
      <h2>本地订阅地址（推荐）</h2>
      <p>不想每次手动导文件？把下面的地址填进客户端的「订阅」里，以后在本页面重新「开始运行」，客户端更新订阅就会自动拿到最新节点。</p>
      <label>Clash / Mihomo 订阅地址</label>
      <input type="text" readonly value="http://{{.Host}}/clash.yaml" onclick="this.select()">
      <label>v2rayN 订阅地址</label>
      <input type="text" readonly value="http://{{.Host}}/v2rayn.txt" onclick="this.select()">
      <p class="hint">点输入框会自动全选，复制即可。该地址只在本程序运行期间有效，仅供本机使用。</p>
    </div>
    {{end}}
  </div>
</body>
</html>`))
