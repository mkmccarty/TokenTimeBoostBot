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

// RequestEggIncIDModal sends a modal to the user requesting their Egg Inc ID
func RequestEggIncIDModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: "m_eggid#" + userID,
			Title:    "BoostBot needs your Egg Inc ID",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    "egginc-id",
							Label:       "Egg Inc game ID (El+16 numbers) *",
							Style:       discordgo.TextInputShort,
							Placeholder: "EI0000000000000000",
							MaxLength:   18,
							Required:    true,
						},
					}},
			}}})
	if err != nil {
		log.Println(err.Error())
	}
}

// HandleEggIDModalSubmit handles the modal submission for an egginc ID
func HandleEggIDModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "That's not a valid Egg Inc ID. It should start with EI followed by 16 numbers."
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
					farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", base64.StdEncoding.EncodeToString(combinedData))
					str = "Egg Inc ID saved.\nRerun the command to evaluate your contract history."
				}
			}
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: str,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
