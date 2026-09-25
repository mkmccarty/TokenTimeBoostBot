package eb

import (
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/boost"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

const (
	FarmHome            = "Home"
	FarmVirtue          = "Virtue"
	FarmHomeAndVirtue   = "Home & Virtue"
	DefaultFarmChoice   = FarmHomeAndVirtue
	StickySettingEbFarm = "eb-farm"
)

func init() {
	boost.RegisterEggIDModalAction("eb", func(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
		HandleEbModal(e, options, encryptedID, okayToSave)
	})
}

// GetSlashEbCommand returns the slash command definition for /eb.
func GetSlashEbCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Show player's Earnings Bonus, roles, and farm details.",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
		Options: []dc.Option{
			dc.StringOption{
				Name:        "farm",
				Description: "Farm view: Home, Virtue, or Home & Virtue. Default is Home & Virtue. (sticky)",
				Required:    false,
				Choices: []dc.Choice[string]{
					{Name: FarmHomeAndVirtue, Value: FarmHomeAndVirtue},
					{Name: FarmHome, Value: FarmHome},
					{Name: FarmVirtue, Value: FarmVirtue},
				},
			},
		},
	}
	return &command
}

// NormalizeFarmChoice cleans and standardizes a user-supplied farm choice.
func NormalizeFarmChoice(choice string) string {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "home":
		return FarmHome
	case "virtue":
		return FarmVirtue
	case "home & virtue", "home and virtue", "both", "all":
		return FarmHomeAndVirtue
	default:
		return DefaultFarmChoice
	}
}

// HandleEb handles the /eb command.
func HandleEb(e *dc.CommandEvent) {
	userID := e.UserID()

	farmChoice := DefaultFarmChoice
	if opt, ok := e.OptString("farm"); ok && strings.TrimSpace(opt) != "" {
		farmChoice = NormalizeFarmChoice(opt)
		farmerstate.SetMiscSettingString(userID, StickySettingEbFarm, farmChoice)
	} else {
		saved := farmerstate.GetMiscSettingString(userID, StickySettingEbFarm)
		if saved != "" {
			farmChoice = NormalizeFarmChoice(saved)
		}
	}

	eggIncID := getEggIncID(userID)
	if eggIncID == "" {
		boost.RequestEggIncIDModal(e, "eb", e.Options())
		return
	}

	ExecuteEb(e, farmChoice, eggIncID, true)
}

// HandleEbModal handles the modal submission when a user provides their Egg Inc ID.
func HandleEbModal(e *dc.ModalEvent, options dc.OptionValues, encryptedID string, okayToSave bool) {
	userID := e.UserID()

	farmChoice := DefaultFarmChoice
	if opt, ok := options.String("farm"); ok && strings.TrimSpace(opt) != "" {
		farmChoice = NormalizeFarmChoice(opt)
		farmerstate.SetMiscSettingString(userID, StickySettingEbFarm, farmChoice)
	} else {
		saved := farmerstate.GetMiscSettingString(userID, StickySettingEbFarm)
		if saved != "" {
			farmChoice = NormalizeFarmChoice(saved)
		}
	}

	eggIncID := decryptEggIncID(encryptedID)
	if eggIncID == "" {
		_ = e.Respond(dc.Message{
			Content:   "Invalid Egg Inc ID received.",
			Ephemeral: true,
		})
		return
	}

	ExecuteEb(e, farmChoice, eggIncID, okayToSave)
}
