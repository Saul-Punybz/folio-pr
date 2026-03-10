package telegram

import (
	"context"
	"fmt"
	"log/slog"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
)

// Bot wraps the Telegram bot with Folio-specific handlers and data access.
type Bot struct {
	bot              *tgbot.Bot
	allowlist        map[int64]string // telegramID -> folio email
	articles         *models.ArticleStore
	briefs           *models.BriefStore
	watchlistHits    *models.WatchlistHitStore
	watchlistOrgs    *models.WatchlistOrgStore
	telegramUsers    *models.TelegramUserStore
	notifications    *models.NotificationStore
	researchProjects *models.ResearchProjectStore
	aiClient         *ai.OllamaClient
	users            *models.UserStore
}

// BotDeps groups the dependencies required by the Telegram bot.
type BotDeps struct {
	Articles         *models.ArticleStore
	Briefs           *models.BriefStore
	WatchlistHits    *models.WatchlistHitStore
	WatchlistOrgs    *models.WatchlistOrgStore
	TelegramUsers    *models.TelegramUserStore
	Notifications    *models.NotificationStore
	ResearchProjects *models.ResearchProjectStore
	AI               *ai.OllamaClient
	Users            *models.UserStore
}

// New creates a new Telegram bot with the given token, user allowlist, and dependencies.
func New(token string, allowlist map[int64]string, deps BotDeps) (*Bot, error) {
	b := &Bot{
		allowlist:        allowlist,
		articles:         deps.Articles,
		briefs:           deps.Briefs,
		watchlistHits:    deps.WatchlistHits,
		watchlistOrgs:    deps.WatchlistOrgs,
		telegramUsers:    deps.TelegramUsers,
		notifications:    deps.Notifications,
		researchProjects: deps.ResearchProjects,
		aiClient:         deps.AI,
		users:            deps.Users,
	}

	opts := []tgbot.Option{
		tgbot.WithDefaultHandler(b.handleMessage),
	}

	tg, err := tgbot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("telegram bot: %w", err)
	}
	b.bot = tg

	// Register command handlers.
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypePrefix, b.handleStart)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/help", tgbot.MatchTypePrefix, b.handleHelp)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/inbox", tgbot.MatchTypePrefix, b.handleInbox)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/saved", tgbot.MatchTypePrefix, b.handleSaved)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/search", tgbot.MatchTypePrefix, b.handleSearch)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/brief", tgbot.MatchTypePrefix, b.handleBrief)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/watchlist", tgbot.MatchTypePrefix, b.handleWatchlist)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, "/research", tgbot.MatchTypePrefix, b.handleResearch)

	// Register callback query handler for inline keyboard buttons.
	b.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, "", tgbot.MatchTypePrefix, b.handleCallback)

	return b, nil
}

// Start begins polling for Telegram updates. Blocks until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) {
	slog.Info("telegram bot: starting")
	b.bot.Start(ctx)
}

// compile-time check to ensure tgmodels is used (needed for handlers in other files).
var _ = tgmodels.ParseModeHTML
