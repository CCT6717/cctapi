package router

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/songquanpeng/one-api/fallback"
)

func renderDashboardHTML(status []map[string]interface{}) string {
	rows := ""
	for _, s := range status {
		level := interfaceToString(s["alert_level"])
		levelClass := "badge normal"
		if level == "warning" {
			levelClass = "badge warning"
		} else if level == "critical" {
			levelClass = "badge critical"
		}

		silenced := ""
		if v, ok := s["silenced"].(bool); ok && v {
			silenced = `<span class="muted">已静音</span>`
		}
		stateNote := fallbackDeploymentStateNote(s)

		rows += `<tr>
<td><strong>` + escapeHTML(s["deployment_id"]) + `</strong>` + silenced + `</td>
<td>` + escapeHTML(s["real_model"]) + `</td>
<td><span class="` + levelClass + `">` + escapeHTML(level) + `</span></td>
<td>` + escapeHTML(s["usage_percent"]) + `</td>
<td class="value">` + escapeHTML(s["used_tokens"]) + ` / ` + escapeHTML(s["daily_limit"]) + `</td>
<td>` + escapeHTML(s["alert_type"]) + `</td>
<td class="value">` + html.EscapeString(stateNote) + `</td>
<td class="actions"><button class="action-btn" data-fallback-action="cooldown" data-deployment-id="` + escapeHTML(s["deployment_id"]) + `">冷却 5 分钟</button><button class="action-btn secondary" data-fallback-action="recover" data-deployment-id="` + escapeHTML(s["deployment_id"]) + `">恢复</button></td>
</tr>`
	}
	if rows == "" {
		rows = `<tr><td colspan="8" class="empty">暂无 fallback 部署数据</td></tr>`
	}

	return renderPage("Fallback 面板", "部署状态面板", "查看 fallback deployment 的实时状态、用量限制和告警状态。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/metrics"><span>监控指标面板</span><small>Prometheus 指标</small></a>
	<a class="nav-card" href="/fallback/scores"><span>排序分数面板</span><small>智能排序得分</small></a>
	<a class="nav-card" href="/fallback/alerts"><span>告警历史</span><small>限额、冷却和恢复</small></a>
	<a class="nav-card" href="/fallback/logs"><span>回退事件日志</span><small>切换记录和原因</small></a>
	<a class="nav-card" href="/api/fallback/alert/status"><span>原始状态数据</span><small>JSON 接口</small></a>
</nav>
<table>
	<thead><tr><th>部署</th><th>模型</th><th>级别</th><th>用量</th><th>Token</th><th>告警</th><th>状态</th><th>操作</th></tr></thead>
	<tbody>`+rows+`</tbody>
</table>
<script>
async function fallbackDeploymentAction(button, deploymentID, action) {
	var url = "/api/fallback/deployments/" + encodeURIComponent(deploymentID);
	if (action === "cooldown") {
		url += "/cooldown?duration_seconds=300";
	} else if (action === "recover") {
		url += "/recover";
	} else {
		return;
	}
	button.disabled = true;
	try {
		var response = await fetch(url, { method: "POST", credentials: "same-origin" });
		var data = await response.json().catch(function(){ return {}; });
		if (!response.ok || data.success === false || data.error) {
			throw new Error(data.message || (data.error && data.error.message) || "操作失败");
		}
		location.reload();
	} catch (error) {
		alert("操作失败：" + error.message);
		button.disabled = false;
	}
}
document.addEventListener("click", function(event) {
	var button = event.target.closest("[data-fallback-action]");
	if (!button) return;
	fallbackDeploymentAction(button, button.getAttribute("data-deployment-id"), button.getAttribute("data-fallback-action"));
});
setTimeout(function(){ location.reload(); }, 15000);
</script>`)
}

func fallbackDeploymentStateNote(status map[string]interface{}) string {
	alertType := interfaceToString(status["alert_type"])
	switch alertType {
	case "exhausted":
		return "耗尽至 " + formatFallbackTime(status["exhausted_until"])
	case "cooldown":
		return "冷却至 " + formatFallbackTime(status["cooldown_until"])
	default:
		return "可用"
	}
}

func formatFallbackTime(value interface{}) string {
	if value == nil {
		return "-"
	}
	switch v := value.(type) {
	case *time.Time:
		if v == nil || v.IsZero() {
			return "-"
		}
		return v.Local().Format("2006-01-02 15:04:05")
	case time.Time:
		if v.IsZero() {
			return "-"
		}
		return v.Local().Format("2006-01-02 15:04:05")
	default:
		return interfaceToString(v)
	}
}

func renderMetricsHTML(metrics string) string {
	rows := ""
	for _, line := range strings.Split(metrics, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		name := parts[0]
		value := ""
		if len(parts) > 1 {
			value = parts[1]
		}
		rows += `<tr><td><code>` + html.EscapeString(name) + `</code></td><td class="value">` + html.EscapeString(value) + `</td></tr>`
	}
	if rows == "" {
		rows = `<tr><td colspan="2" class="empty">暂无指标数据</td></tr>`
	}

	return renderPage("Fallback 监控指标", "监控指标面板", "以面板形式查看 fallback 的 Prometheus 计数器。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/dashboard"><span>部署状态面板</span><small>状态和用量</small></a>
	<a class="nav-card" href="/fallback/scores"><span>排序分数面板</span><small>智能排序得分</small></a>
	<a class="nav-card" href="/fallback/alerts"><span>告警历史</span><small>限额、冷却和恢复</small></a>
	<a class="nav-card" href="/fallback/logs"><span>回退事件日志</span><small>切换记录和原因</small></a>
	<a class="nav-card" href="/metrics"><span>原始指标数据</span><small>Prometheus 文本</small></a>
</nav>
<table>
	<thead><tr><th>指标</th><th>值</th></tr></thead>
	<tbody>`+rows+`</tbody>
</table>
<details>
	<summary>原始指标内容</summary>
	<pre>`+html.EscapeString(metrics)+`</pre>
</details>
<script>setTimeout(function(){ location.reload(); }, 15000);</script>`)
}

