package bot

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
)

type activityItem struct {
	activityType discordgo.ActivityType
	name         func(s *discordgo.Session) string
}

// ActivityManager rotates dynamic Discord presence activities periodically.
type ActivityManager struct {
	session    *discordgo.Session
	logger     *slog.Logger
	activities []activityItem
	interval   time.Duration
}

// NewActivityManager initializes an ActivityManager with dynamic presence statuses.
func NewActivityManager(session *discordgo.Session, logger *slog.Logger) *ActivityManager {
	activities := []activityItem{
		{
			activityType: discordgo.ActivityTypeGame, // Displays: "Playing with Gemini AI 🤖"
			name: func(s *discordgo.Session) string {
				return "with Gemini AI 🤖 | /help"
			},
		},
		{
			activityType: discordgo.ActivityTypeWatching, // Displays: "Watching over 1 server 🛡️"
			name: func(s *discordgo.Session) string {
				guildCount := len(s.State.Guilds)
				if guildCount <= 1 {
					return "over 1 server 🛡️"
				}
				return fmt.Sprintf("over %d servers 🛡️", guildCount)
			},
		},
		{
			activityType: discordgo.ActivityTypeListening, // Displays: "Listening to chat vibes 💬"
			name: func(s *discordgo.Session) string {
				return "chat vibes in #loco-vai 💬"
			},
		},
		{
			activityType: discordgo.ActivityTypeCompeting, // Displays: "Competing for top homie 🏆"
			name: func(s *discordgo.Session) string {
				return "for top server homie 🏆"
			},
		},
		{
			activityType: discordgo.ActivityTypeGame, // Displays: "Playing with Loco Vai"
			name: func(s *discordgo.Session) string {
				latency := s.HeartbeatLatency().Milliseconds()
				return fmt.Sprintf("Loco Vai ⚡ (%dms ping)", latency)
			},
		},
	}

	return &ActivityManager{
		session:    session,
		logger:     logger.With("module", "activity-manager"),
		activities: activities,
		interval:   20 * time.Second,
	}
}

// Start launches the periodic activity rotation loop in a background goroutine.
func (am *ActivityManager) Start(ctx context.Context) {
	go func() {
		// Wait 2 seconds for gateway connection to stabilize
		time.Sleep(2 * time.Second)
		am.update(0)

		ticker := time.NewTicker(am.interval)
		defer ticker.Stop()

		idx := 1
		for {
			select {
			case <-ctx.Done():
				am.logger.Debug("activity rotation stopped")
				return
			case <-ticker.C:
				am.update(idx)
				idx = (idx + 1) % len(am.activities)
			}
		}
	}()
}

func (am *ActivityManager) update(index int) {
	if am.session == nil {
		return
	}

	item := am.activities[index%len(am.activities)]
	text := item.name(am.session)

	var err error
	switch item.activityType {
	case discordgo.ActivityTypeGame:
		err = am.session.UpdateGameStatus(0, text)
	case discordgo.ActivityTypeListening:
		err = am.session.UpdateListeningStatus(text)
	case discordgo.ActivityTypeWatching:
		err = am.session.UpdateWatchStatus(0, text)
	case discordgo.ActivityTypeCompeting:
		err = am.session.UpdateStatusComplex(discordgo.UpdateStatusData{
			Status: "online",
			Activities: []*discordgo.Activity{
				{
					Name: text,
					Type: discordgo.ActivityTypeCompeting,
				},
			},
		})
	default:
		err = am.session.UpdateGameStatus(0, text)
	}

	if err != nil {
		am.logger.Warn("failed to update bot status", "error", err)
	} else {
		am.logger.Info("updated bot activity status", "activity", text, "type", item.activityType)
	}
}
