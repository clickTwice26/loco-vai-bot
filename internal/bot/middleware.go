package bot

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"localoy-bot/internal/command"

	"github.com/bwmarrin/discordgo"
)

// LoggingMiddleware logs details of each slash command execution including runtime duration.
func LoggingMiddleware(logger *slog.Logger) command.Middleware {
	return func(next command.HandlerFunc) command.HandlerFunc {
		return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
			start := time.Now()
			cmdName := i.ApplicationCommandData().Name
			user := command.InteractionUser(i)

			logger.Info("executing command",
				"command", cmdName,
				"user_id", user.ID,
				"username", user.Username,
				"guild_id", i.GuildID,
				"channel_id", i.ChannelID,
			)

			err := next(ctx, s, i)
			duration := time.Since(start)

			if err != nil {
				logger.Error("command completed with error",
					"command", cmdName,
					"duration_ms", duration.Milliseconds(),
					"error", err,
				)
			} else {
				logger.Info("command completed successfully",
					"command", cmdName,
					"duration_ms", duration.Milliseconds(),
				)
			}

			return err
		}
	}
}

// RecoveryMiddleware captures panics during command handling and responds gracefully.
func RecoveryMiddleware(logger *slog.Logger) command.Middleware {
	return func(next command.HandlerFunc) command.HandlerFunc {
		return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) (err error) {
			defer func() {
				if r := recover(); r != nil {
					cmdName := i.ApplicationCommandData().Name
					stack := string(debug.Stack())
					logger.Error("panic recovered during command execution",
						"command", cmdName,
						"panic", r,
						"stack", stack,
					)
					err = fmt.Errorf("internal panic: %v", r)
					_ = command.RespondEphemeral(s, i, "🚨 An unexpected internal error occurred.")
				}
			}()

			return next(ctx, s, i)
		}
	}
}
