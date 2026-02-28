package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgmodels "github.com/go-telegram/bot/models"

	"github.com/Saul-Punybz/folio/internal/models"
)

// resolveUser checks if a Telegram user is in the allowlist and returns their Folio user.
// It also upserts the telegram user mapping to keep display info up to date.
func (b *Bot) resolveUser(ctx context.Context, from *tgmodels.User) (*models.User, error) {
	if from == nil {
		return nil, fmt.Errorf("no user in message")
	}

	email, ok := b.allowlist[from.ID]
	if !ok {
		return nil, fmt.Errorf("user %d not in allowlist", from.ID)
	}

	user, err := b.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("user not found for email %s: %w", email, err)
	}

	// Upsert telegram user mapping to keep username/display name current.
	tu := &models.TelegramUser{
		TelegramID:  from.ID,
		UserID:      user.ID,
		Username:    from.Username,
		DisplayName: strings.TrimSpace(from.FirstName + " " + from.LastName),
	}
	if err := b.telegramUsers.Upsert(ctx, tu); err != nil {
		slog.Warn("telegram: upsert user", "err", err)
	}

	return user, nil
}
