package bottools

import "github.com/bwmarrin/discordgo"

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
