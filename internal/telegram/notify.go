package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
)

// StartNotificationPoller polls for pending notifications every 30 seconds and
// delivers them to the corresponding Telegram users. Blocks until ctx is cancelled.
func (b *Bot) StartNotificationPoller(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	slog.Info("telegram: notification poller started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("telegram: notification poller stopped")
			return
		case <-ticker.C:
			b.deliverPendingNotifications(ctx)
		}
	}
}

// deliverPendingNotifications fetches pending notifications and sends them via Telegram.
func (b *Bot) deliverPendingNotifications(ctx context.Context) {
	pending, err := b.notifications.ListPending(ctx, 20)
	if err != nil {
		slog.Error("telegram: list pending notifications", "err", err)
		return
	}

	for _, notif := range pending {
		// Find the telegram user for this folio user.
		tu, err := b.telegramUsers.GetByUserID(ctx, notif.UserID)
		if err != nil {
			slog.Warn("telegram: no telegram user for notification", "user_id", notif.UserID)
			// Mark as delivered to avoid infinite retries for users without Telegram.
			_ = b.notifications.MarkDelivered(ctx, notif.ID)
			continue
		}

		var text string
		switch notif.Type {
		case "digest":
			var payload struct {
				Summary string `json:"summary"`
			}
			json.Unmarshal(notif.Payload, &payload)
			text = fmt.Sprintf("<b>Resumen Diario</b>\n\n%s", escapeHTML(payload.Summary))
			if len(text) > 4000 {
				text = text[:4000] + "..."
			}

		case "watchlist_hit":
			var payload struct {
				OrgName string `json:"org_name"`
				Title   string `json:"title"`
				URL     string `json:"url"`
			}
			json.Unmarshal(notif.Payload, &payload)
			text = fmt.Sprintf("<b>Alerta Vigilancia: %s</b>\n\n%s\n<a href=\"%s\">Ver</a>",
				escapeHTML(payload.OrgName), escapeHTML(payload.Title), payload.URL)

		case "system":
			var payload struct {
				Message string `json:"message"`
			}
			json.Unmarshal(notif.Payload, &payload)
			text = payload.Message

		default:
			text = "Notificacion sin formato"
		}

		_, err = b.bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:             tu.TelegramID,
			Text:               text,
			ParseMode:          tgmodels.ParseModeHTML,
			LinkPreviewOptions: &tgmodels.LinkPreviewOptions{IsDisabled: tgbot.True()},
		})
		if err != nil {
			slog.Error("telegram: send notification", "err", err, "telegram_id", tu.TelegramID)
			continue
		}

		if err := b.notifications.MarkDelivered(ctx, notif.ID); err != nil {
			slog.Error("telegram: mark delivered", "err", err)
		}
	}
}