func renderScoresHTML(allScores map[string]map[string]float64) string {
	vmNames := make([]string, 0, len(allScores))
	for vmName := range allScores {
		vmNames = append(vmNames, vmName)
	}
	sort.Strings(vmNames)

	content := ""
	for _, vmName := range vmNames {
		scores := allScores[vmName]
		deployments := make([]string, 0, len(scores))
		for deploymentID := range scores {
			deployments = append(deployments, deploymentID)
		}
		sort.SliceStable(deployments, func(i, j int) bool {
			return scores[deployments[i]] > scores[deployments[j]]
		})

		rows := ""
		for i, deploymentID := range deployments {
			rows += `<tr><td>` + fmt.Sprintf("%d", i+1) + `</td><td><strong>` + html.EscapeString(deploymentID) + `</strong></td><td class="value">` + fmt.Sprintf("%.2f", scores[deploymentID]) + `</td></tr>`
		}
		if rows == "" {
			rows = `<tr><td colspan="3" class="empty">暂无排序分数</td></tr>`
		}

		content += `<section class="panel">
<h2>` + html.EscapeString(vmName) + `</h2>
<table><thead><tr><th>排名</th><th>部署</th><th>分数</th></tr></thead><tbody>` + rows + `</tbody></table>
</section>`
	}
	if content == "" {
		content = `<div class="empty">暂无虚拟模型</div>`
	}

	return renderPage("Fallback 排序分数", "排序分数面板", "查看 fallback 智能排序当前使用的 deployment 得分。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/dashboard"><span>部署状态面板</span><small>状态和用量</small></a>
	<a class="nav-card" href="/fallback/metrics"><span>监控指标面板</span><small>Prometheus 指标</small></a>
	<a class="nav-card" href="/fallback/alerts"><span>告警历史</span><small>限额、冷却和恢复</small></a>
	<a class="nav-card" href="/fallback/logs"><span>回退事件日志</span><small>切换记录和原因</small></a>
	<a class="nav-card" href="/api/fallback/sort/scores"><span>原始分数数据</span><small>JSON 接口</small></a>
</nav>`+content+`
<script>setTimeout(function(){ location.reload(); }, 15000);</script>`)
}

func renderLogsHTML(events []fallback.SwitchEvent) string {
	rows := ""
	for _, event := range events {
		status := "-"
		statusClass := "badge normal"
		if event.StatusCode > 0 {
			status = fmt.Sprintf("%d", event.StatusCode)
			if event.StatusCode >= 500 {
				statusClass = "badge critical"
			} else if event.StatusCode >= 400 {
				statusClass = "badge warning"
			}
		}

		duration := "-"
		if event.DurationMs > 0 {
			duration = fmt.Sprintf("%dms", event.DurationMs)
		}
		requestID := event.RequestID
		if requestID == "" {
			requestID = "-"
		}

		rows += `<tr>
<td class="value">` + html.EscapeString(event.CreatedAt.Local().Format("2006-01-02 15:04:05")) + `</td>
<td><strong>` + html.EscapeString(event.VirtualModel) + `</strong></td>
<td><strong>` + html.EscapeString(event.FromDeployment) + `</strong> → <strong>` + html.EscapeString(event.ToDeployment) + `</strong></td>
<td>` + html.EscapeString(event.Reason) + `</td>
<td><span class="` + statusClass + `">` + html.EscapeString(status) + `</span></td>
<td class="value">` + html.EscapeString(duration) + `</td>
<td><code>` + html.EscapeString(requestID) + `</code></td>
</tr>`
	}
	if rows == "" {
		rows = `<tr><td colspan="7" class="empty">暂无回退切换事件</td></tr>`
	}

	return renderPage("Fallback 回退事件日志", "回退事件日志", "查看 fallback 最近的部署切换、原因和耗时。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/dashboard"><span>部署状态面板</span><small>状态和用量</small></a>
	<a class="nav-card" href="/fallback/metrics"><span>监控指标面板</span><small>Prometheus 指标</small></a>
	<a class="nav-card" href="/fallback/scores"><span>排序分数面板</span><small>智能排序得分</small></a>
	<a class="nav-card" href="/fallback/alerts"><span>告警历史</span><small>限额、冷却和恢复</small></a>
	<a class="nav-card" href="/api/fallback/logs?limit=100"><span>原始事件数据</span><small>JSON 接口</small></a>
</nav>
<table>
	<thead><tr><th>时间</th><th>虚拟模型</th><th>切换</th><th>原因</th><th>状态码</th><th>耗时</th><th>请求 ID</th></tr></thead>
	<tbody>`+rows+`</tbody>
</table>
<script>setTimeout(function(){ location.reload(); }, 15000);</script>`)
}

