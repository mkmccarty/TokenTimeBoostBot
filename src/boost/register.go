package boost

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashRegisterCommand returns the /register command
func GetSlashRegisterCommand(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Register your Egg Inc ID with Boost Bot.")
	command.Options = []dc.Option{
		dc.BoolOption{
			Name:        "reset",
			Description: "Reset stored EI number",
		},
	}
	return &command
}

// GetSlashRegisterAltCommand returns the /register-alt command
func GetSlashRegisterAltCommand(cmd string) *dc.Command {
	command := anywhereCommand(cmd, "Register an alternate Egg Inc ID with Boost Bot.")
	command.Options = []dc.Option{
		dc.StringOption{
			Name:         "name",
			Description:  "The name of the alternate account or 'new' for a new one.",
			Required:     true,
			Autocomplete: true,
		},
	}
	return &command
}

// HandleRegisterAlt asks for the alternate's Egg Inc ID.
func HandleRegisterAlt(e *dc.CommandEvent) {
	targetAlt := ""
	if opt, ok := e.OptString("name"); ok {
		targetAlt = opt
	}
	RequestEggIncIDModal(e, "register-alt#"+targetAlt, e.Options())
}

// HandleRegisterAltAutocomplete suggests the alternates the caller controls.
func HandleRegisterAltAutocomplete(e *dc.AutocompleteEvent) {
	alts := farmerstate.GetAltControllerByMiscString("AltController", e.UserID())
	choices := []dc.Choice[string]{
		{Name: "New Alternate", Value: "new"},
	}
	for _, alt := range alts {
		choices = append(choices, dc.Choice[string]{
			Name:  ei.NormalizePlayerNameForDisplay(alt),
			Value: alt,
		})
	}

	_ = e.RespondChoices(choices)
}

// HandleRegister asks for the caller's Egg Inc ID.
func HandleRegister(e *dc.CommandEvent) {
	if opt, ok := e.OptBool("reset"); ok && opt {
		farmerstate.SetMiscSettingString(e.UserID(), "encrypted_ei_id", "")
	}
	RequestEggIncIDModal(e, "register", e.Options())
}

// Register processes the register modal submission, saves the EI ID, and pulls a backup to update the player's IGN.
func Register(e *dc.ModalEvent, encryptedID string, okayToSave bool) {
	userID := e.UserID()

	_ = e.Defer(true)

	eggIncID := ""
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(encryptedID)
		if err == nil {
			decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}

	var str string
	if eggIncID == "" {
		str = "Your Egg Inc ID could not be saved."
	} else {
		backup, _ := ei.GetFirstContactFromAPI(eggIncID, userID, okayToSave)
		if backup == nil {
			str = "Your Egg Inc ID was saved but the backup could not be retrieved from EI."
		} else {
			farmerName := farmerstate.GetMiscSettingString(userID, "ei_ign")
			newName := backup.GetUserName()
			displayName := ei.NormalizePlayerNameForDisplay(newName)
			farmerstate.SetMiscSettingString(userID, "ei_ign", newName)
			te := ei.GetCurrentTruthEggs(backup)
			farmerstate.SetMiscSettingString(userID, "TE", fmt.Sprintf("%d", te))
			artifacts := ei.GetBestCoopArtifactsFromInventory(backup.GetArtifactsDb().GetInventoryItems())
			for key, val := range artifacts {
				farmerstate.SetMiscSettingString(userID, key, val)
			}
			// Set all collectibles to "owned" by default
			if farmerstate.GetMiscSettingString(userID, "collegg") == "" {
				var eggNames []string
				for _, egg := range ei.CustomEggMap {
					eggNames = append(eggNames, egg.Name)
				}
				farmerstate.SetMiscSettingString(userID, "collegg", strings.Join(eggNames, ","))
			}
			str = "Your Egg Inc ID has been registered as " + displayName + fmt.Sprintf(" (TE: %d).", te)
			if farmerName != "" && farmerName != newName {
				str += " (Previously: " + ei.NormalizePlayerNameForDisplay(farmerName) + ")"
			}
			// Respond to register command
			_ = e.Followup(dc.Message{Content: str, Ephemeral: true})
			// Spawn /artifacts command response
			artStr, artComponents := getArtifactsComponents(userID, "", false, "delivery", "")
			artStr = "## Check your collectibles\n" + artStr
			_ = e.Followup(dc.Message{
				Content:      artStr,
				Components:   artComponents,
				Ephemeral:    true,
				ComponentsV1: true,
			})
			return
		}
	}

	_ = e.Followup(dc.Message{Content: str, Ephemeral: true})
}

// RegisterAlt processes the register-alt modal submission.
func RegisterAlt(e *dc.ModalEvent, targetAlt string, encryptedID string) {
	parentUserID := e.UserID()

	_ = e.Defer(true)

	var str string
	if targetAlt != "new" && encryptedID == "" {
		// Clear existing alt
		farmerstate.SetMiscSettingString(targetAlt, "encrypted_ei_id", "")
		str = fmt.Sprintf("Egg Inc ID for alternate %s has been cleared.", targetAlt)
		_ = e.Followup(dc.Message{Content: str, Ephemeral: true})
		return
	}

	if encryptedID == "" {
		str = "You must provide a valid Egg Inc ID for a new alternate."
		_ = e.Followup(dc.Message{Content: str, Ephemeral: true})
		return
	}

	eggIncID := ""
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(encryptedID)
		if err == nil {
			decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}

	if eggIncID == "" {
		str = "Your Egg Inc ID could not be saved."
	} else {
		backup, _ := ei.GetFirstContactFromAPI(eggIncID, parentUserID, true)
		if backup == nil {
			str = "Your Egg Inc ID was saved but the backup could not be retrieved from EI."
		} else {
			newName := backup.GetUserName()
			displayName := ei.NormalizePlayerNameForDisplay(newName)
			altID := targetAlt
			if altID == "new" {
				altID = newName
			}
			// Use altID as the ID for the alt record
			farmerstate.SetMiscSettingString(altID, "ei_ign", newName)
			farmerstate.SetMiscSettingString(altID, "encrypted_ei_id", encryptedID)
			farmerstate.SetMiscSettingString(altID, "AltController", parentUserID)

			te := ei.GetCurrentTruthEggs(backup)
			farmerstate.SetMiscSettingString(altID, "TE", fmt.Sprintf("%d", te))
			artifacts := ei.GetBestCoopArtifactsFromInventory(backup.GetArtifactsDb().GetInventoryItems())
			for key, val := range artifacts {
				farmerstate.SetMiscSettingString(altID, key, val)
			}
			str = "Your alternate Egg Inc ID has been registered as " + displayName + fmt.Sprintf(" (TE: %d).", te)
		}
	}

	_ = e.Followup(dc.Message{Content: str, Ephemeral: true})
}
