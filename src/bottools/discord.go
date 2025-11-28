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
