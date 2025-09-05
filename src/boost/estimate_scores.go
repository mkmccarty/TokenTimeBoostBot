package boost

import (
	"strings"

	"github.com/bwmarrin/discordgo"
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
	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

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
	if contractID == "" || coopID == "" {
		contract := FindContract(i.ChannelID)
		if contract == nil {
			_, _ = s.FollowupMessageCreate(i.Interaction, true,
				&discordgo.WebhookParams{
					Content: "No contract found in this channel. Please provide a contract-id and coop-id.",
				})

			return
		}
		contractID = strings.ToLower(contract.ContractID)
		coopID = strings.ToLower(contract.CoopID)
	}

	var str string
	str, fields, scores := DownloadCoopStatusTeamwork(contractID, coopID, 0)
	if fields == nil || strings.HasSuffix(str, "no such file or directory") || strings.HasPrefix(str, "No grade found") {
		_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Flags:   flags,
			Content: str,
		})
		return
	}

	var footer strings.Builder
	footer.WriteString("-# MAX : Max Chicken Runs & ∆T-Val\n")
	footer.WriteString("-# TVAL: Coop Size-1 Chicken Runs & ∆T-Val\n")
	footer.WriteString("-# SINK: Max Chicken Runs & Token Sink\n")
	footer.WriteString("-# RUNS: Coop Size-1 Chicken Runs, No token sharing\n")
	footer.WriteString("-# MIN:  No Chicken Runs,No token sharing\n")
	footer.WriteString("-# BASE: No BTV, No Chicken Runs, No token sharing\n")
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags: flags | discordgo.MessageFlagsIsComponentsV2,
		Components: []discordgo.MessageComponent{
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
		},
	})
}