func renderAlertHistoryHTML(events []fallback.AlertHistoryEvent) string {
	rows := ""
	for _, event := range events {
		levelClass := "badge normal"
		switch event.Level {
		case string(fallback.AlertWarning):
			levelClass = "badge warning"
		case string(fallback.AlertCritical):
			levelClass = "badge critical"
		}

		tokens := "-"
		if event.DailyLimit > 0 {
			tokens = fmt.Sprintf("%d / %d", event.UsedTokens, event.DailyLimit)
		} else if event.UsedTokens > 0 {
			tokens = fmt.Sprintf("%d", event.UsedTokens)
		}

		percentage := "-"
		if event.Percentage > 0 {
			percentage = fmt.Sprintf("%.1f%%", event.Percentage)
		}

		rows += `<tr>
<td class="value">` + html.EscapeString(event.CreatedAt.Local().Format("2006-01-02 15:04:05")) + `</td>
<td><strong>` + html.EscapeString(event.DeploymentID) + `</strong></td>
<td><span class="` + levelClass + `">` + html.EscapeString(event.Level) + `</span></td>
<td><code>` + html.EscapeString(event.Type) + `</code></td>
<td class="value">` + html.EscapeString(tokens) + `</td>
<td class="value">` + html.EscapeString(percentage) + `</td>
<td>` + html.EscapeString(event.Message) + `</td>
</tr>`
	}
	if rows == "" {
		rows = `<tr><td colspan="7" class="empty">暂无告警历史</td></tr>`
	}

	return renderPage("Fallback 告警历史", "告警历史", "查看 fallback deployment 的限额、冷却、耗尽和恢复记录。", `
<nav class="panel-nav">
	<a class="nav-card" href="/fallback/dashboard"><span>部署状态面板</span><small>状态和用量</small></a>
	<a class="nav-card" href="/fallback/metrics"><span>监控指标面板</span><small>Prometheus 指标</small></a>
	<a class="nav-card" href="/fallback/scores"><span>排序分数面板</span><small>智能排序得分</small></a>
	<a class="nav-card" href="/fallback/logs"><span>回退事件日志</span><small>切换记录和原因</small></a>
	<a class="nav-card" href="/api/fallback/alert/history?limit=100"><span>原始告警数据</span><small>JSON 接口</small></a>
</nav>
<table>
	<thead><tr><th>时间</th><th>部署</th><th>级别</th><th>类型</th><th>Token</th><th>用量</th><th>消息</th></tr></thead>
	<tbody>`+rows+`</tbody>
</table>
<script>setTimeout(function(){ location.reload(); }, 15000);</script>`)
}

