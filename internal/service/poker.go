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
	"<@%s> bhai tumi ki shudhu Discord e online thakar salary pao naki kaj o koro? Shotto kore bolo. 😂",
	"<@%s> are you actually working or just shifting between browser tabs to look productive? We see you.",
	"<@%s> shobai dekhlam kaj kortese, ar tumi ekhane shanti moto CCTV camera hoye lurk korteso. Shundor system!",
	"Kire <@%s>, ghumabe kobe? Raat-din online dekhi, tumi ki cyborg naki? 🤖",
	"<@%s> ekta serious question chilo... coffee shesh hoise naki brain er battery shesh hoise?",
	"<@%s> joto time Discord e spend korteso tar 10%% time code e dile amra koyek mash agei release diye ditam bhai. 🚀",
	"Oi <@%s>, silent spectator hoye thakle cholbe? Kono update ache naki full chill mode?",
	"<@%s> bhai tumi ki ghosting er master class niccho naki keyboard haraye gese? Kichu toh bolo! 👻",
	"<@%s> ekta plan bana, Gulshan e naki Banani te biryani khawabe kobe? Shudhu kotha bolle hobe na. 🍗",
}

const pokeSystemInstruction = `You are Loco (Loco Vai), an exceptionally smart, hilarious, perceptive Discord friend hanging out in this server.
Your mission: Spontaneously poke or playfully roast a specific user with razor-sharp wit and personality.

CRITICAL RULES:
1. CONTEXTUAL & SMART: Don't give a boring generic greeting. Make it feel personalized, perceptive, and observant. Call them out on their habits, what they said, lurking, or the time of day.
2. WITTY & SAVAGE (NOT HURTFUL): Deliver clever, funny, sometimes brutally savage banter, but NEVER be genuinely toxic, abusive, or hurtful. It must feel like hilarious group chat banter among close homies.
3. LANGUAGE & PRONOUNS: Natural Banglish (e.g. "Oi...", "Kire...", "Bro...", "pera nai...", "shotti kore bolo...") or casual English matching the vibe.
   PRONOUN MANDATE: Always address the user with "tumi" (or "apni"). NEVER use "tui", "tor", or "tore" under any circumstances.
4. COMPLETE THOUGHT: You MUST write a 100% complete, grammatically finished sentence that ends with punctuation (?, !, or .). NEVER trail off or leave a sentence cut off mid-thought.
5. LENGTH: 1 to 2 punchy, completed sentences.
6. You MUST include their exact mention tag directly in the response.`

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

	// Enforce minimum gap between pokes
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

