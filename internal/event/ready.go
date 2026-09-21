package event

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// OnReady handles the Discord Ready gateway event.
func OnReady(logger *slog.Logger) func(s *discordgo.Session, r *discordgo.Ready) {
	return func(s *discordgo.Session, r *discordgo.Ready) {
		logger.Info("bot connected to Discord gateway successfully",
			"user", fmt.Sprintf("%s#%s", r.User.Username, r.User.Discriminator),
			"user_id", r.User.ID,
			"guilds_count", len(r.Guilds),
		)

		// Set default presence status
		err := s.UpdateCustomStatus("Serving /help commands")
		if err != nil {
			logger.Warn("failed to update bot status", "error", err)
		}
	}
}
