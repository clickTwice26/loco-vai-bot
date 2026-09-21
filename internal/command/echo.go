package command

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

// EchoCommand handles the /echo command.
type EchoCommand struct{}

// NewEchoCommand creates a new EchoCommand instance.
func NewEchoCommand() *EchoCommand {
	return &EchoCommand{}
}

// Definition returns the slash command definition for /echo.
func (c *EchoCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "echo",
		Description: "Repeat a message back to you",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "message",
				Description: "The message to repeat",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "ephemeral",
				Description: "Whether only you should see the response (default: false)",
				Required:    false,
			},
		},
	}
}

// Handle executes the /echo command.
func (c *EchoCommand) Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	msg := optionMap["message"].StringValue()
	ephemeral := false
	if opt, ok := optionMap["ephemeral"]; ok {
		ephemeral = opt.BoolValue()
	}

	var flags discordgo.MessageFlags
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   flags,
		},
	})
}
