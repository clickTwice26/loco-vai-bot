package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"localoy-bot/internal/rag"

	"github.com/bwmarrin/discordgo"
)

// KBCommand manages the /kb slash command for Knowledge Base interactions.
type KBCommand struct {
	ragEngine rag.Engine
}

// NewKBCommand creates a new KBCommand instance.
func NewKBCommand(ragEngine rag.Engine) *KBCommand {
	return &KBCommand{
		ragEngine: ragEngine,
	}
}

// Definition returns the slash command definition for /kb.
func (c *KBCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "kb",
		Description: "Inspect, search, and update the Localoy Knowledge Base & RAG memory",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "search",
				Description: "Search the knowledge base using semantic vector retrieval",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "query",
						Description: "What are you searching for?",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "add",
				Description: "Add a new knowledge entry to the RAG memory",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "title",
						Description: "Title of the topic (e.g. 'Deployments', 'Shagato')",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "content",
						Description: "Detailed facts, description, or notes",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "category",
						Description: "Category: team, systems, product, or general",
						Required:    false,
						Choices: []*discordgo.ApplicationCommandOptionChoice{
							{Name: "Team", Value: "team"},
							{Name: "Systems & Architecture", Value: "systems"},
							{Name: "Product & Experiences", Value: "product"},
							{Name: "General", Value: "general"},
						},
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "List all indexed knowledge topics",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "delete",
				Description: "Delete a knowledge entry by ID",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "id",
						Description: "ID of the document to delete (check /kb list for IDs)",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "reload",
				Description: "Reload seed knowledge markdown files from disk",
			},
		},
	}
}

// Handle executes the /kb slash command.
func (c *KBCommand) Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	if c.ragEngine == nil {
		return RespondEphemeral(s, i, "⚠️ RAG engine is not initialized.")
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		return RespondEphemeral(s, i, "⚠️ Please specify a subcommand (`search`, `add`, `list`, `reload`).")
	}

	subCmd := options[0]

	switch subCmd.Name {
	case "search":
		query := subCmd.Options[0].StringValue()
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		results, err := c.ragEngine.Search(ctx, query, 3, 0.45)
		if err != nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("⚠️ Search failed: %v", err),
			})
			return err
		}

		if len(results) == 0 {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("🔍 No relevant knowledge found for `%s`.", query),
			})
			return nil
		}

		var fields []*discordgo.MessageEmbedField
		for _, res := range results {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("[%s] %s (Score: %.2f)", strings.ToUpper(res.Document.Category), res.Document.Title, res.Score),
				Value:  truncate(res.Document.Content, 400),
				Inline: false,
			})
		}

		embed := &discordgo.MessageEmbed{
			Title:     fmt.Sprintf("🧠 Knowledge Base Results for: \"%s\"", query),
			Color:     0x5865F2,
			Fields:    fields,
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    &discordgo.MessageEmbedFooter{Text: "Localoy RAG Engine"},
		}

		_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embed},
		})
		return err

	case "add":
		author := InteractionUser(i).Username
		optMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption)
		for _, opt := range subCmd.Options {
			optMap[opt.Name] = opt
		}

		title := optMap["title"].StringValue()
		content := optMap["content"].StringValue()
		category := "general"
		if opt, ok := optMap["category"]; ok {
			category = opt.StringValue()
		}

		doc := rag.Document{
			Title:     title,
			Content:   content,
			Category:  category,
			Source:    fmt.Sprintf("discord:@%s", author),
			UpdatedAt: time.Now().UTC(),
		}

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		if err := c.ragEngine.AddDocument(ctx, doc); err != nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("⚠️ Failed to index knowledge: %v", err),
			})
			return err
		}

		embed := &discordgo.MessageEmbed{
			Title:       "✅ Knowledge Added & Indexed",
			Description: fmt.Sprintf("**Title:** %s\n**Category:** `%s`\n**Added by:** @%s", title, category, author),
			Color:       0x2ECC71,
			Timestamp:   time.Now().Format(time.RFC3339),
		}

		_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Embeds: []*discordgo.MessageEmbed{embed},
		})
		return err

	case "list":
		docs := c.ragEngine.ListDocuments()
		if len(docs) == 0 {
			return RespondEphemeral(s, i, "📚 Knowledge base is currently empty.")
		}

		var sb strings.Builder
		for idx, doc := range docs {
			sb.WriteString(fmt.Sprintf("**%d. [%s] %s** `(id: %s)`\n", idx+1, strings.ToUpper(doc.Category), doc.Title, doc.ID))
			snippet := truncate(doc.Content, 90)
			sb.WriteString(fmt.Sprintf("   └ *%s*\n", snippet))
		}

		embed := &discordgo.MessageEmbed{
			Title:       fmt.Sprintf("📚 Indexed Knowledge Base (%d entries)", len(docs)),
			Description: sb.String(),
			Color:       0x5865F2,
			Timestamp:   time.Now().Format(time.RFC3339),
		}

		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{embed},
				Flags:  discordgo.MessageFlagsEphemeral,
			},
		})

	case "delete":
		docID := subCmd.Options[0].StringValue()
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})

		doc, exists := c.ragEngine.GetDocument(docID)
		if !exists {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("⚠️ No knowledge document found with ID `%s`.", docID),
			})
			return nil
		}

		if err := c.ragEngine.DeleteDocument(ctx, docID); err != nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("⚠️ Failed to delete document: %v", err),
			})
			return err
		}

		_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: fmt.Sprintf("🗑️ Successfully deleted knowledge document: **%s** `(id: %s)`.", doc.Title, docID),
		})
		return err

	case "reload":
		count, err := c.ragEngine.LoadFromDirectory(ctx, "data/knowledge")
		if err != nil {
			return RespondEphemeral(s, i, fmt.Sprintf("⚠️ Reload failed: %v", err))
		}
		return RespondEphemeral(s, i, fmt.Sprintf("🔄 Successfully reloaded %d documents from `data/knowledge`.", count))
	}

	return nil
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
