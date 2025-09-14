package boost

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

// GetSlashReplayEvalCommand returns the command for the /launch-helper command
func GetSlashReplayEvalCommand(cmd string) *discordgo.ApplicationCommand {
	minValue := 0.0
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Evaluate contract history and provide replay guidance.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "threshold",
				Description: "Below % of speedrun score",
				MinValue:    &minValue,
				MaxValue:    50,
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "reset",
				Description: "Reset stored EI number",
				Required:    false,
			},
		},
	}
}

// HandleReplayEval handles the /replay-eval command
func HandleReplayEval(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := bottools.GetInteractionUserID(i)
	percent := -1

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	if opt, ok := optionMap["reset"]; ok {
		if opt.BoolValue() {
			farmerstate.SetMiscSettingString(userID, "encrypted_ei_id", "")
		}
	}
	if opt, ok := optionMap["threshold"]; ok {
		percent = int(opt.UintValue())
	}

	eggIncID := ""
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	encryptionKey, err := base64.StdEncoding.DecodeString(config.Key)
	if err == nil {
		decodedData, err := base64.StdEncoding.DecodeString(eiID)
		if err == nil {
			decryptedData, err := config.DecryptCombined(encryptionKey, decodedData)
			if err == nil {
				eggIncID = string(decryptedData)
			}
		}
	}

	if eggIncID == "" || len(eggIncID) != 18 || eggIncID[:2] != "EI" {
		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "m_replay#" + userID,
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
		return
	}

	// Do the work
	flags := discordgo.MessageFlagsIsComponentsV2
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	archive := ei.GetContractArchiveFromAPI(s, eggIncID, userID)

	cxpVersion := ""
	for _, c := range archive {
		eval := c.GetEvaluation()
		if eval != nil {
			cxpVersion = eval.GetVersion()
			// Replace all non-numeric characters in cxpVersion with underscores
			cxpVersion = strings.Map(func(r rune) rune {
				if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
					return r
				}
				return '_'
			}, cxpVersion)
			break
		}
	}

	// I want to convert this archive to a JSON string, replace the eggIncID with the discord ID and save the file to
	// a local file named contract-archive-<discordID>.json for debugging purposes.

	jsonData, err := json.Marshal(archive)
	if err != nil {
		log.Println("Error marshalling archive to JSON:", err)
		return
	}

	discordID := userID
	fileName := fmt.Sprintf("ttbb-data/eiuserdata/archive-%s-%s.json", discordID, cxpVersion)
	err = os.WriteFile(fileName, jsonData, 0644)
	if err != nil {
		log.Println("Error saving contract archive to file:", err)
		return
	}

	str := printArchivedContracts(archive, percent)
	if str == "" {
		str = "No archived contracts found in Egg Inc API response"
	}
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags: flags,
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: str},
		},
	})

}

// HandleReplayModalSubmit handles the modal submission for the /replay-eval command
func HandleReplayModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

func printArchivedContracts(archive []*ei.LocalContract, percent int) string {
	builder := strings.Builder{}
	if archive == nil {
		log.Print("No archived contracts found in Egg Inc API response")
		return builder.String()
	}
	log.Printf("Downloaded %d archived contracts from Egg Inc API\n", len(archive))

	// Want a preamble string for builder for what we're displaying
	if percent == -1 {
		builder.WriteString("## Contract CS eval of active contracts\n")
	} else {
		builder.WriteString(fmt.Sprintf("## Displaying contract scores less than %d%% of speedrun potential:\n", percent))
	}
	fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s`\n",
		bottools.AlignString("CONTRACT-ID", 30, bottools.StringAlignCenter),
		bottools.AlignString("CS", 6, bottools.StringAlignCenter),
		bottools.AlignString("HIGH", 6, bottools.StringAlignCenter),
		bottools.AlignString("GAP", 6, bottools.StringAlignRight),
		bottools.AlignString("%", 4, bottools.StringAlignCenter),
	)

	count := 0

	for _, c := range archive {

		contractID := c.GetContract().GetIdentifier()
		//coopID := c.GetCoopIdentifier()
		evaluation := c.GetEvaluation()
		cxp := evaluation.GetCxp()

		c := ei.EggIncContractsAll[contractID]
		//if c.ContractVersion == 2 {
		if percent != -1 {
			if cxp < c.Cxp*(1-float64(percent)/100) || c.Cxp == 0 {
				if builder.Len() < 3500 {
					fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s`\n",
						bottools.AlignString(contractID, 30, bottools.StringAlignLeft),
						bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(cxp))), 6, bottools.StringAlignRight),
						bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.Cxp))), 6, bottools.StringAlignRight),
						bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.Cxp-cxp))), 6, bottools.StringAlignRight),
						bottools.AlignString(fmt.Sprintf("%.1f", (cxp/c.Cxp)*100), 4, bottools.StringAlignCenter))
				}
				count++
			}
		} else {
			if c.ContractVersion == 2 && c.ExpirationTime.Unix() > time.Now().Unix() {
				if builder.Len() < 3500 {
					fmt.Fprintf(&builder, "`%12s %6s %6s %6s %6s` <t:%d:R>\n",
						bottools.AlignString(contractID, 30, bottools.StringAlignLeft),
						bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(cxp))), 6, bottools.StringAlignRight),
						bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.Cxp))), 6, bottools.StringAlignRight),
						bottools.AlignString(fmt.Sprintf("%d", int(math.Ceil(c.Cxp-cxp))), 6, bottools.StringAlignRight),
						bottools.AlignString(fmt.Sprintf("%.1f", (cxp/c.Cxp)*100), 4, bottools.StringAlignCenter),
						c.ExpirationTime.Unix())
				}
				count++
			}
		}
		//}
	}
	if builder.Len() > 3500 {
		builder.WriteString("...output truncated...\n")
	}
	if percent != -1 {
		builder.WriteString(fmt.Sprintf("%d of %d contracts met this condition.\n", count, len(archive)))
	}
	if count == 0 {
		builder.Reset()
		builder.WriteString("No contracts met this condition.\n")
	}
	return builder.String()
}
