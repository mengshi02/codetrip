package wiki

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"text/template"
)

// Serve starts a built-in HTTP server to display Wiki documentation
func (g *WikiGenerator) Serve(ctx context.Context, repo string, addr string) error {
	// Pre-generate Wiki content
	result, err := g.Generate(ctx, repo)
	if err != nil {
		return fmt.Errorf("generate wiki: %w", err)
	}

	// Render HTML page
	html, err := renderViewerHTML(result)
	if err != nil {
		return fmt.Errorf("render html: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(html)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	fmt.Printf("Wiki viewer serving on http://%s\n", addr)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("http server error: %w", err)
	}

	return nil
}

// viewerHTMLTemplate is the built-in viewer HTML template (embedded CSS, no external dependencies)
const viewerHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Wiki - Project Documentation</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif;display:flex;min-height:100vh;background:#fafafa;color:#333}
.sidebar{width:260px;background:#fff;border-right:1px solid #e0e0e0;padding:20px 12px;position:fixed;top:0;bottom:0;overflow-y:auto}
.sidebar h2{font-size:18px;font-weight:600;color:#1976d2;margin-bottom:16px;padding:0 8px}
.sidebar ul{list-style:none}
.sidebar li{padding:10px 12px;cursor:pointer;border-radius:6px;font-size:14px;color:#555;transition:all .15s ease;margin-bottom:2px}
.sidebar li:hover{background:#e3f2fd;color:#1565c0}
.sidebar li.active{background:#1976d2;color:#fff;font-weight:500}
.sidebar .badge{display:inline-block;font-size:11px;padding:1px 6px;border-radius:10px;margin-left:6px;opacity:.8}
.sidebar li.active .badge{background:rgba(255,255,255,.3);color:#fff}
.sidebar li:not(.active) .badge{background:#e0e0e0;color:#666}
.main{margin-left:260px;padding:32px 40px;flex:1;max-width:900px}
.section{display:none}
.section.active{display:block}
.section h1{font-size:28px;font-weight:700;color:#1a1a1a;border-bottom:3px solid #1976d2;padding-bottom:12px;margin-bottom:24px}
.section .content{font-size:15px;line-height:1.7;color:#444}
.section .content h2{font-size:22px;margin-top:28px;margin-bottom:12px;color:#1a1a1a}
.section .content h3{font-size:18px;margin-top:20px;margin-bottom:8px;color:#333}
.section .content p{margin-bottom:12px}
.section .content ul,.section .content ol{margin:8px 0 12px 24px}
.section .content li{margin-bottom:4px}
.section .content code{background:#f0f0f0;padding:2px 6px;border-radius:3px;font-size:13px;font-family:"SF Mono",Menlo,Monaco,monospace}
.section .content pre{background:#263238;color:#eee;padding:16px;border-radius:6px;overflow-x:auto;margin:12px 0;font-size:13px;line-height:1.5}
.section .content table{border-collapse:collapse;width:100%;margin:12px 0}
.section .content th,.section .content td{border:1px solid #ddd;padding:8px 12px;text-align:left}
.section .content th{background:#f5f5f5;font-weight:600}
.section .content a{color:#1976d2;text-decoration:none}
.section .content a:hover{text-decoration:underline}
</style>
</head>
<body>
<div class="sidebar">
<h2>Project Wiki</h2>
<ul>
{{range $i, $s := .Sections}}
<li onclick="show({{$i}})" id="nav-{{$i}}">{{$s.Title}}<span class="badge">{{$s.Type}}</span></li>
{{end}}
</ul>
</div>
<div class="main">
{{range $i, $s := .Sections}}
<div class="section{{if eq $i 0}} active{{end}}" id="section-{{$i}}">
<h1>{{$s.Title}}</h1>
<div class="content">{{$s.Content}}</div>
</div>
{{end}}
</div>
<script>
function show(i){
document.querySelectorAll('.section').forEach(function(e){e.classList.remove('active')});
document.querySelectorAll('.sidebar li').forEach(function(e){e.classList.remove('active')});
document.getElementById('section-'+i).classList.add('active');
document.getElementById('nav-'+i).classList.add('active');
}
</script>
</body>
</html>`

// renderViewerHTML renders viewer HTML
func renderViewerHTML(result *WikiResult) ([]byte, error) {
	tmpl, err := template.New("viewer").Parse(viewerHTMLTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if err := tmpl.Execute(buf, result); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	// Copy content since buf will be returned to pool
	data := make([]byte, buf.Len())
	copy(data, buf.Bytes())

	return data, nil
}