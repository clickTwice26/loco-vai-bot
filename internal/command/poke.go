package command

import (
	"context"
	"fmt"
	"time"

	"localoy-bot/internal/service"

	"github.com/bwmarrin/discordgo"
)

// PokeCommand implements the /poke slash command.
type PokeCommand struct {
	poker service.PokerService
}

// NewPokeCommand creates a new PokeCommand.
func NewPokeCommand(poker service.PokerService) *PokeCommand {
	return &PokeCommand{
		poker: poker,
	}
}

// Definition returns the slash command definition for /poke.
func (c *PokeCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "poke",
		Description: "Let Loco Vai playfully poke or roast someone with witty banter",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "target",
				Description: "The person to poke (leave empty for a random victim)",
				Required:    false,
			},
		},
	}
}

// Handle executes the /poke slash command.
func (c *PokeCommand) Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	var targetUser *discordgo.User

	options := i.ApplicationCommandData().Options
	if len(options) > 0 && options[0].Name == "target" {
		targetUser = options[0].UserValue(s)
	}

	if targetUser != nil && targetUser.Bot {
		return RespondEphemeral(s, i, "🤖 Bhai bot der poke kore ki labh? Manush ke poke koro!")
	}

	// Defer reply
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		return fmt.Errorf("failed to defer poke interaction: %w", err)
	}

	pokeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var pokeMessage string
	if targetUser != nil && c.poker != nil {
		msg, err := c.poker.CraftPoke(pokeCtx, targetUser)
		if err != nil {
			msg = fmt.Sprintf("Oi <@%s>, Loco Vai er theke ekta meaningful poke! ☕", targetUser.ID)
		}
		pokeMessage = msg
	} else if c.poker != nil {
		// Pick random candidate
		if err := c.poker.ExecuteRandomPoke(pokeCtx, s); err != nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("⚠️ Could not poke right now: %s", err.Error()),
			})
			return err
		}
		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: "🎯 Loco Vai just dropped a spontaneous poke in the channel!",
		})
		return err
	} else {
		pokeMessage = "⚠️ Poke system is not active."
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: pokeMessage,
	})
	return err
}
