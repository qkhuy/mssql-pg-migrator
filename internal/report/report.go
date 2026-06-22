// Package report renders an assess.Assessment to Markdown or a self-contained,
// visual HTML page so users can see exactly what will migrate and how — every
// schema, table, column, type, view and routine, with data volumes and a
// color-coded status.
package report

import (
	"bytes"
	"fmt"
	html "html/template"
	text "text/template"

	"github.com/qkhuy/mssql-pg-migrator/internal/assess"
)

// comma formats an integer with thousands separators (e.g. 11800000 -> 11,800,000).
func comma(n int64) string {
	s := fmt.Sprintf("%d", n)
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg, s = true, s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// Markdown renders the assessment as a Markdown document.
func Markdown(a *assess.Assessment) (string, error) {
	t, err := text.New("md").Funcs(text.FuncMap{"comma": comma}).Parse(markdownTmpl)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, a); err != nil {
		return "", err
	}
	return b.String(), nil
}

// HTML renders the assessment as a self-contained HTML page (inline CSS).
func HTML(a *assess.Assessment) (string, error) {
	t, err := html.New("html").Funcs(html.FuncMap{"comma": comma}).Parse(htmlTmpl)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, a); err != nil {
		return "", err
	}
	return b.String(), nil
}

const markdownTmpl = `# Báo cáo đánh giá Migration

**Nguồn:** ` + "`{{.SourceEngine}}`" + ` → **Đích:** ` + "`{{.TargetEngine}}`" + `
**Thời điểm:** {{.GeneratedAt.Format "2006-01-02 15:04:05"}}

## Tổng quan

| Hạng mục | Số lượng |
|---|---|
| Bảng | {{.Summary.TotalTables}} |
| Cột | {{.Summary.TotalColumns}} |
| View | {{.Summary.TotalViews}} |
| Routine (procedure/function/trigger) | {{.Summary.TotalRoutines}} |
| Tổng dữ liệu (ước tính) | {{comma .Summary.TotalRows}} rows |
| **Tự động** | **{{.Summary.AutoPercent}}%** ({{.Summary.AutoObjects}} tự động · {{.Summary.ReviewObjects}} cần review · {{.Summary.UnsupportedObjects}} không hỗ trợ) |

## Schema → Schema

| Nguồn | Đích |
|---|---|
{{range .Schemas}}| {{.Source}} | {{.Target}} |
{{end}}
## Bảng (table → table, column → column, type → type)
{{range .Tables}}
### {{.Source}} → {{.Target}}  ·  {{.Status.Label}}  ·  ~{{comma .EstimatedRows}} rows

| Cột nguồn | Cột đích | Kiểu nguồn | Kiểu đích | Trạng thái | Ghi chú |
|---|---|---|---|---|---|
{{range .Columns}}| {{.Source}} | {{.Target}} | ` + "`{{.SourceType}}`" + ` | ` + "`{{.TargetType}}`" + ` | {{.Status.Label}} | {{.Note}} |
{{end}}{{end}}
## View → View

| Nguồn | Đích | Trạng thái | Ghi chú |
|---|---|---|---|
{{range .Views}}| {{.Source}} | {{.Target}} | {{.Status.Label}} | {{.Note}} |
{{end}}
## Routine (procedure / function / trigger)

| Loại | Nguồn | Đích | Trạng thái | Ghi chú |
|---|---|---|---|---|
{{range .Routines}}| {{.Kind}} | {{.Source}} | {{.Target}} | {{.Status.Label}} | {{.Note}} |
{{end}}`

