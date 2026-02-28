package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/Saul-Punybz/folio/internal/intelligence"
)

// handleMessage is the default handler for non-command text messages.
// It treats plain text as an AI chat question and responds using local articles
// and web search results.
func (b *Bot) handleMessage(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.Message == nil || update.Message.Text == "" {
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
	_ = user

	// Send typing indicator in a loop until the AI responds.
	typingCtx, typingCancel := context.WithCancel(ctx)
	defer typingCancel()
	go b.sendTypingLoop(typingCtx, bot, update.Message.Chat.ID)

	// Call intelligence.Chat for an AI-powered response.
	resp, err := intelligence.Chat(ctx, intelligence.Deps{
		Articles: b.articles,
		AI:       b.aiClient,
	}, intelligence.ChatRequest{
		Question: update.Message.Text,
	})
	typingCancel()

	if err != nil {
		slog.Error("telegram: chat", "err", err)
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error al procesar tu pregunta. Intenta de nuevo.",
		})
		return
	}

	// Format the AI response with source links.
	text := resp.Answer
	if len(resp.Sources) > 0 {
		text += "\n\n<b>Fuentes locales:</b>"
		for _, s := range resp.Sources {
			text += fmt.Sprintf("\n- <a href=\"%s\">%s</a> [%s]", s.URL, escapeHTML(s.Title), s.Source)
		}
	}
	if len(resp.WebSources) > 0 && len(resp.WebSources) <= 5 {
		text += "\n\n<b>Fuentes web:</b>"
		for _, s := range resp.WebSources {
			text += fmt.Sprintf("\n- <a href=\"%s\">%s</a>", s.URL, escapeHTML(s.Title))
		}
	}

	// Telegram messages have a 4096 character limit.
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

// sendTypingLoop sends ChatActionTyping every 4 seconds until the context is cancelled.
func (b *Bot) sendTypingLoop(ctx context.Context, bot *tgbot.Bot, chatID int64) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	// Send immediately.
	bot.SendChatAction(ctx, &tgbot.SendChatActionParams{
		ChatID: chatID,
		Action: tgmodels.ChatActionTyping,
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			bot.SendChatAction(ctx, &tgbot.SendChatActionParams{
				ChatID: chatID,
				Action: tgmodels.ChatActionTyping,
			})
		}
	}
}

// escapeHTML escapes HTML special characters for Telegram's HTML parse mode.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
