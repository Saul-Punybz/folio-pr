package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// handleStart welcomes the user if they are in the allowlist.
func (b *Bot) handleStart(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	user, err := b.resolveUser(ctx, update.Message.From)
	if err != nil {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No autorizado. Este bot es privado.",
		})
		return
	}

	text := fmt.Sprintf(`Bienvenido a <b>Folio Bot</b>, %s.

Tu centro de inteligencia politica de Puerto Rico, ahora en Telegram.

<b>Comandos disponibles:</b>
/inbox - Ver articulos recientes
/saved - Ver articulos guardados
/search &lt;query&gt; - Buscar articulos
/brief - Resumen diario de inteligencia
/watchlist - Alertas de vigilancia
/research &lt;tema&gt; - Investigacion profunda
/help - Lista de comandos

Tambien puedes enviar cualquier pregunta como texto libre y el asistente de IA te respondera usando las noticias locales.`, escapeHTML(user.Email))

	bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// handleHelp sends a list of available commands.
func (b *Bot) handleHelp(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	_, err := b.resolveUser(ctx, update.Message.From)
	if err != nil {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No autorizado.",
		})
		return
	}

	text := `<b>Comandos de Folio Bot</b>

/inbox - Ver los 5 articulos mas recientes en tu inbox
/saved - Ver articulos que has guardado
/search &lt;query&gt; - Buscar articulos por palabra clave
/brief - Ver el resumen diario de inteligencia
/watchlist - Ver alertas recientes de tu lista de vigilancia
/research &lt;tema&gt; - Lanzar investigacion profunda sobre un tema
/research status - Ver estado de investigaciones
/help - Mostrar este mensaje

<b>Chat con IA:</b>
Envia cualquier pregunta como texto libre y el asistente de IA respondera usando las noticias y fuentes web.`

	bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// handleInbox shows the 5 most recent inbox articles with action buttons.
func (b *Bot) handleInbox(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	_, err := b.resolveUser(ctx, update.Message.From)
	if err != nil {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No autorizado.",
		})
		return
	}

	articles, err := b.articles.ListByStatus(ctx, "inbox", 5, 0)
	if err != nil {
		slog.Error("telegram: inbox", "err", err)
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error al cargar el inbox.",
		})
		return
	}

	if len(articles) == 0 {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Tu inbox esta vacio.",
		})
		return
	}

	for _, a := range articles {
		text := formatArticle(a)
		idStr := a.ID.String()

		// Build inline keyboard with Save and Trash buttons.
		keyboard := &tgmodels.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
				{
					{Text: "Guardar", CallbackData: "save:" + idStr},
					{Text: "Archivar", CallbackData: "trash:" + idStr},
				},
			},
		}

		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:             update.Message.Chat.ID,
			Text:               text,
			ParseMode:          tgmodels.ParseModeHTML,
			ReplyMarkup:        keyboard,
			LinkPreviewOptions: &tgmodels.LinkPreviewOptions{IsDisabled: tgbot.True()},
		})
	}
}

// handleSaved shows saved articles.
func (b *Bot) handleSaved(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	_, err := b.resolveUser(ctx, update.Message.From)
	if err != nil {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No autorizado.",
		})
		return
	}

	articles, err := b.articles.ListByStatus(ctx, "saved", 5, 0)
	if err != nil {
		slog.Error("telegram: saved", "err", err)
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error al cargar los guardados.",
		})
		return
	}

	if len(articles) == 0 {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No tienes articulos guardados.",
		})
		return
	}

	for _, a := range articles {
		text := formatArticle(a)
		idStr := a.ID.String()

		keyboard := &tgmodels.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgmodels.InlineKeyboardButton{
				{
					{Text: "Fijar", CallbackData: "pin:" + idStr},
					{Text: "Archivar", CallbackData: "trash:" + idStr},
				},
			},
		}

		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:             update.Message.Chat.ID,
			Text:               text,
			ParseMode:          tgmodels.ParseModeHTML,
			ReplyMarkup:        keyboard,
			LinkPreviewOptions: &tgmodels.LinkPreviewOptions{IsDisabled: tgbot.True()},
		})
	}
}

