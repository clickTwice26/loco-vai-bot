package service

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"localoy-bot/internal/memory"

	"github.com/bwmarrin/discordgo"
)

var defaultFallbackPokes = []string{
	"Oi <@%s>, screen er dike eivabe takaye thakle code automatically fix hoye jabe na, start typing! ☕",
	"<@%s> bhai tumi ki shudhu Discord e online thakar salary pao naki kaj o koro? 😂",
	"<@%s> are you actually working or just shifting between browser tabs to look busy? Shotto kore bolo.",
	"<@%s> shobai dekhlam kaj kortese, ar tumi ekhane shanti moto lurk korteso. Shundor system!",
	"Kire <@%s>, ghumabi kobe? Shob shomoy online dekhi, tumi ki cyborg naki? 🤖",
	"<@%s> ekta serious question chilo... coffee shesh hoise naki brain er battery shesh hoise?",
	"<@%s> joto time Discord e spend korteso tar 10%% time code e dile amra koyek mash agei release diye ditam bhai. 🚀",
	"Oi <@%s>, silent spectator hoye thakle cholbe? Kono update ache naki chill mode e aso?",
}

const pokeSystemInstruction = `You are Loco (Loco Vai), a witty, sharp, hilarious friend hanging out in this Discord server.
Your task: Deliver a spontaneous, meaningful poke or light-hearted roast targeting a specific user.

CRITICAL GUIDELINES:
1. Tone: Playful group chat banter. Make it clever, witty, and sometimes brutally savage in a funny homie way.
2. ABSOLUTELY FORBIDDEN: Never be toxic, hateful, genuinely hurtful, or abusive. It must feel like hilarious banter among close friends that makes everyone laugh.
3. Language: Natural Bangla/Banglish or English matching the chat's vibe (e.g. "Oi @user...", "Kire @user...", "Bro @user...").
4. Length: 1 to 2 punchy, conversational sentences.
5. You MUST include the target user's mention tag directly in your response.`

// PokerService periodically and randomly selects a server member to poke in the chat channel.
type PokerService interface {
	Start(ctx context.Context, session *discordgo.Session)
	ExecuteRandomPoke(ctx context.Context, session *discordgo.Session) error
	CraftPoke(ctx context.Context, target *discordgo.User) (string, error)
}

type pokerService struct {
	geminiClient GeminiClient
	memoryStore  memory.Store
	channelID    string
	minInterval  time.Duration
	maxInterval  time.Duration
	logger       *slog.Logger
	rng          *rand.Rand
	mu           sync.Mutex
	lastPokeTime time.Time
}

// NewPokerService initializes a PokerService.
func NewPokerService(
	geminiClient GeminiClient,
	memoryStore memory.Store,
	channelID string,
	minIntervalMinutes int,
	maxIntervalMinutes int,
	logger *slog.Logger,
) PokerService {
	if minIntervalMinutes <= 0 {
		minIntervalMinutes = 60
	}
	if maxIntervalMinutes <= minIntervalMinutes {
		maxIntervalMinutes = minIntervalMinutes + 60
	}

	return &pokerService{
		geminiClient: geminiClient,
		memoryStore:  memoryStore,
		channelID:    strings.TrimSpace(channelID),
		minInterval:  time.Duration(minIntervalMinutes) * time.Minute,
		maxInterval:  time.Duration(maxIntervalMinutes) * time.Minute,
		logger:       logger.With("module", "auto-poker"),
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Start runs the randomized poke loop in a background goroutine.
func (ps *pokerService) Start(ctx context.Context, session *discordgo.Session) {
	if ps.channelID == "" {
		ps.logger.Info("auto-poker is disabled (no LOCO_CHANNEL_ID configured)")
		return
	}

	go func() {
		// Calculate initial delay: randomly up to minInterval minutes (minimum 1 minute)
		minMins := int(ps.minInterval.Minutes())
		initialDelayMinutes := 1
		if minMins > 1 {
			initialDelayMinutes = 1 + ps.rng.Intn(minMins)
		}

		ps.logger.Info("auto-poker scheduled",
			"channel_id", ps.channelID,
			"min_gap", ps.minInterval,
			"first_poke_in_mins", initialDelayMinutes,
		)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(initialDelayMinutes) * time.Minute):
		}

		for {
			// Trigger a poke
			pokeCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			if err := ps.ExecuteRandomPoke(pokeCtx, session); err != nil {
				ps.logger.Warn("failed to execute auto poke", "error", err)
			}
			cancel()

			// Calculate next randomized delay with a strict minimum gap of minInterval
			extraGapMinutes := 0
			delta := int((ps.maxInterval - ps.minInterval).Minutes())
			if delta > 0 {
				extraGapMinutes = ps.rng.Intn(delta)
			}
			nextDelay := ps.minInterval + time.Duration(extraGapMinutes)*time.Minute

			ps.logger.Info("next auto poke scheduled", "wait_duration", nextDelay.String())

			select {
			case <-ctx.Done():
				ps.logger.Debug("auto-poker loop terminated")
				return
			case <-time.After(nextDelay):
			}
		}
	}()
}

