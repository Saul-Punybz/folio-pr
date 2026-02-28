package telegram

import (
	"context"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"github.com/google/uuid"
)

// handleCallback processes inline keyboard button presses.
// Callback data format: "action:UUID" (e.g. "save:550e8400-e29b-41d4-a716-446655440000").
func (b *Bot) handleCallback(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
	if update.CallbackQuery == nil {
		return
	}

	data := update.CallbackQuery.Data
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return
	}

	action := parts[0]
	idPrefix := parts[1]

	// Parse the article UUID from the callback data.
	articleID, err := uuid.Parse(idPrefix)
	if err != nil {
		bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            "Error: ID no valido",
		})
		return
	}

	var responseText string
	switch action {
	case "save":
		err = b.articles.UpdateStatus(ctx, articleID, "saved")
		responseText = "Guardado"
	case "trash":
		err = b.articles.UpdateStatus(ctx, articleID, "trashed")
		responseText = "Archivado"
	case "pin":
		err = b.articles.SetPinned(ctx, articleID, true)
		responseText = "Fijado"
	default:
		responseText = "Accion desconocida"
	}

	if err != nil {
		slog.Error("telegram: callback", "action", action, "err", err)
		responseText = "Error: " + err.Error()
	}

	bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		Text:            responseText,
	})
}
