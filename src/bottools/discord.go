package bottools

import "github.com/bwmarrin/discordgo"

// GetInteractionUserID returns the user ID from an interaction, whether in a guild or DM
func GetInteractionUserID(i *discordgo.InteractionCreate) string {
	if i.GuildID == "" {
		return i.User.ID
	}
	return i.Member.User.ID
}
