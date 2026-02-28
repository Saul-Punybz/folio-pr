package telegram

import (
	"fmt"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
)

// formatArticle renders an article as an HTML-formatted Telegram message.
func formatArticle(a models.Article) string {
	var s string
	s += fmt.Sprintf("<b>%s</b>\n", escapeHTML(a.Title))
	s += fmt.Sprintf("%s", escapeHTML(a.Source))
	if a.PublishedAt != nil {
		s += fmt.Sprintf(" | %s", timeAgo(*a.PublishedAt))
	}
	s += fmt.Sprintf("\n<a href=\"%s\">Leer articulo</a>", a.URL)
	if a.Summary != "" {
		summary := a.Summary
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		s += fmt.Sprintf("\n<i>%s</i>", escapeHTML(summary))
	}
	return s
}

// timeAgo returns a human-readable Spanish time-ago string.
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "ahora"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("hace %dm", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("hace %dh", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("hace %dd", days)
	}
}
