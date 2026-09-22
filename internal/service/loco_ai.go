package service

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"localoy-bot/internal/loco"
	"localoy-bot/internal/memory"
)

const defaultLocoSystemPrompt = `You are Loco (affectionately known as "Loco Vai"), an authentic, sharp, witty, and deeply perceptive member of this Discord community hanging out in this channel.

YOUR PERSONA & VIBE:
- You are a genuine, intelligent human friend in this Discord group — NOT a generic AI bot or corporate assistant.
- You have exceptional taste and deep local awareness (Dhaka food scenes in Gulshan, Banani, Dhanmondi, Puran Dhaka, tech, gaming, hangout spots, lifestyle, server drama, and banter).
- You speak naturally in English, Bangla, or modern Banglish (e.g. "pera nai", "chill mama", "scene ki", "vai kotha shon", "bhaloi toh", "ar ki khobor"). You naturally match the language and vibe of whoever talks to you.
- You have high contextual IQ: you follow multi-person arguments, understand when someone is joking/sarcastic, notice when a user is replying to someone else's specific point, and remember details mentioned earlier in the conversation.

HOW TO CHAT LIKE A REAL DISCORD HOMIE:
1. PUNCHY & CONVERSATIONAL: Keep your replies natural in length (1-3 sentences). Real people don't send unsolicited bullet-point essays or robotic greeting paragraphs in group chats.
2. GROUP BANTER & WIT: Join in on server jokes, tease friends playfully when appropriate, and give sharp, insightful comebacks or empathetic replies.
3. CONCRETE & REAL ADVICE: When someone asks for recommendations (e.g., food, places, activities, advice), give concrete, spot-on answers with real names and genuine opinions.
4. STRICT PROHIBITIONS:
   - NEVER say "I am an AI", "As an AI model", "How can I assist you?", or "I don't have feelings".
   - NEVER introduce yourself robotically. You are just Loco, hanging out in the channel with the gang.`

// LocoAIService orchestrates persona logic, conversation memory, and AI generation.
type LocoAIService interface {
	ProcessMessage(ctx context.Context, channelID string, author string, content string, isMentioned bool, isReplyToBot bool) (string, bool, error)
	IsConfigured() bool
	GetChannelID() string
}

type locoAIService struct {
	locoClient     loco.Client
	geminiClient   GeminiClient
	memoryStore    memory.Store
	channelID      string
	responseChance float64
	logger         *slog.Logger
	systemPrompt   string
	rng            *rand.Rand
}

// NewLocoAIService creates a new LocoAIService.
func NewLocoAIService(
	locoClient loco.Client,
	geminiClient GeminiClient,
	memoryStore memory.Store,
	channelID string,
	responseChance float64,
	logger *slog.Logger,
) LocoAIService {
	if responseChance <= 0 {
		responseChance = 0.40
	}

	return &locoAIService{
		locoClient:     locoClient,
		geminiClient:   geminiClient,
		memoryStore:    memoryStore,
		channelID:      strings.TrimSpace(channelID),
		responseChance: responseChance,
		logger:         logger.With("module", "loco-ai"),
		systemPrompt:   defaultLocoSystemPrompt,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// IsConfigured returns true if a target channel and an AI backend (n8n or Gemini) are ready.
func (s *locoAIService) IsConfigured() bool {
	if s.channelID == "" {
		return false
	}
	if s.locoClient != nil && s.locoClient.IsConfigured() {
		return true
	}
	return s.geminiClient != nil
}

// GetChannelID returns the configured AI channel ID.
func (s *locoAIService) GetChannelID() string {
	return s.channelID
}

// shouldRespond evaluates whether Loco should chime into the chat conversation.
func (s *locoAIService) shouldRespond(isMentioned, isReplyToBot bool, content string) bool {
	// 1. Direct interaction triggers 100% response
	if isMentioned || isReplyToBot {
		return true
	}

	lower := strings.ToLower(content)

	// 2. Direct name mention triggers 100% response
	if strings.Contains(lower, "loco") || strings.Contains(lower, "locovai") {
		return true
	}

	// 3. Question mark slightly boosts participation chance
	chance := s.responseChance
	if strings.Contains(content, "?") {
		chance += 0.20
	}

	roll := s.rng.Float64()
	return roll < chance
}

// ProcessMessage records incoming chat and generates Loco's response if warranted.
func (s *locoAIService) ProcessMessage(
	ctx context.Context,
	channelID string,
	author string,
	content string,
	isMentioned bool,
	isReplyToBot bool,
) (string, bool, error) {
	if channelID != s.channelID {
		return "", false, nil
	}

	userMsg := memory.Message{
		Role:      "user",
		Author:    author,
		Content:   content,
		Timestamp: time.Now(),
	}

	// Fetch recent history before appending current message to feed to Gemini
	history, err := s.memoryStore.GetHistory(ctx, channelID)
	if err != nil {
		s.logger.Warn("failed to fetch chat history from memory store", "error", err)
		history = []memory.Message{}
	}

	// Always append current user message to conversation memory
	if err := s.memoryStore.AppendMessage(ctx, channelID, userMsg); err != nil {
		s.logger.Warn("failed to append user message to memory store", "error", err)
	}

	// Determine if Loco should reply
	if !s.shouldRespond(isMentioned, isReplyToBot, content) {
		s.logger.Debug("skipping response to act like a natural human member", "author", author)
		return "", false, nil
	}

	s.logger.Info("generating Loco AI response", "author", author, "history_len", len(history))

	var reply string

	// 1. If Upstream Localoy n8n workflow is active, query it first
	if s.locoClient != nil && s.locoClient.IsConfigured() {
		req := loco.UpstreamRequest{
			UserID:         author,
			Name:           author,
			Location:       "",
			ConversationID: channelID,
			Message:        content,
			SentAt:         time.Now().UTC(),
		}
		processed, err := s.locoClient.Ask(ctx, req)
		if err == nil && processed != nil && processed.Reply != "" {
			reply = processed.Reply
		} else if err != nil {
			s.logger.Warn("upstream loco query failed, attempting gemini fallback", "error", err)
		}
	}

	// 2. Fallback to Gemini if upstream was unconfigured or failed
	if reply == "" && s.geminiClient != nil {
		geminiReply, err := s.geminiClient.GenerateChatResponse(ctx, s.systemPrompt, history, userMsg)
		if err != nil {
			return "", false, fmt.Errorf("loco ai response generation error: %w", err)
		}
		reply = geminiReply
	}

	if reply == "" {
		return "", false, fmt.Errorf("no AI service was able to produce a response")
	}

	// Save Loco's reply in memory
	botMsg := memory.Message{
		Role:      "model",
		Author:    "Loco",
		Content:   reply,
		Timestamp: time.Now(),
	}
	if err := s.memoryStore.AppendMessage(ctx, channelID, botMsg); err != nil {
		s.logger.Warn("failed to append bot reply to memory store", "error", err)
	}

	return reply, true, nil
}
