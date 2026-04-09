package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

type lobbyParams struct {
	contractID string
	coopID     string
	flags      discordgo.MessageFlags
}

// GetSlashLobbyCommand returns the slash command for showing the current coop lobby.
func GetSlashLobbyCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Show the current contract lobby roster",
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

func parseLobbyParams(i *discordgo.InteractionCreate) lobbyParams {
	flags := discordgo.MessageFlagsIsComponentsV2

	var (
		contractID string
		coopID     string
	)

	optionMap := bottools.GetCommandOptionsMap(i)

	if opt, ok := optionMap["contract-id"]; ok {
		contractID = strings.ToLower(strings.ReplaceAll(opt.StringValue(), " ", ""))
	}
	if opt, ok := optionMap["coop-id"]; ok {
		coopID = strings.ToLower(strings.ReplaceAll(opt.StringValue(), " ", ""))
	}
	if opt, ok := optionMap["private-reply"]; ok && opt.BoolValue() {
		flags |= discordgo.MessageFlagsEphemeral
	}

	return lobbyParams{
		contractID: contractID,
		coopID:     coopID,
		flags:      flags,
	}
}

// HandleLobbyCommand handles the /lobby slash command.
func HandleLobbyCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !CheckCoopStatusPermission(s, i, ei.CoopStatusFixEnabled != nil && ei.CoopStatusFixEnabled()) {
		return
	}

	p := parseLobbyParams(i)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   p.flags,
		},
	})

	contractID, coopID, errMsg := resolveLobbyRequest(i, p.contractID, p.coopID)
	if errMsg != "" {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Flags: p.flags | discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{
				discordgo.TextDisplay{Content: errMsg},
			},
		})
		return
	}

	userID := bottools.GetInteractionUserID(i)
	components := buildLobbyComponents(contractID, coopID, userID, false, true)
	_, sendErr := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags:      p.flags,
		Components: components,
	})
	if sendErr != nil {
		log.Println("lobby FollowupMessageCreate:", sendErr)
	}
}

// HandleLobbyButtons handles refresh and close button interactions for /lobby.
func HandleLobbyButtons(s *discordgo.Session, i *discordgo.InteractionCreate) {
	parts := strings.Split(i.MessageComponentData().CustomID, "#")
	if len(parts) < 2 {
		return
	}

	action := parts[1]
	flags := discordgo.MessageFlagsIsComponentsV2
	if i.Message != nil && i.Message.Flags&discordgo.MessageFlagsEphemeral != 0 {
		flags |= discordgo.MessageFlagsEphemeral
	}

	switch action {
	case "close":
		var kept []discordgo.MessageComponent
		for _, c := range i.Message.Components {
			if _, ok := c.(*discordgo.ActionsRow); !ok {
				kept = append(kept, c)
			}
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Flags:      flags,
				Components: kept,
			},
		})

	case "refresh":
		if len(parts) < 4 {
			return
		}
		contractID := parts[2]
		coopID := parts[3]

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
			Data: &discordgo.InteractionResponseData{Flags: flags},
		})

		userID := bottools.GetInteractionUserID(i)
		components := buildLobbyComponents(contractID, coopID, userID, true, true)
		edit := discordgo.WebhookEdit{Components: &components}
		if _, err := s.FollowupMessageEdit(i.Interaction, i.Message.ID, &edit); err != nil {
			log.Println("lobby FollowupMessageEdit:", err)
		}
	}
}

func resolveLobbyRequest(i *discordgo.InteractionCreate, contractID string, coopID string) (string, string, string) {
	if contractID == "" || coopID == "" {
		contract := FindContract(i.ChannelID)
		if contract == nil {
			commandLink := bottools.GetFormattedCommand("lobby")
			if commandLink == "" {
				commandLink = "/lobby"
			}
			return "", "", fmt.Sprintf("No contract found in this channel. Run %s in a channel with an active contract, or provide a `contract-id` and `coop-id`.", commandLink)
		}
		contractID = contract.ContractID
		coopID = strings.ToLower(contract.CoopID)
	}

	return contractID, coopID, ""
}

