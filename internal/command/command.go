package command

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

// HandlerFunc defines the signature for executing a slash command interaction.
type HandlerFunc func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error

// Middleware defines a function that wraps a HandlerFunc.
type Middleware func(next HandlerFunc) HandlerFunc

// Command defines the interface that all slash commands must implement.
type Command interface {
	// Definition provides the Discord ApplicationCommand specification (name, description, options, permissions).
	Definition() *discordgo.ApplicationCommand
	// Handle processes the interaction when this command is invoked.
	Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error
}

// Registry manages command registration, middleware chaining, and interaction routing.
type Registry struct {
	commands    map[string]Command
	ordered     []Command
	middlewares []Middleware
	logger      *slog.Logger
}

// NewRegistry initializes an empty Command Registry.
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		commands:    make(map[string]Command),
		ordered:     make([]Command, 0),
		middlewares: make([]Middleware, 0),
		logger:      logger.With("module", "command-registry"),
	}
}

// Use adds one or more middlewares to the command execution pipeline.
func (r *Registry) Use(middlewares ...Middleware) {
	r.middlewares = append(r.middlewares, middlewares...)
}

// Register adds one or more commands to the registry.
func (r *Registry) Register(cmds ...Command) {
	for _, cmd := range cmds {
		name := cmd.Definition().Name
		if _, exists := r.commands[name]; exists {
			r.logger.Warn("overwriting already registered command", "command", name)
		}
		r.commands[name] = cmd
		r.ordered = append(r.ordered, cmd)
		r.logger.Debug("registered command", "command", name)
	}
}

// Get looks up a command by name.
func (r *Registry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// All returns all registered commands in registration order.
func (r *Registry) All() []Command {
	return r.ordered
}

// Definitions returns the slice of discordgo.ApplicationCommand definitions for API sync.
func (r *Registry) Definitions() []*discordgo.ApplicationCommand {
	defs := make([]*discordgo.ApplicationCommand, 0, len(r.ordered))
	for _, cmd := range r.ordered {
		defs = append(defs, cmd.Definition())
	}
	return defs
}

// Sync overwrites the application commands on Discord (guild-scoped if guildID != "", otherwise global).
func (r *Registry) Sync(s *discordgo.Session, appID, guildID string) ([]*discordgo.ApplicationCommand, error) {
	defs := r.Definitions()
	scope := "global"
	if guildID != "" {
		scope = fmt.Sprintf("guild (%s)", guildID)
	}

	r.logger.Info("syncing application commands with Discord", "count", len(defs), "scope", scope)

	created, err := s.ApplicationCommandBulkOverwrite(appID, guildID, defs)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk overwrite application commands: %w", err)
	}

	r.logger.Info("successfully synced application commands", "count", len(created), "scope", scope)
	return created, nil
}

// RemoveAll deletes all registered commands from Discord (for clean teardown if enabled).
func (r *Registry) RemoveAll(s *discordgo.Session, appID, guildID string) error {
	r.logger.Info("removing application commands from Discord", "guildID", guildID)
	_, err := s.ApplicationCommandBulkOverwrite(appID, guildID, []*discordgo.ApplicationCommand{})
	if err != nil {
		return fmt.Errorf("failed to remove application commands: %w", err)
	}
	return nil
}

// InteractionUser returns the user who initiated the interaction (whether in a guild or DM).
func InteractionUser(i *discordgo.InteractionCreate) *discordgo.User {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User
	}
	if i.User != nil {
		return i.User
	}
	return &discordgo.User{Username: "unknown"}
}

// Route handles an incoming interaction by matching it to a registered command and executing the middleware chain.
func (r *Registry) Route(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	cmdName := i.ApplicationCommandData().Name
	cmd, exists := r.commands[cmdName]
	if !exists {
		r.logger.Warn("received interaction for unknown command", "command", cmdName)
		_ = RespondEphemeral(s, i, "❌ Unknown command.")
		return
	}

	// Build handler with middleware chain
	handler := cmd.Handle
	for idx := len(r.middlewares) - 1; idx >= 0; idx-- {
		handler = r.middlewares[idx](handler)
	}

	ctx := context.Background()
	if err := handler(ctx, s, i); err != nil {
		user := InteractionUser(i)
		r.logger.Error("command execution failed",
			"command", cmdName,
			"user", user.Username,
			"error", err,
		)

		// Respond with user-friendly error if interaction hasn't been responded to yet
		_ = RespondEphemeral(s, i, fmt.Sprintf("⚠️ An error occurred while executing `/%s`.", cmdName))
	}
}

// RespondEphemeral is a helper function to quickly reply with an ephemeral message.
func RespondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, message string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
