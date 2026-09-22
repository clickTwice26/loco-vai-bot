package loco

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

var validModules = map[string]bool{
	"event":    true,
	"watch":    true,
	"offer":    true,
	"dining":   true,
	"activity": true,
	"store":    true,
}

// Client interacts with the upstream n8n model and Partner Backend card hydration API.
type Client interface {
	Ask(ctx context.Context, req UpstreamRequest) (*ProcessedResponse, error)
	IsConfigured() bool
}

type client struct {
	upstreamURL   string
	signingSecret string
	partnerURL    string
	serviceToken  string
	httpClient    *http.Client
	logger        *slog.Logger
}

// NewClient initializes a new Loco Upstream Client.
func NewClient(
	upstreamURL string,
	signingSecret string,
	partnerURL string,
	serviceToken string,
	logger *slog.Logger,
) Client {
	return &client{
		upstreamURL:   strings.TrimSpace(upstreamURL),
		signingSecret: strings.TrimSpace(signingSecret),
		partnerURL:    strings.TrimSuffix(strings.TrimSpace(partnerURL), "/"),
		serviceToken:  strings.TrimSpace(serviceToken),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With("module", "loco-upstream"),
	}
}

// IsConfigured returns true if the upstream model URL and signing secret are set.
func (c *client) IsConfigured() bool {
	return c.upstreamURL != "" && c.signingSecret != ""
}

// Ask sends the user's message to n8n with an HMAC-SHA256 signature and hydrates any experience cards.
func (c *client) Ask(ctx context.Context, req UpstreamRequest) (*ProcessedResponse, error) {
	if !c.IsConfigured() {
		return nil, errors.New("LOCO_UPSTREAM_URL or LOCO_WEBHOOK_SIGNING_SECRET is not configured")
	}

	if req.SentAt.IsZero() {
		req.SentAt = time.Now().UTC()
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode upstream request: %w", err)
	}

	// Sign payload
	sigHeader := GenerateSignature(c.signingSecret, bodyBytes, req.SentAt)

	// Time budget for upstream is 25s
	upstreamCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, c.upstreamURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Localoy-Signature", sigHeader)

	c.logger.Debug("dispatching request to upstream model",
		"url", c.upstreamURL,
		"user_id", req.UserID,
		"conversation_id", req.ConversationID,
	)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errors.New("AI service timed out")
		}
		return nil, fmt.Errorf("AI service is unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AI service error (status %d)", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read upstream response body: %w", err)
	}

	var upstreamResp UpstreamResponse
	if err := json.Unmarshal(respBody, &upstreamResp); err != nil {
		return nil, fmt.Errorf("AI service returned a malformed response: %w", err)
	}

	reply := strings.TrimSpace(upstreamResp.ChatResponse)
	if reply == "" {
		return nil, errors.New("AI service returned an empty chat response")
	}

	responseType := "normal"
	if strings.ToLower(upstreamResp.ResponseType) == "itinerary" {
		responseType = "itinerary"
	}

	// Collect and validate all experience refs
	refsMap := make(map[string]CardRef)
	var refsList []CardRef

	for _, exp := range upstreamResp.Experiences {
		module := strings.ToLower(strings.TrimSpace(exp.Type))
		id := strings.TrimSpace(exp.ID)
		if id != "" && validModules[module] {
			key := fmt.Sprintf("%s:%s", module, id)
			if _, exists := refsMap[key]; !exists {
				ref := CardRef{Module: module, ID: id}
				refsMap[key] = ref
				refsList = append(refsList, ref)
			}
		}
	}

	// Also check plan step experiences
	for _, step := range upstreamResp.Plan {
		if step.Experience != nil {
			module := strings.ToLower(strings.TrimSpace(step.Experience.Type))
			id := strings.TrimSpace(step.Experience.ID)
			if id != "" && validModules[module] {
				key := fmt.Sprintf("%s:%s", module, id)
				if _, exists := refsMap[key]; !exists {
					ref := CardRef{Module: module, ID: id}
					refsMap[key] = ref
					refsList = append(refsList, ref)
				}
			}
		}
	}

	// Hydrate cards if Partner Backend is configured (fails open)
	hydratedMap := make(map[string]HydratedCard)
	if len(refsList) > 0 && c.partnerURL != "" && c.serviceToken != "" {
		hydratedCards, err := c.hydrateCards(ctx, refsList)
		if err != nil {
			c.logger.Warn("partner cards hydration failed, proceeding without cards", "error", err)
		} else {
			for _, card := range hydratedCards {
				key := fmt.Sprintf("%s:%s", card.Module, card.Mini.ExperienceItemID)
				hydratedMap[key] = card
			}
		}
	}

	// Pair cards with refs in order
	var finalCards []HydratedCard
	for _, ref := range refsList {
		key := fmt.Sprintf("%s:%s", ref.Module, ref.ID)
		if card, exists := hydratedMap[key]; exists {
			finalCards = append(finalCards, card)
		}
	}

	// Build plan steps
	var finalPlan []PlanStep
	for idx, step := range upstreamResp.Plan {
		order := idx + 1
		if step.Order != nil {
			order = *step.Order
		}

		ps := PlanStep{
			Order: order,
			Time:  step.SuggestedTime,
			Note:  step.Note,
		}

		if step.Experience != nil {
			module := strings.ToLower(strings.TrimSpace(step.Experience.Type))
			id := strings.TrimSpace(step.Experience.ID)
			if id != "" && validModules[module] {
				ref := CardRef{Module: module, ID: id}
				ps.Ref = &ref

				key := fmt.Sprintf("%s:%s", module, id)
				if card, exists := hydratedMap[key]; exists {
					cardCopy := card
					ps.Card = &cardCopy
				}
			}
		}

		finalPlan = append(finalPlan, ps)
	}

	// Build budget
	var finalBudget *Budget
	if upstreamResp.BudgetTotal != nil || upstreamResp.BudgetUsed != nil {
		currency := strings.TrimSpace(upstreamResp.Currency)
		if currency == "" {
			currency = "BDT"
		}
		total := 0.0
		if upstreamResp.BudgetTotal != nil {
			total = *upstreamResp.BudgetTotal
		}
		used := 0.0
		if upstreamResp.BudgetUsed != nil {
			used = *upstreamResp.BudgetUsed
		}
		finalBudget = &Budget{
			Total:    total,
			Used:     used,
			Currency: currency,
		}
	}

	return &ProcessedResponse{
		Reply:        reply,
		ResponseType: responseType,
		Cards:        finalCards,
		Refs:         refsList,
		Plan:         finalPlan,
		Budget:       finalBudget,
	}, nil
}

// hydrateCards calls POST /api/v1/internal/loco/cards with a 3s budget.
func (c *client) hydrateCards(ctx context.Context, refs []CardRef) ([]HydratedCard, error) {
	cardCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("%s/api/v1/internal/loco/cards", c.partnerURL)
	payload := CardsRequest{Refs: refs}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(cardCtx, http.MethodPost, endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.serviceToken)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("card hydration endpoint returned status %d", resp.StatusCode)
	}

	var cardsResp CardsResponse
	if err := json.NewDecoder(resp.Body).Decode(&cardsResp); err != nil {
		return nil, err
	}

	return cardsResp.Data.Cards, nil
}
