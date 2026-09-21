package bot

import (
	"context"
	"fmt"
	"log/slog"

	"localoy-bot/config"
	"localoy-bot/internal/command"
	"localoy-bot/internal/event"
	"localoy-bot/internal/service"

	"github.com/bwmarrin/discordgo"
)

// Bot manages the Discord session lifecycle, command registration, and event listeners.
type Bot struct {
	session       *discordgo.Session
	cfg           *config.Config
	logger        *slog.Logger
	registry      *command.Registry
	systemService service.SystemService
	locoAIService service.LocoAIService
}

// New creates and initializes a new Bot instance.
func New(
	cfg *config.Config,
	logger *slog.Logger,
	registry *command.Registry,
	systemService service.SystemService,
	locoAIService service.LocoAIService,
) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create discordgo session: %w", err)
	}

	// Configure gateway intents
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	return &Bot{
		session:       session,
		cfg:           cfg,
		logger:        logger.With("module", "bot"),
		registry:      registry,
		systemService: systemService,
		locoAIService: locoAIService,
	}, nil
}

// Start opens the Discord gateway connection and registers application commands.
func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("registering event handlers...")

	// Register lifecycle and gateway events
	b.session.AddHandler(event.OnReady(b.logger))
	b.session.AddHandler(event.OnGuildCreate(b.logger))
	b.session.AddHandler(event.OnGuildDelete(b.logger))
	b.session.AddHandler(event.OnMessageCreate(b.locoAIService, b.logger))

	// Register interaction router
	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		b.registry.Route(s, i)
	})

	b.logger.Info("connecting to Discord gateway...")
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("failed to open discord gateway session: %w", err)
	}

	// Synchronize slash commands
	if _, err := b.registry.Sync(b.session, b.cfg.AppID, b.cfg.GuildID); err != nil {
		b.logger.Error("failed to sync slash commands", "error", err)
		return fmt.Errorf("command synchronization failed: %w", err)
	}

	b.logger.Info("bot is now fully online and listening for events")
	return nil
}

// Stop cleanly terminates the Discord connection and performs necessary teardown.
func (b *Bot) Stop(ctx context.Context) error {
	b.logger.Info("shutting down bot...")

	if b.cfg.RemoveCommandsOnShutdown {
		b.logger.Info("removing registered commands from Discord...")
		if err := b.registry.RemoveAll(b.session, b.cfg.AppID, b.cfg.GuildID); err != nil {
			b.logger.Warn("failed to cleanly delete commands on shutdown", "error", err)
		}
	}

	if err := b.session.Close(); err != nil {
		return fmt.Errorf("error closing discord session: %w", err)
	}

	b.logger.Info("discord session closed successfully")
	return nil
}

// Session returns the underlying discordgo Session.
func (b *Bot) Session() *discordgo.Session {
	return b.session
}
