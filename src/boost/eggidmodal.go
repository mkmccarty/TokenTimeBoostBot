package boost

import (
	"encoding/base64"
	"log"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

var (
	optionMapCache      = make(map[string]map[string]*discordgo.ApplicationCommandInteractionDataOption)
	optionMapCacheMutex sync.Mutex
)

func boolPtr(v bool) *bool {
	return &v
}

// RequestEggIncIDModal sends a modal to the user requesting their Egg Inc ID
func RequestEggIncIDModal(s *discordgo.Session, i *discordgo.InteractionCreate, action string, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	userID := bottools.GetInteractionUserID(i)
	optionMapCacheMutex.Lock()
	optionMapCache[userID] = optionMap
	optionMapCacheMutex.Unlock()

	var components []discordgo.MessageComponent

	components = append(components, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.TextInput{
				CustomID:    "egginc-id",
				Label:       "Egg Inc ID (EI+16 digits)",
				Style:       discordgo.TextInputShort,
				Placeholder: "EI0000000000000000",
				MaxLength:   18,
				Required:    boolPtr(true),
			},
		},
	})

	if action != "register" {
		components = append(components, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.TextInput{
					CustomID:    "confirm",
					Label:       "Save or Forget this ID after this session?",
					Style:       discordgo.TextInputShort,
					Placeholder: "save or forget",
					Value:       "save",
					MaxLength:   6,
					Required:    boolPtr(true),
				},
			},
		})
	}

	title := "BoostBot needs your Egg Inc ID"
	parts := strings.Split(action, "#")
	if parts[0] == "register-alt" && len(parts) > 1 {
		name := parts[1]
		if name == "new" {
			title = "Register New Alternate"
		} else {
			title = "Register Alternate: " + name
		}
	} else if parts[0] == "register" {
		title = "Register Primary Account"
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID:   "m_eggid#" + action,
			Title:      title,
			Components: components,
		},
	})
	if err != nil {
		log.Println(err.Error())
	}
}

// HandleEggIDModalSubmit handles the modal submission for an egginc ID
func HandleEggIDModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "That's not a valid Egg Inc ID. It should start with EI followed by 16 numbers."
	encryptedID := ""
	okayToSave := false
	targetAlt := "new"
	userID := bottools.GetInteractionUserID(i)
	modalData := i.ModalSubmitData()
	for _, row := range modalData.Components {
		for _, comp := range row.(*discordgo.ActionsRow).Components {
			switch input := comp.(type) {
			case *discordgo.TextInput:
				if input.CustomID == "egginc-id" && input.Value != "" {
					eggIncID := strings.TrimSpace(input.Value)
					if len(eggIncID) == 18 && eggIncID[:2] == "EI" && utf8.ValidString(eggIncID) {
						encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
						if err == nil {
							combinedData, err := config.EncryptAndCombine(encryptionKey, []byte(eggIncID))
							if err == nil {
								encryptedID = base64.StdEncoding.EncodeToString(combinedData)
								str = "Egg Inc ID saved.\nRerun the command to evaluate your contract history."
							}
						}
					}
				}
				if input.CustomID == "confirm" && strings.ToLower(strings.TrimSpace(input.Value)) == "save" {
					okayToSave = true
				}
			case *discordgo.SelectMenu:
				if input.CustomID == "target-alt" {
					if len(input.Values) > 0 {
						targetAlt = input.Values[0]
					}
				}
			}
		}
	}

	parts := strings.Split(modalData.CustomID, "#")
	if parts[1] == "register" || parts[1] == "register-alt" {
		okayToSave = true
	} else {
		if !okayToSave {
			str += "\nI will forget your Egg Inc ID after this session."
		}
	}

	if okayToSave && encryptedID != "" {
		if parts[1] != "register-alt" {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", encryptedID)
		}
		str += "\nI will remember your Egg Inc ID for future sessions."
	}

	if parts[1] == "register-alt" && len(parts) > 2 {
		targetAlt = parts[2]
	}

	optionMapCacheMutex.Lock()
	optionMap := optionMapCache[userID]
	delete(optionMapCache, userID)
	optionMapCacheMutex.Unlock()

	switch parts[1] {
	case "register":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to register."
			break
		}
		Register(s, i, encryptedID, okayToSave)
		return
	case "register-alt":
		RegisterAlt(s, i, targetAlt, encryptedID)
		return
	case "replay":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to proceed."
			break
		}
		RerunEval(s, i, optionMap, encryptedID, okayToSave)
		return
	case "virtue":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to proceed."
			break
		}
		Virtue(s, i, optionMap, encryptedID, okayToSave)
		return
	case "contract-report":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to proceed."
			break
		}
		err := ContractReport(s, i, optionMap, encryptedID, okayToSave)
		// This should not happen, but just in case
		if err != nil {
			log.Println("Error in ContractReport after EggID modal:", err)
		}
		return
	default:
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
