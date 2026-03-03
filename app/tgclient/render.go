package tgclient

import (
	"fmt"
	"html"
	"os"
	"strings"
)

func renderMarkdown(path string, records []MessageRecord) error {
	var b strings.Builder
	b.WriteString("# @aigc1024 Timeline\n\n")
	for _, r := range records {
		b.WriteString(fmt.Sprintf("## %s · #%d\n\n", r.Date.Format("2006-01-02 15:04:05 UTC"), r.ID))
		if r.Text != "" {
			b.WriteString(r.Text)
			b.WriteString("\n\n")
		}
		for _, m := range r.Media {
			switch m.Kind {
			case "image":
				b.WriteString(fmt.Sprintf("![](%s)\n\n", m.RelPath))
			case "video":
				b.WriteString(fmt.Sprintf("[Video](%s)\n\n", m.RelPath))
			default:
				b.WriteString(fmt.Sprintf("[%s](%s)\n\n", m.Name, m.RelPath))
			}
		}
		b.WriteString(fmt.Sprintf("[Telegram](%s)\n\n---\n\n", r.Permalink))
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func renderHTML(path string, channel string, records []MessageRecord) error {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<title>@" + html.EscapeString(channel) + " timeline</title>")
	b.WriteString("<style>body{font-family:Inter,Arial,sans-serif;max-width:980px;margin:20px auto;padding:0 12px;color:#111}article{border:1px solid #eee;border-radius:10px;padding:12px;margin:12px 0}img{max-width:100%;border-radius:8px}video{max-width:100%;border-radius:8px}.meta{color:#666;font-size:13px;margin-bottom:8px}pre{white-space:pre-wrap}</style></head><body>")
	b.WriteString("<h1>@" + html.EscapeString(channel) + " knowledge timeline</h1>")
	for _, r := range records {
		b.WriteString("<article>")
		b.WriteString("<div class=\"meta\">" + html.EscapeString(r.Date.Format("2006-01-02 15:04:05 UTC")) + " · #" + fmt.Sprintf("%d", r.ID) + "</div>")
		if r.Text != "" {
			b.WriteString("<pre>" + html.EscapeString(r.Text) + "</pre>")
		}
		for _, m := range r.Media {
			href := html.EscapeString(m.RelPath)
			switch m.Kind {
			case "image":
				b.WriteString("<p><img src=\"" + href + "\" alt=\"image\"></p>")
			case "video":
				b.WriteString("<p><video controls src=\"" + href + "\"></video><br><a href=\"" + href + "\">Video Link</a></p>")
			default:
				b.WriteString("<p><a href=\"" + href + "\">" + html.EscapeString(m.Name) + "</a></p>")
			}
		}
		b.WriteString("<p><a href=\"" + html.EscapeString(r.Permalink) + "\" target=\"_blank\" rel=\"noopener\">Telegram</a></p>")
		b.WriteString("</article>")
	}
	b.WriteString("</body></html>")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
