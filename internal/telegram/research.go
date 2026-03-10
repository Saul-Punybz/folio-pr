package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"

	"github.com/Saul-Punybz/folio/internal/models"
)

// handleResearch handles /research commands:
// /research <topic> — create new research project
// /research status  — list all projects with status
// /research         — show help
func (b *Bot) handleResearch(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update) {
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

	// Parse argument after "/research "
	arg := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/research"))

	// No argument: show help
	if arg == "" {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      researchHelpText(),
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	// "status" subcommand
	if strings.ToLower(arg) == "status" {
		b.handleResearchStatus(ctx, bot, update, user)
		return
	}

	// Otherwise, create a new research project with the argument as topic
	b.handleResearchCreate(ctx, bot, update, user, arg)
}

func (b *Bot) handleResearchCreate(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update, user *models.User, topic string) {
	if b.researchProjects == nil {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "La funcion de investigacion no esta disponible.",
		})
		return
	}

	project := &models.ResearchProject{
		UserID:   user.ID,
		Topic:    topic,
		Keywords: []string{},
	}

	if err := b.researchProjects.Create(ctx, project); err != nil {
		slog.Error("telegram: research create", "err", err)
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error al crear la investigacion.",
		})
		return
	}

	text := fmt.Sprintf(`<b>Investigacion Iniciada</b>

<b>Tema:</b> %s
<b>Estado:</b> En cola
<b>ID:</b> <code>%s</code>

El sistema procesara tu investigacion automaticamente. Recibiras una notificacion cuando complete cada fase.

Usa /research status para ver el progreso.`, escapeHTML(topic), project.ID.String())

	bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      text,
		ParseMode: tgmodels.ParseModeHTML,
	})
}

func (b *Bot) handleResearchStatus(ctx context.Context, bot *tgbot.Bot, update *tgmodels.Update, user *models.User) {
	if b.researchProjects == nil {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "La funcion de investigacion no esta disponible.",
		})
		return
	}

	projects, err := b.researchProjects.ListByUser(ctx, user.ID)
	if err != nil {
		slog.Error("telegram: research status", "err", err)
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Error al cargar las investigaciones.",
		})
		return
	}

	if len(projects) == 0 {
		bot.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "No tienes investigaciones. Usa /research &lt;tema&gt; para crear una.",
			ParseMode: tgmodels.ParseModeHTML,
		})
		return
	}

	var sb strings.Builder
	sb.WriteString("<b>Investigaciones</b>\n\n")

	for i, p := range projects {
		if i >= 10 {
			sb.WriteString(fmt.Sprintf("\n... y %d mas", len(projects)-10))
			break
		}

		emoji := statusEmoji(p.Status)
		sb.WriteString(fmt.Sprintf("%s <b>%s</b>\n", emoji, escapeHTML(p.Topic)))
		sb.WriteString(fmt.Sprintf("   Estado: %s | Fase: %d/3\n", p.Status, p.Phase))
		sb.WriteString(fmt.Sprintf("   %s\n\n", timeAgo(p.CreatedAt)))
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

func statusEmoji(status string) string {
	switch status {
	case "queued":
		return "⏳"
	case "searching":
		return "🔍"
	case "scraping":
		return "📄"
	case "synthesizing":
		return "🧠"
	case "done":
		return "✅"
	case "failed":
		return "❌"
	case "cancelled":
		return "🚫"
	default:
		return "❓"
	}
}

func researchHelpText() string {
	return `<b>Investigacion Profunda</b>

Lanza una investigacion detallada sobre cualquier tema en Puerto Rico. El sistema busca en multiples fuentes, recopila informacion y genera un dossier completo.

<b>Comandos:</b>
/research &lt;tema&gt; — Crear nueva investigacion
/research status — Ver estado de investigaciones

<b>Ejemplo:</b>
/research reciclaje
/research energia renovable
/research corrupcion municipal`
}
