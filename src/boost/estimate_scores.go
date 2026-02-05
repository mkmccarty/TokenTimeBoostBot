package boost

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
)

// GetSlashCsEstimates returns the slash command for estimating scores
func GetSlashCsEstimates(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name: cmd,

		Description: "Provide a Contract Score estimates for a running contract",
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
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "contract-id",
				Description:  "Select a contract-id",
				Required:     false,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "coop-id",
				Description: "Your coop-id",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "private-reply",
				Description: "Response visibility, default is public",
				Required:    false,
			},
		},
	}
}

// HandleCsEstimatesCommand handles the estimate scores command
func HandleCsEstimatesCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {

	flags := discordgo.MessageFlagsIsComponentsV2

	var contractID string
	var coopID string
	optionMap := bottools.GetCommandOptionsMap(i)

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = strings.ToLower(opt.StringValue())
		contractID = strings.ReplaceAll(contractID, " ", "")
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = strings.ToLower(opt.StringValue())
		coopID = strings.ReplaceAll(coopID, " ", "")
	}
	if opt, ok := optionMap["private-reply"]; ok {
		if opt.BoolValue() {
			flags |= discordgo.MessageFlagsEphemeral
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	// Unser contractID and coopID means we want the Boost Bot contract
	var foundContractHash = ""
	if contractID == "" || coopID == "" {
		contract := FindContract(i.ChannelID)
		if contract == nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: "No contract found in this channel. Please provide a contract-id and coop-id.",
				})

			return
		}
		foundContractHash = contract.ContractHash
		contractID = contract.ContractID
		coopID = strings.ToLower(contract.CoopID)
	}

	var str string
	str, fields, scores := DownloadCoopStatusTeamwork(contractID, coopID, true)
	if fields == nil || strings.HasSuffix(str, "no such file or directory") || strings.HasPrefix(str, "No grade found") {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Flags:   flags,
			Content: str,
		})
		return
	}

	eiContract := ei.EggIncContractsAll[contractID]
	var footer strings.Builder
	if eiContract.SeasonalScoring == ei.SeasonalScoringStandard {
		footer.WriteString("-# MAX : BTV & Max Chicken Runs & ∆T-Val\n")
		footer.WriteString("-# TVAL: BTV, Coop Size-1 Chicken Runs & ∆T-Val\n")
		footer.WriteString("-# SINK: BTV, Max Chicken Runs & Token Sink\n")
		footer.WriteString("-# RUNS: BTV, Coop Size-1 Chicken Runs, No token sharing\n")
		footer.WriteString("-# MIN:  BTV, No Chicken Runs,No token sharing\n")
		footer.WriteString("-# BASE: No BTV, No Chicken Runs, No token sharing\n")
	} else {
		footer.WriteString("-# MAX : BTV & Max Chicken Runs\n")
		footer.WriteString("-# MIN:  BTV & No Chicken Runs\n")
		footer.WriteString("-# BASE: No BTV, No Chicken Runs\n")

	}

	var components, buttons []discordgo.MessageComponent
	components = append(components,
		discordgo.TextDisplay{
			Content: str,
		},
		discordgo.TextDisplay{
			Content: "## Projected Contract Scores",
		},
		discordgo.TextDisplay{
			Content: scores,
		},
		discordgo.TextDisplay{
			Content: footer.String(),
		},
	)

	// Build buttons if we have a found contract in the current channel
	if foundContractHash != "" {
		buttonConfigs := []struct {
			label  string
			style  discordgo.ButtonStyle
			action string
		}{
			{"Completion Ping", discordgo.PrimaryButton, "completionping"},
			//{"Check-in Ping", discordgo.PrimaryButton, "checkinping"},
			{"Close", discordgo.DangerButton, "close"},
		}

		for _, config := range buttonConfigs {
			buttons = append(buttons, discordgo.Button{
				Label:    config.label,
				Style:    config.style,
				CustomID: fmt.Sprintf("csestimate#%s#%s", config.action, foundContractHash),
			})
		}

		components = append(components, discordgo.ActionsRow{
			Components: buttons,
		})
	}

	// Send the response
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      flags,
		Components: components,
	})
}

// HandleCsEstimateButtons handles button interactions for /cs-estimate
func HandleCsEstimateButtons(s *discordgo.Session, i *discordgo.InteractionCreate) {

	const ttl = 5 * time.Minute
	expired := false
	if i.Message != nil {
		createdAt, err := discordgo.SnowflakeTimestamp(i.Message.ID)
		if err != nil {
			log.Println("Error parsing message timestamp:", err)
			return
		}
		expired = time.Since(createdAt) > ttl
	}

	flags := discordgo.MessageFlagsIsComponentsV2 | discordgo.MessageFlagsEphemeral
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Content:    "",
			Flags:      flags,
			Components: []discordgo.MessageComponent{},
		},
	})
	if err != nil {
		log.Println(err)
	}

	if !expired {
		// Handle the button actions
		action := reaction[1]
		contractHash := reaction[2]
		contract := FindContractByHash(contractHash)
		if contract == nil {
			log.Println("Contract not found for hash:", contractHash)
			return
		}

		// Is the user in the contract?
		userID := getInteractionUserID(i)
		if !userInContract(contract, userID) {
			// Ignore if the user isn't in the contract
			return
		}

		switch action {
		case "completionping":
			go sendCompletionPing(s, i, contract, userID)
		case "checkinping":
			//go SendCheckinPings(s, i, contract)
		default:
			// default to close
		}
	}

	// Remove the buttons regardless of expiration
	var comp []discordgo.MessageComponent
	if len(i.Message.Components) > 0 {
		comp = i.Message.Components[:len(i.Message.Components)-1]
	}
	// Edit the original message to remove buttons
	edit := discordgo.WebhookEdit{
		Components: &comp,
	}
	_, _ = s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit)
	if err != nil {
		log.Println(err)
	}
}

// sendCompletionPing sends a ping with the estimated completion time of the contract
func sendCompletionPing(s *discordgo.Session, i *discordgo.InteractionCreate, contract *Contract, userID string) {

	//Check if the contract is still ongoing
	if time.Now().After(contract.EstimatedEndTime) {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: "The contract has already ended. Ping not sent.",
		})
		return
	}

	// Get the role mention from the active contract
	roleMention := contract.Location[0].RoleMention
	if roleMention == "" {
		roleMention = "@here"
	}

	message :=
		fmt.Sprintf("%s, The contract **%s** `%s` will complete on\n## %s at %s!\nPlease set an alarm or leave your devices on!\n-# Ping requested by %s",
			roleMention,
			contract.Name,
			contract.CoopID,
			bottools.WrapTimestamp(contract.EstimatedEndTime.Unix(), bottools.TimestampLongDate),
			bottools.WrapTimestamp(contract.EstimatedEndTime.Unix(), bottools.TimestampLongTime),
			contract.Boosters[userID].Mention,
		)

	_, err := s.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
		Content: message,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{
				discordgo.AllowedMentionTypeRoles,
				discordgo.AllowedMentionTypeUsers,
			},
		},
	})

	if err != nil {
		log.Println("Error sending completion ping:", err)
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: "Error sending completion ping.",
		})
		return
	}
}