// ExecuteRandomPoke picks an active human user and sends a tailored poke.
func (ps *pokerService) ExecuteRandomPoke(ctx context.Context, session *discordgo.Session) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Enforce 1-hour minimum gap between pokes
	if !ps.lastPokeTime.IsZero() && time.Since(ps.lastPokeTime) < ps.minInterval {
		return fmt.Errorf("poke rate limit enforced: last poke was %s ago", time.Since(ps.lastPokeTime))
	}

	targetUser, err := ps.pickRandomCandidate(session)
	if err != nil {
		return fmt.Errorf("unable to pick poke target: %w", err)
	}

	pokeMessage, err := ps.CraftPoke(ctx, targetUser)
	if err != nil {
		ps.logger.Warn("error crafting AI poke, using witty fallback", "error", err)
		pokeMessage = ps.fallbackPoke(targetUser)
	}

	// Trigger realistic typing indicator
	_ = session.ChannelTyping(ps.channelID)
	time.Sleep(2 * time.Second)

	_, err = session.ChannelMessageSend(ps.channelID, pokeMessage)
	if err != nil {
		return fmt.Errorf("failed to send poke message to discord: %w", err)
	}

	ps.lastPokeTime = time.Now()
	ps.logger.Info("successfully poked user", "user_id", targetUser.ID, "username", targetUser.Username)

	// Save to memory store so Loco and users remember the poke context
	if ps.memoryStore != nil {
		_ = ps.memoryStore.AppendMessage(ctx, ps.channelID, memory.Message{
			Role:      "model",
			Author:    "Loco",
			Content:   pokeMessage,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// CraftPoke uses Gemini or contextual logic to craft a meaningful, witty poke.
func (ps *pokerService) CraftPoke(ctx context.Context, target *discordgo.User) (string, error) {
	if ps.geminiClient == nil {
		return ps.fallbackPoke(target), nil
	}

	// Fetch recent chat history to ground the poke in reality
	var history []memory.Message
	if ps.memoryStore != nil {
		if h, err := ps.memoryStore.GetHistory(ctx, ps.channelID); err == nil {
			history = h
		}
	}

	displayName := target.Username
	mentionTag := fmt.Sprintf("<@%s>", target.ID)

	prompt := fmt.Sprintf(
		"Give a random, spontaneous, meaningful poke/roast to %s in the chat. Use their mention tag %s in the sentence.",
		displayName, mentionTag,
	)

	currentMsg := memory.Message{
		Role:      "user",
		Author:    "System",
		Content:   prompt,
		Timestamp: time.Now(),
	}

	reply, err := ps.geminiClient.GenerateChatResponse(ctx, pokeSystemInstruction, history, currentMsg)
	if err != nil {
		return "", err
	}

	reply = strings.TrimSpace(reply)
	// Ensure mention tag is present
	if !strings.Contains(reply, target.ID) && !strings.Contains(reply, mentionTag) {
		reply = fmt.Sprintf("%s %s", mentionTag, reply)
	}

	return reply, nil
}

// pickRandomCandidate selects a non-bot user who recently participated in the channel or server.
func (ps *pokerService) pickRandomCandidate(session *discordgo.Session) (*discordgo.User, error) {
	selfUser := session.State.User
	candidateMap := make(map[string]*discordgo.User)

	// 1. Try to find candidates from recent channel messages first (active chatters)
	messages, err := session.ChannelMessages(ps.channelID, 50, "", "", "")
	if err == nil && len(messages) > 0 {
		for _, m := range messages {
			if m.Author != nil && !m.Author.Bot {
				if selfUser == nil || m.Author.ID != selfUser.ID {
					candidateMap[m.Author.ID] = m.Author
				}
			}
		}
	}

	// 2. If channel messages have no candidates, check guild members
	if len(candidateMap) == 0 {
		channel, err := session.Channel(ps.channelID)
		if err == nil && channel.GuildID != "" {
			members, err := session.GuildMembers(channel.GuildID, "", 100)
			if err == nil {
				for _, mem := range members {
					if mem.User != nil && !mem.User.Bot {
						if selfUser == nil || mem.User.ID != selfUser.ID {
							candidateMap[mem.User.ID] = mem.User
						}
					}
				}
			}
		}
	}

	if len(candidateMap) == 0 {
		return nil, fmt.Errorf("no suitable human candidates found to poke")
	}

	// Randomly select one candidate
	candidates := make([]*discordgo.User, 0, len(candidateMap))
	for _, u := range candidateMap {
		candidates = append(candidates, u)
	}

	return candidates[ps.rng.Intn(len(candidates))], nil
}

func (ps *pokerService) fallbackPoke(target *discordgo.User) string {
	idx := ps.rng.Intn(len(defaultFallbackPokes))
	template := defaultFallbackPokes[idx]
	return fmt.Sprintf(template, target.ID)
}
