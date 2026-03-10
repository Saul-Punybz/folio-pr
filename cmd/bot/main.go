// Command bot runs the Folio Telegram bot for mobile access and notifications.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/config"
	"github.com/Saul-Punybz/folio/internal/db"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/telegram"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()

	if cfg.Telegram.BotToken == "" {
		slog.Error("TELEGRAM_BOT_TOKEN is required")
		os.Exit(1)
	}

	allowlist := cfg.Telegram.ParseAllowlist()
	if len(allowlist) == 0 {
		slog.Warn("TELEGRAM_ALLOWLIST is empty — no users will be authorized")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database connection
	pool, err := db.Connect(ctx, cfg.DB)
	if err != nil {
		slog.Error("database connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Create stores
	articleStore := models.NewArticleStore(pool)
	briefStore := models.NewBriefStore(pool)
	watchlistOrgStore := models.NewWatchlistOrgStore(pool)
	watchlistHitStore := models.NewWatchlistHitStore(pool)
	telegramUserStore := models.NewTelegramUserStore(pool)
	notificationStore := models.NewNotificationStore(pool)
	researchProjectStore := models.NewResearchProjectStore(pool)
	userStore := models.NewUserStore(pool)

	// AI client
	aiClient := ai.NewClient(
		cfg.Ollama.Host,
		cfg.Ollama.InstructModel,
		cfg.Ollama.EmbedModel,
	)

	// Create bot
	bot, err := telegram.New(cfg.Telegram.BotToken, allowlist, telegram.BotDeps{
		Articles:         articleStore,
		Briefs:           briefStore,
		WatchlistHits:    watchlistHitStore,
		WatchlistOrgs:    watchlistOrgStore,
		TelegramUsers:    telegramUserStore,
		Notifications:    notificationStore,
		ResearchProjects: researchProjectStore,
		AI:               aiClient,
		Users:            userStore,
	})
	if err != nil {
		slog.Error("failed to create bot", "err", err)
		os.Exit(1)
	}

	// Start notification poller in background
	go bot.StartNotificationPoller(ctx)

	// Start bot (blocks until context is cancelled)
	slog.Info("bot starting", "allowlist_size", len(allowlist))

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go bot.Start(ctx)

	sig := <-sigCh
	slog.Info("received shutdown signal", "signal", sig.String())
	cancel()

	slog.Info("bot stopped")
}
