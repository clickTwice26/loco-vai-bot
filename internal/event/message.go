package event

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"localoy-bot/internal/service"

	"github.com/bwmarrin/discordgo"
)

// OnMessageCreate handles incoming Discord chat messages.
func OnMessageCreate(locoAI service.LocoAIService, logger *slog.Logger) func(s *discordgo.Session, m *discordgo.MessageCreate) {
	log := logger.With("module", "event-message")

	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Ignore bot messages to avoid loops
		if m.Author == nil || m.Author.Bot {
			return
		}

		cleanContent := strings.TrimSpace(m.Content)
		if cleanContent == "" {
			return
		}

		botUser := s.State.User
		isMentioned := false
		isReplyToBot := false

		// Check if bot was explicitly tagged
		if botUser != nil {
			for _, user := range m.Mentions {
				if user.ID == botUser.ID {
					isMentioned = true
					break
				}
			}

			// Check if message is a Discord reply referencing the bot
			if m.ReferencedMessage != nil && m.ReferencedMessage.Author != nil && m.ReferencedMessage.Author.ID == botUser.ID {
				isReplyToBot = true
			}
		}

		// 1. Check if message is in the dedicated Loco AI chat channel
		if locoAI != nil && locoAI.IsConfigured() && m.ChannelID == locoAI.GetChannelID() {
			ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
			defer cancel()

			// Trigger typing indicator to feel natural
			_ = s.ChannelTyping(m.ChannelID)

			// Clean author name
			authorName := m.Author.Username
			if m.Member != nil && m.Member.Nick != "" {
				authorName = m.Member.Nick
			}

			// Capture reply context if this message is replying to another message in the channel
			formattedContent := cleanContent
			if m.ReferencedMessage != nil {
				refAuthor := "someone"
				if m.ReferencedMessage.Author != nil {
					if m.ReferencedMessage.Member != nil && m.ReferencedMessage.Member.Nick != "" {
						refAuthor = m.ReferencedMessage.Member.Nick
					} else {
						refAuthor = m.ReferencedMessage.Author.Username
					}
				}
				refSnippet := strings.TrimSpace(m.ReferencedMessage.Content)
				if len(refSnippet) > 80 {
					refSnippet = refSnippet[:77] + "..."
				}
				formattedContent = fmt.Sprintf("(replying to %s's \"%s\"): %s", refAuthor, refSnippet, cleanContent)
			}

			// Process message through Loco AI
			reply, responded, err := locoAI.ProcessMessage(ctx, m.ChannelID, authorName, formattedContent, isMentioned, isReplyToBot)
			if err != nil {
				log.Error("failed to process message via Loco AI", "error", err, "author", authorName)
				return
			}

			if responded && reply != "" {
				// Small human-like typing pause before sending
				time.Sleep(600 * time.Millisecond)

				if isReplyToBot || isMentioned {
					_, err = s.ChannelMessageSendReply(m.ChannelID, reply, m.Reference())
				} else {
					_, err = s.ChannelMessageSend(m.ChannelID, reply)
				}

				if err != nil {
					log.Error("failed to send Discord message reply", "error", err)
				}
			}
			return
		}

		// 2. Outside the AI channel: Direct bot mentions prompt a friendly slash-command nudge
		if isMentioned {
			log.Debug("bot mentioned outside AI channel", "channel_id", m.ChannelID, "author", m.Author.Username)
			reply := fmt.Sprintf("👋 Hey %s! Type `/help` to see my commands, or head over to the Loco chat channel to chat with me!", m.Author.Mention())
			_, _ = s.ChannelMessageSendReply(m.ChannelID, reply, m.Reference())
		}
	}
}
