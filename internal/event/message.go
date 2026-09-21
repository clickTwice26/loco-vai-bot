package event

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// OnMessageCreate handles general message events.
// For example, if the bot is mentioned, it replies directing the user to use slash commands.
func OnMessageCreate(logger *slog.Logger) func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Ignore bot messages to avoid loops
		if m.Author == nil || m.Author.Bot {
			return
		}

		// Check if the bot was directly mentioned
		if s.State.User != nil {
			for _, user := range m.Mentions {
				if user.ID == s.State.User.ID {
					logger.Debug("bot was mentioned in channel", "channel_id", m.ChannelID, "author", m.Author.Username)
					reply := fmt.Sprintf("👋 Hey %s! I use slash commands now. Type `/help` to see what I can do!", m.Author.Mention())
					_, _ = s.ChannelMessageSendReply(m.ChannelID, reply, m.Reference())
					return
				}
			}
		}
	}
}