const htmlTmpl = `<!DOCTYPE html>
<html lang="vi">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Báo cáo đánh giá Migration</title>
<style>
  :root { --auto:#1a7f37; --auto-bg:#dafbe1; --review:#9a6700; --review-bg:#fff8c5; --unsup:#cf222e; --unsup-bg:#ffebe9; }
  * { box-sizing: border-box; }
  body { font-family: -apple-system, Segoe UI, Roboto, sans-serif; margin: 0; color: #1f2328; background: #f6f8fa; }
  .wrap { max-width: 1100px; margin: 0 auto; padding: 32px 20px 64px; }
  h1 { font-size: 24px; margin: 0 0 4px; }
  h2 { font-size: 18px; margin: 36px 0 12px; border-bottom: 1px solid #d0d7de; padding-bottom: 6px; }
  h3 { font-size: 15px; margin: 22px 0 8px; }
  .meta { color: #656d76; font-size: 13px; }
  .arrow { color: #656d76; }
  .cards { display: flex; flex-wrap: wrap; gap: 12px; margin: 16px 0; }
  .card { background: #fff; border: 1px solid #d0d7de; border-radius: 8px; padding: 14px 18px; min-width: 120px; }
  .card .n { font-size: 22px; font-weight: 700; }
  .card .l { font-size: 12px; color: #656d76; }
  table { border-collapse: collapse; width: 100%; background: #fff; font-size: 13px; margin-bottom: 8px; }
  th, td { border: 1px solid #d0d7de; padding: 6px 10px; text-align: left; vertical-align: top; }
  th { background: #f6f8fa; font-weight: 600; }
  code { background: #eff1f3; padding: 1px 5px; border-radius: 4px; font-size: 12px; }
  .badge { display: inline-block; padding: 1px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; white-space: nowrap; }
  .badge.auto { color: var(--auto); background: var(--auto-bg); }
  .badge.review { color: var(--review); background: var(--review-bg); }
  .badge.unsupported { color: var(--unsup); background: var(--unsup-bg); }
  .bar { height: 10px; border-radius: 6px; overflow: hidden; display: flex; margin: 10px 0 4px; border: 1px solid #d0d7de; }
  .bar span { display: block; height: 100%; }
  .bar .auto { background: var(--auto); }
  .bar .review { background: #d4a72c; }
  .bar .unsupported { background: var(--unsup); }
</style>
</head>
<body>
<div class="wrap">
  <h1>Báo cáo đánh giá Migration</h1>
  <div class="meta">Nguồn <code>{{.SourceEngine}}</code> <span class="arrow">→</span> Đích <code>{{.TargetEngine}}</code> · {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</div>

  <div class="cards">
    <div class="card"><div class="n">{{.Summary.TotalTables}}</div><div class="l">Bảng</div></div>
    <div class="card"><div class="n">{{.Summary.TotalColumns}}</div><div class="l">Cột</div></div>
    <div class="card"><div class="n">{{.Summary.TotalViews}}</div><div class="l">View</div></div>
    <div class="card"><div class="n">{{.Summary.TotalRoutines}}</div><div class="l">Routine</div></div>
    <div class="card"><div class="n">{{comma .Summary.TotalRows}}</div><div class="l">Rows (ước tính)</div></div>
    <div class="card"><div class="n">{{.Summary.AutoPercent}}%</div><div class="l">Tự động</div></div>
  </div>
  <div class="bar">
    <span class="auto" style="width:{{.Summary.AutoObjects}}0px;flex:{{.Summary.AutoObjects}}"></span>
    <span class="review" style="flex:{{.Summary.ReviewObjects}}"></span>
    <span class="unsupported" style="flex:{{.Summary.UnsupportedObjects}}"></span>
  </div>
  <div class="meta">{{.Summary.AutoObjects}} tự động · {{.Summary.ReviewObjects}} cần review · {{.Summary.UnsupportedObjects}} không hỗ trợ</div>

  <h2>Schema → Schema</h2>
  <table>
    <tr><th>Nguồn</th><th></th><th>Đích</th></tr>
    {{range .Schemas}}<tr><td>{{.Source}}</td><td class="arrow">→</td><td>{{.Target}}</td></tr>
    {{end}}
  </table>

  <h2>Bảng <span class="meta">(table → table · column → column · type → type)</span></h2>
  {{range .Tables}}
  <h3>{{.Source}} <span class="arrow">→</span> {{.Target}} &nbsp; <span class="badge {{.Status.Class}}">{{.Status.Label}}</span> &nbsp; <span class="meta">~{{comma .EstimatedRows}} rows</span></h3>
  <table>
    <tr><th>Cột nguồn</th><th>Cột đích</th><th>Kiểu nguồn</th><th>Kiểu đích</th><th>Trạng thái</th><th>Ghi chú</th></tr>
    {{range .Columns}}<tr>
      <td>{{.Source}}</td><td>{{.Target}}</td>
      <td><code>{{.SourceType}}</code></td><td><code>{{.TargetType}}</code></td>
      <td><span class="badge {{.Status.Class}}">{{.Status.Label}}</span></td>
      <td>{{.Note}}</td>
    </tr>
    {{end}}
  </table>
  {{end}}

  <h2>View → View</h2>
  <table>
    <tr><th>Nguồn</th><th>Đích</th><th>Trạng thái</th><th>Ghi chú</th></tr>
    {{range .Views}}<tr><td>{{.Source}}</td><td>{{.Target}}</td><td><span class="badge {{.Status.Class}}">{{.Status.Label}}</span></td><td>{{.Note}}</td></tr>
    {{end}}
  </table>

  <h2>Routine <span class="meta">(procedure / function / trigger)</span></h2>
  <table>
    <tr><th>Loại</th><th>Nguồn</th><th>Đích</th><th>Trạng thái</th><th>Ghi chú</th></tr>
    {{range .Routines}}<tr><td>{{.Kind}}</td><td>{{.Source}}</td><td>{{.Target}}</td><td><span class="badge {{.Status.Class}}">{{.Status.Label}}</span></td><td>{{.Note}}</td></tr>
    {{end}}
  </table>
</div>
</body>
</html>`
