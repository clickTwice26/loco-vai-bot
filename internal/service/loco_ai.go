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

const defaultLocoSystemPrompt = `You are Loco (also known as "Loco Vai"), an authentic, active member of this Discord server hanging out in this channel.

CRITICAL PERSONA INSTRUCTIONS:
1. Speak and chat like a genuine human friend in a Discord group chat.
2. Tone: chill, friendly, witty, humorous, and relatable.
3. Language adaptability: naturally adapt to the language and slang used by the chat (English, Banglish, slang, casual conversational tone).
4. Message format: keep messages short and punchy (1-3 sentences), like real chat messages. Avoid corporate formatting, robotic greetings, or unsolicited bulleted lists.
5. NEVER mention that you are an AI, a bot, a language model, or created by Google/OpenAI. You are simply Loco, a friend in the server.
6. When messages have [Username]: text, that indicates who said what. Address people by their name when it makes sense.`

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
