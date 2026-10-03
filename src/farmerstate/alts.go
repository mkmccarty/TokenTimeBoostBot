package farmerstate

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// AltAccount holds identification and display info for an alternate account.
type AltAccount struct {
	ID          string
	IGN         string
	DisplayName string
	EggIncID    string
}

// IsDiscordSnowflake reports whether s looks like a Discord snowflake ID (17-20 digits).
func IsDiscordSnowflake(s string) bool {
	if len(s) < 17 || len(s) > 20 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func decryptEggIncID(eiID string) string {
	if eiID == "" {
		return ""
	}
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		return ""
	}
	decodedData, err := base64.StdEncoding.DecodeString(eiID)
	if err != nil {
		return ""
	}
	decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
	if err != nil {
		return ""
	}
	id := string(decryptedData)
	if len(id) == 18 && id[:2] == "EI" {
		return id
	}
	return ""
}

// GetUserEggIncID returns the decrypted Egg Inc ID for a user or alt.
func GetUserEggIncID(userID string) string {
	eiID := GetMiscSettingString(userID, "encrypted_ei_id")
	return decryptEggIncID(eiID)
}

// FindAltDiscordID returns the registered Discord user ID for an alt, if any.
func FindAltDiscordID(altID string) string {
	altID = strings.TrimSpace(altID)
	if altID == "" {
		return ""
	}
	if IsDiscordSnowflake(altID) {
		return altID
	}
	if dID := strings.TrimSpace(GetMiscSettingString(altID, "discord_id")); IsDiscordSnowflake(dID) {
		return dID
	}
	if dID := strings.TrimSpace(GetMiscSettingString(altID, "DiscordID")); IsDiscordSnowflake(dID) {
		return dID
	}
	if dID, err := GetDiscordUserIDFromEiIgnExact(altID); err == nil && IsDiscordSnowflake(dID) {
		return dID
	}
	if ign := strings.TrimSpace(GetMiscSettingString(altID, "ei_ign")); ign != "" {
		if dID, err := GetDiscordUserIDFromEiIgnExact(ign); err == nil && IsDiscordSnowflake(dID) {
			return dID
		}
	}
	return ""
}

// GetUserAltsWithSavedEID returns all registered alternate accounts for userID that have a saved Egg Inc ID.
func GetUserAltsWithSavedEID(userID string) []AltAccount {
	altIDs := GetAltControllerByMiscString("AltController", userID)
	var result []AltAccount
	seen := make(map[string]bool)
	for _, altID := range altIDs {
		altID = strings.TrimSpace(altID)
		if altID == "" || seen[altID] {
			continue
		}
		seen[altID] = true
		eid := GetUserEggIncID(altID)
		if eid == "" {
			continue
		}
		ign := strings.TrimSpace(GetMiscSettingString(altID, "ei_ign"))
		displayName := altID
		if IsDiscordSnowflake(altID) {
			if ign != "" {
				displayName = ei.NormalizePlayerNameForDisplay(ign)
			}
		} else {
			if ign != "" && !strings.EqualFold(altID, ign) {
				displayName = fmt.Sprintf("%s (%s)", altID, ei.NormalizePlayerNameForDisplay(ign))
			} else if ign != "" {
				displayName = ei.NormalizePlayerNameForDisplay(ign)
			}
		}
		result = append(result, AltAccount{
			ID:          altID,
			IGN:         ign,
			DisplayName: displayName,
			EggIncID:    eid,
		})
	}
	return result
}

// ResolveAltSelection determines which user ID and Egg Inc ID to display, along with any informational message.
func ResolveAltSelection(userID string, altParam string, cmdName string) (targetID string, eggIncID string, notice string) {
	altParam = strings.TrimSpace(altParam)
	if altParam == "" {
		return userID, "", ""
	}

	allAlts := GetAltControllerByMiscString("AltController", userID)
	alts := GetUserAltsWithSavedEID(userID)

	if len(alts) == 0 {
		if len(allAlts) == 0 {
			return userID, "", fmt.Sprintf("You have no registered alternate accounts. Showing your %s instead.", cmdName)
		}
		return userID, "", fmt.Sprintf("You have no registered alternate accounts with a saved Egg Inc ID. Showing your %s instead.", cmdName)
	}

	for _, alt := range alts {
		if strings.EqualFold(alt.ID, altParam) ||
			strings.EqualFold(alt.IGN, altParam) ||
			strings.EqualFold(alt.DisplayName, altParam) {
			return alt.ID, alt.EggIncID, ""
		}
	}

	return userID, "", fmt.Sprintf("Alternate account \"%s\" not found with a saved Egg Inc ID. Showing your %s instead.", altParam, cmdName)
}
