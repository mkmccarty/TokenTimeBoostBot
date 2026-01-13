package bottools

import (
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
)

// GetInteractionUserID returns the user ID from an interaction, whether in a guild or DM
func GetInteractionUserID(i *discordgo.InteractionCreate) string {
	if i.GuildID == "" {
		return i.User.ID
	}
	return i.Member.User.ID
}

// NewSmallSeparatorComponent returns a Discord separator component configured with
// small spacing and optional visibility.
func NewSmallSeparatorComponent(visible bool) *discordgo.Separator {
	divider := visible
	spacing := discordgo.SeparatorSpacingSizeSmall

	return &discordgo.Separator{
		Divider: &divider,
		Spacing: &spacing,
	}
}

// FindCategoryID walks up the parent chain until it finds the category.
// Returns the category ID, or "" if err.
func FindCategoryID(s *discordgo.Session, channelID string) (string, error) {
	ch, err := getChannel(s, channelID)
	if err != nil {
		return "", err
	}
	// Is a category.
	if ch.Type == discordgo.ChannelTypeGuildCategory {
		return ch.ID, nil
	}
	// No parent found.
	if ch.ParentID == "" {
		return "", nil
	}

	// Recurse up the parent chain.
	return FindCategoryID(s, ch.ParentID)
}

// getChannel retrieves a channel from State first, then falls back to Channel directly.
func getChannel(s *discordgo.Session, id string) (*discordgo.Channel, error) {
	if ch, err := s.State.Channel(id); err == nil {
		return ch, nil
	}
	return s.Channel(id)
}

// IsValidDiscordID checks if a string is a valid Discord snowflake ID by validating
// the embedded timestamp is within Discord's operational range
func IsValidDiscordID(id string) bool {
	// Discord IDs are 17-20 digit numbers
	if len(id) < 17 || len(id) > 20 {
		return false
	}

	// Parse as int64
	snowflake, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return false
	}

	// Extract timestamp: top 42 bits, milliseconds since Discord epoch (Jan 1, 2015)
	const discordEpoch int64 = 1420070400000
	timestamp := (snowflake >> 22) + discordEpoch

	// Validate timestamp is after Discord's launch and not too far in future
	now := time.Now().UnixMilli()
	tenYearsFromNow := now + (10 * 365 * 24 * 60 * 60 * 1000)

	return timestamp >= discordEpoch && timestamp <= tenYearsFromNow
}