// handleSearch searches articles by a query extracted from the message text.
func (b *Bot) handleSearch(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	_, err := b.resolveUser(ctx, update.Message.From)
	if err != nil {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No autorizado.",
		})
		return
	}

	// Parse query from "/search <query>".
	query := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/search"))
	if query == "" {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Uso: /search &lt;palabra clave&gt;",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	articles, err := b.articles.SearchChat(ctx, query, 10)
	if err != nil {
		slog.Error("telegram: search", "err", err, "query", query)
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error en la busqueda.",
		})
		return
	}

	if len(articles) == 0 {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   fmt.Sprintf("No se encontraron resultados para: %s", escapeHTML(query)),
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Resultados para:</b> %s\n\n", escapeHTML(query)))

	for i, a := range articles {
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b>\n", i+1, escapeHTML(a.Title)))
		sb.WriteString(fmt.Sprintf("   %s", escapeHTML(a.Source)))
		if a.PublishedAt != nil {
			sb.WriteString(fmt.Sprintf(" | %s", timeAgo(*a.PublishedAt)))
		}
		sb.WriteString(fmt.Sprintf("\n   <a href=\"%s\">Leer</a>\n\n", a.URL))
	}

	text := sb.String()
	if len(text) > 4000 {
		text = text[:4000] + "..."
	}

	bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:             update.Message.Chat.ID,
		Text:               text,
		ParseMode:          tgmodels.ParseModeHTML,
		LinkPreviewOptions: &tgmodels.LinkPreviewOptions{IsDisabled: tgbot.True()},
	})
}

// handleBrief sends the latest daily intelligence brief.
func (b *Bot) handleBrief(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	_, err := b.resolveUser(ctx, update.Message.From)
	if err != nil {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No autorizado.",
		})
		return
	}

	brief, err := b.briefs.GetLatest(ctx)
	if err != nil {
		slog.Error("telegram: brief", "err", err)
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No hay resumen disponible todavia.",
		})
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Resumen Diario - %s</b>\n", brief.Date.Format("2 Jan 2006")))
	sb.WriteString(fmt.Sprintf("Articulos procesados: %d\n\n", brief.ArticleCount))
	sb.WriteString(escapeHTML(brief.Summary))

	if len(brief.TopTags) > 0 {
		sb.WriteString("\n\n<b>Temas principales:</b> ")
		sb.WriteString(escapeHTML(strings.Join(brief.TopTags, ", ")))
	}

	text := sb.String()
	if len(text) > 4000 {
		text = text[:4000] + "..."
	}

	bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

// handleWatchlist shows recent watchlist hits for the user.
func (b *Bot) handleWatchlist(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.Message == nil {
		return
	}

	user, err := b.resolveUser(ctx, update.Message.From)
	if err != nil {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No autorizado.",
		})
		return
	}

	hits, err := b.watchlistHits.ListRecentByUser(ctx, user.ID, 10)
	if err != nil {
		slog.Error("telegram: watchlist", "err", err)
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error al cargar la lista de vigilancia.",
		})
		return
	}

	if len(hits) == 0 {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No hay alertas recientes de vigilancia.",
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("<b>Alertas de Vigilancia</b>\n\n")

	for i, h := range hits {
		seenIcon := ""
		if !h.Seen {
			seenIcon = " [NUEVO]"
		}
		sb.WriteString(fmt.Sprintf("%d. <b>%s</b>%s\n", i+1, escapeHTML(h.OrgName), seenIcon))
		sb.WriteString(fmt.Sprintf("   %s\n", escapeHTML(h.Title)))
		sb.WriteString(fmt.Sprintf("   %s | %s\n", escapeHTML(h.SourceType), timeAgo(h.CreatedAt)))
		sb.WriteString(fmt.Sprintf("   <a href=\"%s\">Ver</a>\n\n", h.URL))
	}

	text := sb.String()
	if len(text) > 4000 {
		text = text[:4000] + "..."
	}

	bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:             update.Message.Chat.ID,
		Text:               text,
		ParseMode:          tgmodels.ParseModeHTML,
		LinkPreviewOptions: &tgmodels.LinkPreviewOptions{IsDisabled: tgbot.True()},
	})
}
