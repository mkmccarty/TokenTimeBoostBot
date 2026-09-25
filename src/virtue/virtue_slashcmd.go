package virtue

import (
	"encoding/base64"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func init() {
	boost.RegisterEggIDModalAction("virtue", func(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
		HandleVirtueModal(e, options, encryptedID, okayToSave)
	})
}

func anywhereCommand(name, description string) dc.Command {
	return dc.Command{
		Name:        name,
		Description: description,
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
	}
}

// GetSlashVirtueCommand returns the command definition for the /virtue command.
func GetSlashVirtueCommand(cmd string) *dc.Command {
	teMin, teMax := 1, 98
	command := anywhereCommand(cmd, "Evaluate virtue farm and provide detailed EoV overview.")
	command.Options = []dc.Option{
		dc.IntOption{
			Name:        "simulate-shift",
			Description: "What does a 0 pop shift look like for this egg?",
			Choices: []dc.Choice[int]{
				{Name: "Curiosity", Value: 50},
				{Name: "Integrity", Value: 51},
				{Name: "Humility", Value: 52},
				{Name: "Resilience", Value: 53},
				{Name: "Kindness", Value: 54},
			},
		},
		dc.IntOption{
			Name:        "simulate-shift-target-te",
			Description: "Target Truth Eggs for simulated shift (requires simulate-shift).",
			MinValue:    &teMin,
			MaxValue:    &teMax,
		},
		dc.BoolOption{
			Name:        "help",
			Description: "Explain what this command reports",
		},
		dc.BoolOption{
			Name:        "reset",
			Description: "Reset stored EI number",
		},
		dc.BoolOption{
			Name:        "compact",
			Description: "Compact display (sticky)",
		},
	}
	return &command
}

// decryptEggIncID decrypts an encrypted Egg Inc ID string using config.Key.
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

// getEggIncID retrieves and decrypts the user's stored Egg Inc ID.
func getEggIncID(userID string) string {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	return decryptEggIncID(eiID)
}

// HandleVirtue handles the /virtue command.
func HandleVirtue(e *dc.CommandEvent) {
	userID := e.UserID()

	if opt, ok := e.OptBool("help"); ok && opt {
		_ = e.Respond(dc.Message{Content: virtueHelpText(), Ephemeral: true})
		return
	}

	if opt, ok := e.OptBool("reset"); ok && opt {
		farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
	}

	eggIncID := getEggIncID(userID)
	if eggIncID == "" {
		boost.RequestEggIncIDModal(e, "virtue", e.Options())
		return
	}

	ExecuteVirtue(e, e.Options(), eggIncID, true)
}

// HandleVirtueModal handles the modal submission when a user provides their Egg Inc ID.
func HandleVirtueModal(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
	eggIncID := decryptEggIncID(encryptedID)
	if eggIncID == "" {
		_ = e.Respond(dc.Message{
			Content:   "Invalid Egg Inc ID received.",
			Ephemeral: true,
		})
		return
	}

	ExecuteVirtue(e, options, eggIncID, okayToSave)
}

func virtueHelpText() string {
	return strings.TrimSpace(`
# /virtue Help

The /virtue command evaluates your Eggs of Virtue home farm and shows shift planning details.

## What the output shows
- Your virtue title line (Ascender or Prestiged One)
- Current reset count and shift count
- Current shift cost in Soul Eggs
- Egg-by-egg status for Curiosity, Integrity, Humility, Resilience, and Kindness
- Progress and estimates used to plan your next shift
- Fleet, habitat, train, and other farm context needed for decision making
- Notes for current farm state and possible simulated outcomes

## Options
- help (boolean): Show this explanation instead of running /virtue.
- simulate-shift (Curiosity/Integrity/Humility/Resilience/Kindness): Simulate a 0-pop shift for the selected egg.
- simulate-shift-target-te (1-98): Optional target Truth Eggs used with simulate-shift.
- compact (boolean): Toggle compact output mode. This is sticky and remembered for future runs.
- reset (boolean): Clear your stored Egg Inc ID so you can set it again.

## Notes
- /virtue uses your stored Egg Inc ID.
- If your ID is missing or invalid, the bot asks you to provide it.
- Best results come from being on an Egg of Virtue home farm.
`)
}