// CraftPoke uses Gemini with deep multi-dimensional context to craft an intelligent, perceptive poke.
func (ps *pokerService) CraftPoke(ctx context.Context, target *discordgo.User) (string, error) {
	if ps.geminiClient == nil {
		return ps.fallbackPoke(target), nil
	}

	// 1. Fetch recent chat history
	var history []memory.Message
	if ps.memoryStore != nil {
		if h, err := ps.memoryStore.GetHistory(ctx, ps.channelID); err == nil {
			history = h
		}
	}

	// 2. Compute time-of-day context (Dhaka Time UTC+6)
	dhakaLoc := time.FixedZone("Asia/Dhaka", 6*3600)
	now := time.Now().In(dhakaLoc)
	hour := now.Hour()
	var timeContext string
	switch {
	case hour >= 0 && hour < 5:
		timeContext = fmt.Sprintf("Late night (%d:%02d AM). If funny, tease their nocturnal sleep schedule, insomnia, or late night screen glare.", hour, now.Minute())
	case hour >= 5 && hour < 11:
		timeContext = fmt.Sprintf("Morning (%d:%02d AM). If funny, tease them about waking up, whether their brain is working, or needing coffee/cha.", hour, now.Minute())
	case hour >= 11 && hour < 16:
		timeContext = fmt.Sprintf("Midday/Lunch time (%d:%02d PM). If funny, tease about food, hunger, post-lunch bhaat ghoom, or surviving tasks.", hour, now.Minute())
	case hour >= 16 && hour < 20:
		timeContext = fmt.Sprintf("Late afternoon/Evening (%d:%02d PM). Tease about wrapping up, adda, snacks, or chill plans.", hour, now.Minute())
	default:
		timeContext = fmt.Sprintf("Night (%d:%02d PM). Peak adda time, dinner plans, gaming, or chilling.", hour, now.Minute())
	}

	// 3. Analyze target's recent statements in history
	var targetLastStatement string
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role == "user" && (strings.EqualFold(msg.Author, target.Username) || strings.Contains(msg.Author, target.Username)) {
			targetLastStatement = msg.Content
			break
		}
	}

	var activityContext string
	if targetLastStatement != "" {
		activityContext = fmt.Sprintf("Target recently said: \"%s\". If relevant, make a sharp witty callback to that!", targetLastStatement)
	} else {
		activityContext = "Target has NOT spoken recently in the channel. They are lurking silently or ghosting. Tease them for being a silent spectator / ghosting!"
	}

	displayName := target.Username
	mentionTag := fmt.Sprintf("<@%s>", target.ID)

	prompt := fmt.Sprintf(`Target User: %s (%s)
Time of Day: %s
User Activity: %s

Task: Deliver an intelligent, witty, slightly savage but good-humored poke/roast to %s. Remember to include %s directly.`,
		displayName, mentionTag, timeContext, activityContext, displayName, mentionTag,
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
	reply = strings.Trim(reply, "\"")

	// Sanitize pronouns: enforce 'tumi' over 'tui'
	reply = sanitizePronouns(reply)

	// Ensure sentence is not truncated
	reply = ensureSentenceCompletion(reply)

	// Ensure mention tag is present
	if !strings.Contains(reply, target.ID) && !strings.Contains(reply, mentionTag) {
		reply = fmt.Sprintf("%s %s", mentionTag, reply)
	}

	return reply, nil
}

func sanitizePronouns(s string) string {
	// Replace any accidental 'tui' variants with 'tumi'
	replacements := []struct {
		old string
		new string
	}{
		{" tui ", " tumi "},
		{" Tui ", " Tumi "},
		{" tui,", " tumi,"},
		{" Tui,", " Tumi,"},
		{" tui?", " tumi?"},
		{" Tui?", " Tumi?"},
		{" tor ", " tomar "},
		{" Tor ", " Tomar "},
		{" tore ", " tomake "},
		{" Tore ", " Tomake "},
		{" tuke ", " tomake "},
		{" ghumabi ", " ghumabe "},
		{" khawabi ", " khawabe "},
		{" bolbi ", " bolbe "},
		{" korbi ", " korbe "},
	}
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.old, r.new)
	}
	if strings.HasPrefix(s, "Tui ") {
		s = "Tumi " + s[4:]
	} else if strings.HasPrefix(s, "tui ") {
		s = "tumi " + s[4:]
	}
	return s
}

func ensureSentenceCompletion(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	lower := strings.ToLower(s)
	// If it was cut off mid-thought on common dangling Bangla/English words, complete it naturally
	switch {
	case strings.HasSuffix(lower, "tui ki") || strings.HasSuffix(lower, "tumi ki"):
		return s + " ghumabe na? 😴"
	case strings.HasSuffix(lower, "are you"):
		return s + " sleeping or still awake? 👀"
	case strings.HasSuffix(lower, "ki"):
		return s + " obostha? 😂"
	case strings.HasSuffix(lower, "na"):
		return s + "?"
	}

	// If missing ending punctuation, add a question mark or exclamation mark depending on structure
	lastChar := s[len(s)-1:]
	if lastChar != "." && lastChar != "!" && lastChar != "?" && lastChar != "…" {
		if strings.Contains(lower, "ki ") || strings.Contains(lower, "kobe") || strings.Contains(lower, "why") || strings.Contains(lower, "how") {
			s += "?"
		} else {
			s += " 😂"
		}
	}

	return s
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