func renderPage(title, heading, subtitle, body string) string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + html.EscapeString(title) + `</title>
<style>
:root { color-scheme: light; --text:#172033; --muted:#667085; --line:#d9e0ea; --soft:#f6f8fb; --blue:#155eef; --green:#067647; --yellow:#b54708; --red:#b42318; }
* { box-sizing: border-box; }
body { margin:0; background: #fff; color: var(--text); font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
.wrap { max-width: 1180px; margin:0 auto; padding: 28px 24px 48px; }
.top { display: flex; justify-content: space-between; align-items: flex-end; gap: 16px; margin-bottom: 20px; border-bottom: 1px solid var(--line); padding-bottom: 18px; }
h1 { margin: 0; font-size: 28px; line-height: 1.2; letter-spacing: 0; }
h2 { margin: 24px 0 10px; font-size: 18px; letter-spacing: 0; }
p { margin: 6px 0 0; color: var(--muted); }
.panel-nav { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 12px; margin: 0 0 18px; }
.nav-card { display: flex; flex-direction: column; justify-content: center; min-height: 82px; padding: 14px 16px; border: 1px solid var(--line); border-radius: 8px; color: var(--text); text-decoration: none; background: linear-gradient(180deg, #fff 0%, #f8fafc 100%); box-shadow: 0 1px 2px rgba(16, 24, 40, .04); }
.nav-card:hover { border-color: #b9c7da; box-shadow: 0 6px 18px rgba(16, 24, 40, .08); transform: translateY(-1px); }
.nav-card span { font-size: 16px; font-weight: 700; }
.nav-card small { margin-top: 4px; color: var(--muted); font-size: 12px; }
table { width:100%; border-collapse: collapse; border: 1px solid var(--line); border-radius: 8px; overflow: hidden; }
th, td { padding: 11px 12px; text-align: left; border-bottom: 1px solid var(--line); vertical-align: middle; }
th { background: var(--soft); color: #344054; font-weight: 600; font-size: 12px; text-transform: uppercase; }
tr:last-child td { border-bottom: 0; }
code, pre { font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace; }
pre { white-space: pre-wrap; background: #0f172a; color: #e2e8f0; border-radius: 8px; padding: 14px; overflow: auto; }
details { margin-top: 16px; }
summary { cursor: pointer; color: var(--blue); font-weight: 600; }
.badge { display: inline-flex; align-items: center; min-width: 72px; justify-content: center; border-radius: 999px; padding: 3px 9px; font-size: 12px; font-weight: 700; text-transform: capitalize; }
.normal { color: var(--green); background: #ecfdf3; }
.warning { color: var(--yellow); background: #fffaeb; }
.critical { color: var(--red); background: #fef3f2; }
.muted { margin-left: 8px; color: var(--muted); font-size: 12px; }
.value { font-variant-numeric: tabular-nums; font-weight: 700; }
.actions { display: flex; gap: 8px; flex-wrap: wrap; min-width: 160px; }
.action-btn { border: 1px solid #b9c7da; border-radius: 8px; background: #fff; color: var(--blue); cursor: pointer; font-weight: 700; padding: 7px 10px; }
.action-btn.secondary { color: var(--green); }
.action-btn:hover { background: #f8fafc; border-color: #8ea3bf; }
.action-btn:disabled { cursor: wait; opacity: .6; }
.panel { margin-top: 18px; }
.empty { color: var(--muted); text-align: center; padding: 24px; }
@media (max-width: 760px) {
	.wrap { padding: 20px 14px 36px; }
	.top { display: block; }
	.panel-nav { grid-template-columns: 1fr; }
	table { display: block; overflow-x: auto; white-space: nowrap; }
}
</style>
</head>
<body>
<main class="wrap">
	<div class="top"><div><h1>` + html.EscapeString(heading) + `</h1><p>` + html.EscapeString(subtitle) + `</p></div></div>
	` + body + `
</main>
</body>
</html>`
}

func interfaceToString(v interface{}) string {
	if v == nil {
		return "-"
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

func escapeHTML(v interface{}) string {
	return html.EscapeString(interfaceToString(v))
}
