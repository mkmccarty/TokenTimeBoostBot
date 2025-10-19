package boost

import (
	"encoding/base64"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

var optionMapCache = make(map[string]map[string]*discordgo.ApplicationCommandInteractionDataOption)

// I want a map from a string to map[string]*discordgo.ApplicationCommandInteractionDataOption
//optionMapCache := make(map[string]map[string]*discordgo.ApplicationCommandInteractionDataOption)

// RequestEggIncIDModal sends a modal to the user requesting their Egg Inc ID
func RequestEggIncIDModal(s *discordgo.Session, i *discordgo.InteractionCreate, action string, optionMap map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	userID := bottools.GetInteractionUserID(i)
	optionMapCache[userID] = optionMap

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "m_eggid#" + action,
			Title:    "BoostBot needs your Egg Inc ID",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "egginc-id",
							Label:       "Egg Inc ID (EI+16 digits)",
							Style:       discordgo.TextInputShort,
							Placeholder: "EI0000000000000000",
							MaxLength:   18,
							Required:    true,
						},
					}},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "confirm",
							Label:       "Save or Forget this ID after this session?",
							Style:       discordgo.TextInputShort,
							Placeholder: "forget or save",
							Value:       "forget",
							MaxLength:   6,
							Required:    true,
						},
					},
				},
			}}})
	if err != nil {
		log.Println(err.Error())
	}
}

// HandleEggIDModalSubmit handles the modal submission for an egginc ID
func HandleEggIDModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "That's not a valid Egg Inc ID. It should start with EI followed by 16 numbers."
	encryptedID := ""
	okayToSave := false
	userID := bottools.GetInteractionUserID(i)
	modalData := i.ModalSubmitData()
	for _, comp := range modalData.Components {
		input := comp.(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput)
		if input.CustomID == "egginc-id" && input.Value != "" {
			eggIncID := strings.TrimSpace(input.Value)
			if len(eggIncID) != 18 || eggIncID[:2] != "EI" || !utf8.ValidString(eggIncID) {
				break
			}
			encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
			if err == nil {
				combinedData, err := config.EncryptAndCombine(encryptionKey, []byte(eggIncID))
				if err == nil {
					encryptedID = base64.StdEncoding.EncodeToString(combinedData)
					str = "Egg Inc ID saved.\nRerun the command to evaluate your contract history."
				}
			}
		}
		if input.CustomID == "confirm" && input.Value != "" {
			confirm := strings.ToLower(strings.TrimSpace(input.Value))
			if confirm == "save" {
				farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", encryptedID)
				str += "\nI will remember your Egg Inc ID for future sessions."
				okayToSave = true
			} else {
				str += "\nI will forget your Egg Inc ID after this session."
			}
		}
	}

	optionMap := optionMapCache[userID]
	delete(optionMapCache, userID)

	parts := strings.Split(modalData.CustomID, "#")
	switch parts[1] {
	case "replay":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to proceed."
			break
		}
		ReplayEval(s, i, optionMap, encryptedID, okayToSave)
		return
	case "virtue":
		if encryptedID == "" {
			str = "You must provide a valid Egg Inc ID to proceed."
			break
		}
		Virtue(s, i, optionMap, encryptedID, okayToSave)
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
