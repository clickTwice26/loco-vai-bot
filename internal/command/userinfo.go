package command

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

// UserInfoCommand handles the /userinfo command.
type UserInfoCommand struct{}

// NewUserInfoCommand creates a new UserInfoCommand instance.
func NewUserInfoCommand() *UserInfoCommand {
	return &UserInfoCommand{}
}

// Definition returns the slash command definition for /userinfo.
func (c *UserInfoCommand) Definition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "userinfo",
		Description: "Display information about a user or yourself",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "target",
				Description: "The user to inspect (defaults to yourself)",
				Required:    false,
			},
		},
	}
}

// Handle executes the /userinfo command.
func (c *UserInfoCommand) Handle(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	var targetUser *discordgo.User
	var targetMember *discordgo.Member

	// Default to interaction author
	if i.Member != nil {
		targetUser = i.Member.User
		targetMember = i.Member
	} else if i.User != nil {
		targetUser = i.User
	}

	// Check if target option was provided
	options := i.ApplicationCommandData().Options
	if len(options) > 0 && options[0].Name == "target" {
		targetUser = options[0].UserValue(s)
		if i.GuildID != "" && targetUser != nil {
			if member, err := s.GuildMember(i.GuildID, targetUser.ID); err == nil {
				targetMember = member
			}
		}
	}

	if targetUser == nil {
		return RespondEphemeral(s, i, "⚠️ Could not resolve target user.")
	}

	avatarURL := targetUser.AvatarURL("256")
	if avatarURL == "" {
		avatarURL = "https://cdn.discordapp.com/embed/avatars/0.png"
	}

	accountCreatedAt, err := discordgo.SnowflakeTimestamp(targetUser.ID)
	createdStr := "Unknown"
	if err == nil {
		createdStr = fmt.Sprintf("<t:%d:F> (<t:%d:R>)", accountCreatedAt.Unix(), accountCreatedAt.Unix())
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "Username",
			Value:  fmt.Sprintf("`%s`", targetUser.Username),
			Inline: true,
		},
		{
			Name:   "User ID",
			Value:  fmt.Sprintf("`%s`", targetUser.ID),
			Inline: true,
		},
		{
			Name:   "Bot Account?",
			Value:  fmt.Sprintf("`%t`", targetUser.Bot),
			Inline: true,
		},
		{
			Name:   "Account Created",
			Value:  createdStr,
			Inline: false,
		},
	}

	if targetMember != nil && targetMember.JoinedAt != (time.Time{}) {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Joined Server",
			Value:  fmt.Sprintf("<t:%d:F> (<t:%d:R>)", targetMember.JoinedAt.Unix(), targetMember.JoinedAt.Unix()),
			Inline: false,
		})

		if len(targetMember.Roles) > 0 {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Roles Count",
				Value:  fmt.Sprintf("`%d` roles", len(targetMember.Roles)),
				Inline: true,
			})
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("User Info - %s", targetUser.Username),
		Color:     0x2ECC71, // Green
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: avatarURL},
		Fields:    fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Requested by %s", InteractionUser(i).Username),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}
