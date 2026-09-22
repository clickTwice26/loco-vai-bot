package command

import (
	"context"
	"fmt"
	"time"

	"localoy-bot/internal/loco"
	"localoy-bot/internal/service"

	"github.com/bwmarrin/discordgo"
)

// LocoCommand implements the /loco slash command.
type LocoCommand struct {
	locoClient   loco.Client
	geminiClient service.GeminiClient
}

// NewLocoCommand creates a new LocoCommand.
func NewLocoCommand(locoClient loco.Client, geminiClient service.GeminiClient) *LocoCommand {
	return &LocoCommand{
		locoClient:   locoClient,
		geminiClient: geminiClient,
	}
}

// Definition returns the slash command definition for /loco.
func (c *LocoCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "loco",
		Description: "Ask Loco AI for recommendations, hangout spots, and curated itineraries",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "prompt",
				Description: "What are you looking for? (e.g. 'cheap dinner in Gulshan tonight')",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "location",
				Description: "Area / neighborhood (e.g. 'Gulshan', 'Banani', 'Dhanmondi')",
				Required:    false,
			},
		},
	}
}

// Handle processes the /loco slash command.
func (c *LocoCommand) Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	user := InteractionUser(i)
	options := i.ApplicationCommandData().Options
	optMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optMap[opt.Name] = opt
	}

	prompt := optMap["prompt"].StringValue()
	location := ""
	if opt, ok := optMap["location"]; ok {
		location = opt.StringValue()
	}

	// Defer reply since model takes 2-8 seconds
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		return fmt.Errorf("failed to defer interaction: %w", err)
	}

	// If Upstream n8n / Localoy model is configured, query it
	if c.locoClient != nil && c.locoClient.IsConfigured() {
		req := loco.UpstreamRequest{
			UserID:         user.ID,
			Name:           user.Username,
			Location:       location,
			ConversationID: user.ID,
			Message:        prompt,
			SentAt:         time.Now().UTC(),
		}

		resp, err := c.locoClient.Ask(ctx, req)
		if err != nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("⚠️ Loco encountered an issue: %s", err.Error()),
			})
			return err
		}

		embeds := loco.FormatDiscordEmbeds(resp, user.Username)
		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: embeds,
		})
		return err
	}

	// Fallback to Gemini if upstream model is not configured
	if c.geminiClient != nil {
		systemPrompt := "You are Loco (Loco Vai), a friendly local guide for dining, events, and hangout spots."
		reply, err := c.geminiClient.GenerateChatResponse(ctx, systemPrompt, nil, service.UserChatMessage(user.Username, prompt))
		if err != nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "⚠️ Could not connect to Loco AI right now.",
			})
			return err
		}

		embed := &discordgo.MessageEmbed{
			Title:       "✨ Loco AI",
			Description: reply,
			Color:       0x5865F2,
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("Requested by %s", user.Username),
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}

		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embed},
		})
		return err
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: "⚠️ Loco AI is not currently configured.",
	})
	return err
}
