package event

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// OnGuildCreate handles the GuildCreate event (bot joined a guild or loaded cached guild).
func OnGuildCreate(logger *slog.Logger) func(s *discordgo.Session, g *discordgo.GuildCreate) {
	return func(s *discordgo.Session, g *discordgo.GuildCreate) {
		logger.Info("guild joined / available",
			"guild_id", g.ID,
			"guild_name", g.Name,
			"member_count", g.MemberCount,
		)
	}
}

// OnGuildDelete handles the GuildDelete event (bot removed or kicked from a guild).
func OnGuildDelete(logger *slog.Logger) func(s *discordgo.Session, g *discordgo.GuildDelete) {
	return func(s *discordgo.Session, g *discordgo.GuildDelete) {
		logger.Info("guild left / removed",
			"guild_id", g.ID,
		)
	}
}
