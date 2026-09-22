package loco

import "time"

// UpstreamRequest represents the payload dispatched to n8n / the upstream model.
type UpstreamRequest struct {
	UserID         string    `json:"userId"`
	Name           string    `json:"name"`
	Location       string    `json:"location"`
	ConversationID string    `json:"conversationId"`
	Message        string    `json:"message"`
	SentAt         time.Time `json:"sentAt"`
}

// ExperienceRef represents an experience reference from the model.
type ExperienceRef struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// PlanStepRaw represents a single itinerary step as returned by the upstream model.
type PlanStepRaw struct {
	Order         *int           `json:"order,omitempty"`
	SuggestedTime string         `json:"suggested_time"`
	Experience    *ExperienceRef `json:"experience,omitempty"`
	Note          string         `json:"note"`
}

// UpstreamResponse represents the raw payload returned by n8n.
type UpstreamResponse struct {
	ChatResponse string          `json:"chat_response"`
	ResponseType string          `json:"response_type"`
	HasCards     bool            `json:"has_cards"`
	Experiences  []ExperienceRef `json:"experiences,omitempty"`
	Plan         []PlanStepRaw   `json:"plan,omitempty"`
	BudgetTotal  *float64        `json:"budget_total,omitempty"`
	BudgetUsed   *float64        `json:"budget_used,omitempty"`
	Currency     string          `json:"currency,omitempty"`
}

// CardRef is the payload structure sent to the Partner Backend card hydration endpoint.
type CardRef struct {
	Module string `json:"module"`
	ID     string `json:"id"`
}

// CardsRequest is sent to POST /api/v1/internal/loco/cards.
type CardsRequest struct {
	Refs []CardRef `json:"refs"`
}

// CardMini represents a v4 banner mini card hydrated by the Partner Backend.
type CardMini struct {
	ExperienceItemID string `json:"experience_item_id"`
	Title            string `json:"title,omitempty"`
	ShortDescription string `json:"short_description,omitempty"`
	BannerImage      string `json:"banner_image,omitempty"`
	BannerImageURL   string `json:"banner_image_url,omitempty"`
	HeroImageURL     string `json:"hero_image_url,omitempty"`
	ImageURL         string `json:"image_url,omitempty"`
	Image            string `json:"image,omitempty"`
	CoverImage       string `json:"cover_image,omitempty"`
	ThumbnailURL     string `json:"thumbnail_url,omitempty"`
	AreaName         string `json:"area_name,omitempty"`
	DisplayPrice     string `json:"display_price,omitempty"`
	Rating           string `json:"rating,omitempty"`
	Slug             string `json:"slug,omitempty"`
	WebURL           string `json:"web_url,omitempty"`
}

// GetImageURL returns the first available non-empty image URL.
func (m CardMini) GetImageURL() string {
	if m.HeroImageURL != "" {
		return m.HeroImageURL
	}
	if m.BannerImage != "" {
		return m.BannerImage
	}
	if m.BannerImageURL != "" {
		return m.BannerImageURL
	}
	if m.ImageURL != "" {
		return m.ImageURL
	}
	if m.CoverImage != "" {
		return m.CoverImage
	}
	if m.Image != "" {
		return m.Image
	}
	if m.ThumbnailURL != "" {
		return m.ThumbnailURL
	}
	return ""
}

// HydratedCard pairs a module with its hydrated mini card.
type HydratedCard struct {
	Module string   `json:"module"`
	Mini   CardMini `json:"mini"`
}

// CardsResponse is returned by POST /api/v1/internal/loco/cards.
type CardsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Cards []HydratedCard `json:"cards"`
	} `json:"data"`
	Message string `json:"message,omitempty"`
}

// PlanStep is a normalized plan step paired with an optional hydrated card.
type PlanStep struct {
	Order int           `json:"order"`
	Time  string        `json:"time"`
	Note  string        `json:"note"`
	Ref   *CardRef      `json:"ref,omitempty"`
	Card  *HydratedCard `json:"card,omitempty"`
}

// Budget represents a normalized budget summary.
type Budget struct {
	Total    float64 `json:"total"`
	Used     float64 `json:"used"`
	Currency string  `json:"currency"`
}

// ProcessedResponse is the clean, final normalized structure for the Discord bot.
type ProcessedResponse struct {
	Reply        string         `json:"reply"`
	ResponseType string         `json:"responseType"`
	Cards        []HydratedCard `json:"cards"`
	Refs         []CardRef      `json:"refs"`
	Plan         []PlanStep     `json:"plan"`
	Budget       *Budget        `json:"budget,omitempty"`
}
