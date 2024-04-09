package boost

import (
	"errors"
)

// SetWish sets the wish for a contract identified by the guild ID and channel ID.
func SetWish(guildID string, channelID string, wish string) error {
	var contract = FindContract(channelID)
	if contract == nil {
		return errors.New(errorNoContract)
	}

	contract.lastWishPrompt = wish

	return nil
}

// GetWish gets the wish for a contract identified by the guild ID and channel ID.
func GetWish(guildID string, channelID string) string {
	var contract = FindContract(channelID)
	if contract == nil {
		return ""
	}

	return contract.lastWishPrompt
}