func buildLobbyComponents(contractID string, coopID string, userID string, bypassCache bool, includeButtons bool) []discordgo.MessageComponent {
	content, err := buildLobbyContent(contractID, coopID, userID, bypassCache)
	if err != nil {
		components := []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: err.Error()},
		}
		if includeButtons {
			components = append(components, lobbyButtons(contractID, coopID))
		}
		return components
	}

	components := []discordgo.MessageComponent{
		discordgo.TextDisplay{Content: content.header},
		discordgo.TextDisplay{Content: content.lobby},
	}
	if includeButtons {
		components = append(components, lobbyButtons(contractID, coopID))
	}

	return components
}

type lobbyContent struct {
	header string
	lobby  string
}

func buildLobbyContent(contractID string, coopID string, userID string, bypassCache bool) (lobbyContent, error) {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	eiContract := ei.EggIncContractsAll[contractID]
	if eiContract.ID == "" {
		return lobbyContent{}, fmt.Errorf("invalid contract ID")
	}

	var (
		coopStatus       *ei.ContractCoopStatusResponse
		dataTimestampStr string
		err              error
	)

	if bypassCache {
		coopStatus, _, dataTimestampStr, err = ei.GetCoopStatusUncached(contractID, coopID, eiID)
	} else {
		coopStatus, _, dataTimestampStr, err = ei.GetCoopStatus(contractID, coopID, eiID)
	}
	if err != nil {
		return lobbyContent{}, err
	}
	if coopStatus.GetResponseStatus() != ei.ContractCoopStatusResponse_NO_ERROR {
		return lobbyContent{}, fmt.Errorf("%s", ei.ContractCoopStatusResponse_ResponseStatus_name[int32(coopStatus.GetResponseStatus())])
	}
	if coopStatus.GetGrade() == ei.Contract_GRADE_UNSET {
		return lobbyContent{}, fmt.Errorf("No grade found for contract %s/%s", contractID, coopID)
	}

	grade := int(coopStatus.GetGrade())
	if grade < 0 || grade >= len(eiContract.Grade) {
		return lobbyContent{}, fmt.Errorf("Invalid grade found for contract %s/%s", contractID, coopID)
	}

	coopID = coopStatus.GetCoopIdentifier()

	memberNames := make([]string, 0, len(coopStatus.GetContributors()))
	for _, contributor := range coopStatus.GetContributors() {
		name := contributor.GetUserName()
		if name == "" {
			name = contributor.GetUserId()
		}
		if name == "" {
			name = "Unknown"
		}
		memberNames = append(memberNames, name)
	}
	sort.Strings(memberNames)

	carpetLink := fmt.Sprintf("%s/%s/%s", "https://eicoop-carpet.netlify.app", contractID, coopID)
	var header strings.Builder
	fmt.Fprintf(&header, "Lobby: %d/%d\n%s contract %s/[**%s**](%s)\n",
		len(memberNames), eiContract.MaxCoopSize,
		ei.GetBotEmojiMarkdown("contract_grade_"+ei.GetContractGradeString(grade)),
		coopStatus.GetContractIdentifier(), coopID, carpetLink)
	header.WriteString(dataTimestampStr)

	var lobby strings.Builder
	if len(memberNames) == 0 {
		lobby.WriteString("No lobby members found.")
	} else {
		for _, name := range memberNames {
			fmt.Fprintf(&lobby, "- %s\n", name)
		}
	}

	return lobbyContent{header: header.String(), lobby: lobby.String()}, nil
}

func lobbyButtons(contractID string, coopID string) discordgo.ActionsRow {
	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Refresh",
				Style:    discordgo.SecondaryButton,
				CustomID: fmt.Sprintf("lobby#refresh#%s#%s", contractID, coopID),
				Emoji:    &discordgo.ComponentEmoji{Name: "🔄"},
			},
			discordgo.Button{
				Label:    "Close",
				Style:    discordgo.DangerButton,
				CustomID: fmt.Sprintf("lobby#close#%s#%s", contractID, coopID),
			},
		},
	}
}
