package command

import (
	"context"
	"fmt"
	"time"

	"localoy-bot/internal/service"

	"github.com/bwmarrin/discordgo"
)

// PingCommand handles the /ping command.
type PingCommand struct {
	systemService service.SystemService
}

// NewPingCommand creates a new PingCommand instance.
func NewPingCommand(sys service.SystemService) *PingCommand {
	return &PingCommand{
		systemService: sys,
	}
}

// Definition returns the Discord slash command definition for /ping.
func (c *PingCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "ping",
		Description: "Check bot latency, gateway heartbeat, and uptime",
	}
}

// Handle executes the /ping command.
func (c *PingCommand) Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	start := time.Now()

	// Initial response
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		return fmt.Errorf("failed to defer interaction: %w", err)
	}

	roundtripLatency := time.Since(start).Milliseconds()
	heartbeatLatency := s.HeartbeatLatency().Milliseconds()
	uptime := c.systemService.FormatUptime()
	mem := c.systemService.GetMemoryUsage()

	embed := &discordgo.MessageEmbed{
		Title: "🏓 Pong!",
		Color: 0x5865F2, // Discord Blurple
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Roundtrip Latency",
				Value:  fmt.Sprintf("`%d ms`", roundtripLatency),
				Inline: true,
			},
			{
				Name:   "Gateway Heartbeat",
				Value:  fmt.Sprintf("`%d ms`", heartbeatLatency),
				Inline: true,
			},
			{
				Name:   "Uptime",
				Value:  fmt.Sprintf("`%s`", uptime),
				Inline: true,
			},
			{
				Name:   "Memory (Alloc / Sys)",
				Value:  fmt.Sprintf("`%.2f MB / %.2f MB`", mem.AllocatedMB, mem.SysMB),
				Inline: true,
			},
			{
				Name:   "Goroutines",
				Value:  fmt.Sprintf("`%d`", mem.Goroutines),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Localoy Bot",
		},
	}

	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	return err
}
