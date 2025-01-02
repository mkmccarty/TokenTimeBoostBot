package events

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// SlashEventHelperCommand returns the command for the /launch-helper command
func SlashEventHelperCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Display Last Event(s) and current Event(s) information.",
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
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "ultra",
				Description: "Show ultra event info. Default is false. [Sticky]",
				Required:    false,
			},
		},
	}
}

// HandleEventHelper handles the /launch-helper command
func HandleEventHelper(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ultraIcon, _, _ := ei.GetBotEmoji("ultra")

	userID := getInteractionUserID(i)

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	// User interacting with bot, is this first time ?
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	ultra := false
	if opt, ok := optionMap["ultra"]; ok {
		ultra = opt.BoolValue()
		farmerstate.SetMiscSettingFlag(userID, "ultra", ultra)
	} else {
		ultra = farmerstate.GetMiscSettingFlag(userID, "ultra")
	}

	var field []*discordgo.MessageEmbedField
	var events strings.Builder

	eventMutex.Lock()
	localLastEvent := make([]ei.EggEvent, len(LastEvent))
	copy(localLastEvent, LastEvent)

	localEggIncEvents := make([]ei.EggEvent, len(EggIncEvents))
	copy(localEggIncEvents, EggIncEvents)
	eventMutex.Unlock()

	events.WriteString("## Current Events:\n")
	// Build list of current Events
	for _, e := range localEggIncEvents {
		ultraStr := ""
		if e.Ultra {
			ultraStr = ultraIcon
			if !ultra {
				continue
			}
		}
		hours := e.EndTime.Sub(e.StartTime).Hours()
		if hours < 1.0 {
			mins := e.EndTime.Sub(e.StartTime).Minutes()
			events.WriteString(fmt.Sprintf("%s%s for %.2dm ends <t:%d:R>\n", ultraStr, e.Message, int(mins), e.EndTime.Unix()))
		} else {
			events.WriteString(fmt.Sprintf("%s%s for %.2dh ends <t:%d:R>\n", ultraStr, e.Message, int(hours), e.EndTime.Unix()))
		}
	}

	var prevEvents strings.Builder
	var ultraEvents strings.Builder
	str := ""

	continuedStr := ""

	// Previous Non Ultra Events
	for _, e := range localLastEvent {
		ultraStr := ""
		if e.Ultra {
			ultraStr = ultraIcon
			if !ultra {
				continue
			}
		}
		hours := e.EndTime.Sub(e.StartTime).Hours()
		if hours < 1.0 {
			mins := e.EndTime.Sub(e.StartTime).Minutes()
			str = fmt.Sprintf("%s%s for %.2dm <t:%d:R>\n", ultraStr, e.Message, int(mins), e.StartTime.Unix())
		} else {
			str = fmt.Sprintf("%s%s for %.2dh <t:%d:R>\n", ultraStr, e.Message, int(hours), e.StartTime.Unix())
		}

		if e.Ultra {
			ultraEvents.WriteString(str)
		} else {
			prevEvents.WriteString(str)
		}
		/*
			if len(prevEvents.String()) > 900 {
				field = append(field, &discordgo.MessageEmbedField{
					Name:   "Event History",
					Value:  prevEvents.String(),
					Inline: false,
				})
				prevEvents.Reset()
				continuedStr = " (Continued)"
			}*/
	}

	field = append(field, &discordgo.MessageEmbedField{
		Name:   "Event History",
		Value:  prevEvents.String(),
		Inline: false,
	})

	if ultra {
		field = append(field, &discordgo.MessageEmbedField{
			Name:   "Ultra Event History" + continuedStr,
			Value:  ultraEvents.String(),
			Inline: false,
		})
	}

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			Content: events.String() + "\n\n",
			Embeds: []*discordgo.MessageEmbed{{
				Type:   discordgo.EmbedTypeRich,
				Color:  0x0055FF,
				Fields: field,
			}},
		})

}
