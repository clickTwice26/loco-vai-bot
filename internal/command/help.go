package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// HelpCommand dynamically renders the list of available slash commands.
type HelpCommand struct {
	getCommands func() []Command
}

// NewHelpCommand creates a new HelpCommand instance.
func NewHelpCommand(getCommands func() []Command) *HelpCommand {
	return &HelpCommand{
		getCommands: getCommands,
	}
}

// Definition returns the slash command definition for /help.
func (c *HelpCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "help",
		Description: "Show a list of all available commands and their usage",
	}
}

// Handle executes the /help command.
func (c *HelpCommand) Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	commands := c.getCommands()

	var sb strings.Builder
	for _, cmd := range commands {
		def := cmd.Definition()
		sb.WriteString(fmt.Sprintf("• `/%s` - %s\n", def.Name, def.Description))

		for _, opt := range def.Options {
			reqStr := "optional"
			if opt.Required {
				reqStr = "required"
			}
			sb.WriteString(fmt.Sprintf("   └ `[%s]` *(%s)*: %s\n", opt.Name, reqStr, opt.Description))
		}
		sb.WriteString("\n")
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📖 Localoy Bot Commands",
		Description: sb.String(),
		Color:       0x5865F2,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Type / followed by the command name to use it",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}
