package loco

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var moduleIcons = map[string]string{
	"dining":   "🍽️",
	"event":    "🎟️",
	"offer":    "🏷️",
	"watch":    "🎬",
	"activity": "🧗",
	"store":    "🛍️",
}

var moduleColors = map[string]int{
	"dining":   0xE67E22, // Orange
	"event":    0x9B59B6, // Purple
	"offer":    0x2ECC71, // Green
	"watch":    0xE74C3C, // Red
	"activity": 0x1ABC9C, // Teal
	"store":    0x3498DB, // Blue
}

// FormatDiscordEmbeds turns a ProcessedResponse into one or more rich Discord embeds with experience card images.
func FormatDiscordEmbeds(resp *ProcessedResponse, queryUser string) []*discordgo.MessageEmbed {
	embedColor := 0x5865F2 // Discord Blurple
	title := "✨ Loco AI"

	if resp.ResponseType == "itinerary" {
		embedColor = 0xF1C40F // Gold
		title = "🗺️ Localoy Plan & Itinerary"
	}

	mainEmbed := &discordgo.MessageEmbed{
		Title:       title,
		Description: resp.Reply,
		Color:       embedColor,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Loco AI • Requested by %s", queryUser),
		},
	}

	// 1. Format Itinerary Steps
	if len(resp.Plan) > 0 {
		var planLines []string
		for _, step := range resp.Plan {
			stepTitle := step.Note
			timePrefix := ""
			if strings.TrimSpace(step.Time) != "" {
				timePrefix = fmt.Sprintf("`%s` • ", step.Time)
			}

			cardDetails := ""
			if step.Card != nil && step.Card.Mini.Title != "" {
				icon := moduleIcons[step.Card.Module]
				if icon == "" {
					icon = "📍"
				}
				cardDetails = fmt.Sprintf("\n   └ %s **%s**", icon, step.Card.Mini.Title)
				if step.Card.Mini.DisplayPrice != "" {
					cardDetails += fmt.Sprintf(" *(%s)*", step.Card.Mini.DisplayPrice)
				}
			}

			planLines = append(planLines, fmt.Sprintf("**%d.** %s%s%s", step.Order, timePrefix, stepTitle, cardDetails))
		}

		mainEmbed.Fields = append(mainEmbed.Fields, &discordgo.MessageEmbedField{
			Name:   "📅 Itinerary Timeline",
			Value:  strings.Join(planLines, "\n\n"),
			Inline: false,
		})
	}

	// 2. Format Budget Summary
	if resp.Budget != nil && (resp.Budget.Total > 0 || resp.Budget.Used > 0) {
		budgetValue := fmt.Sprintf("💵 **Used:** `%.0f %s` / **Total:** `%.0f %s`",
			resp.Budget.Used, resp.Budget.Currency,
			resp.Budget.Total, resp.Budget.Currency,
		)
		mainEmbed.Fields = append(mainEmbed.Fields, &discordgo.MessageEmbedField{
			Name:   "💰 Budget Summary",
			Value:  budgetValue,
			Inline: true,
		})
	}

	embeds := []*discordgo.MessageEmbed{mainEmbed}

	// 3. Attach individual card embeds with images (up to 4 cards to respect Discord limits)
	cardsToRender := resp.Cards
	if len(cardsToRender) == 0 && len(resp.Plan) > 0 {
		for _, step := range resp.Plan {
			if step.Card != nil {
				cardsToRender = append(cardsToRender, *step.Card)
			}
		}
	}

	// If there's only 1 card with an image, attach it as the thumbnail of the main embed
	if len(cardsToRender) == 1 {
		imgURL := cardsToRender[0].Mini.GetImageURL()
		if imgURL != "" {
			mainEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: imgURL}
		}
	}

	// Build individual card embeds for rich image display
	for idx, card := range cardsToRender {
		if idx >= 4 { // Max 4 card embeds per response
			break
		}

		if strings.TrimSpace(card.Mini.Title) == "" {
			continue
		}

		icon := moduleIcons[card.Module]
		if icon == "" {
			icon = "📍"
		}

		cardColor := moduleColors[card.Module]
		if cardColor == 0 {
			cardColor = 0x5865F2
		}

		var descParts []string
		if card.Mini.ShortDescription != "" {
			descParts = append(descParts, card.Mini.ShortDescription)
		}

		var metaParts []string
		if card.Mini.AreaName != "" {
			metaParts = append(metaParts, "📍 "+card.Mini.AreaName)
		}
		if card.Mini.DisplayPrice != "" {
			metaParts = append(metaParts, "💵 "+card.Mini.DisplayPrice)
		}
		if card.Mini.Rating != "" {
			metaParts = append(metaParts, "⭐ "+card.Mini.Rating)
		}

		if len(metaParts) > 0 {
			descParts = append(descParts, strings.Join(metaParts, " • "))
		}

		desc := strings.Join(descParts, "\n\n")
		if desc == "" {
			desc = "Localoy Experience"
		}

		cardEmbed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("%s %s", icon, card.Mini.Title),
			Description: desc,
			Color:       cardColor,
		}

		// Attach experience image
		imgURL := card.Mini.GetImageURL()
		if imgURL != "" {
			cardEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: imgURL}
		}

		embeds = append(embeds, cardEmbed)
	}

	return embeds
}

// FormatDiscordEmbed is a backward-compatible wrapper returning the primary embed.
func FormatDiscordEmbed(resp *ProcessedResponse, queryUser string) *discordgo.MessageEmbed {
	embeds := FormatDiscordEmbeds(resp, queryUser)
	if len(embeds) > 0 {
		return embeds[0]
	}
	return nil
}
