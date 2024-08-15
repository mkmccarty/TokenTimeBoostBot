package boost

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func getArtifactsComponents(userID string, contractOnly bool) (string, []discordgo.MessageComponent) {
	minValues := 0
	minV := 0

	// is this channelID a thread

	var builder strings.Builder
	if !contractOnly {
		fmt.Fprintf(&builder, "Select your coop artifacts <@%s>", userID)
	} else {
		fmt.Fprintf(&builder, "Adjust your coop artifact overrides for this contract <@%s>", userID)
		fmt.Fprintf(&builder, "\n**This doesn't do anything until Contract ELR is implemented.**")
	}

	// Menus for Deflector, Metronome, Compass and Gusset
	// Menu for Colleggtables

	defl := farmerstate.GetMiscSettingString(userID, "defl")
	metr := farmerstate.GetMiscSettingString(userID, "metr")
	comp := farmerstate.GetMiscSettingString(userID, "comp")
	guss := farmerstate.GetMiscSettingString(userID, "guss")
	coll := farmerstate.GetMiscSettingString(userID, "collegg")

	temp := "PERM"
	if contractOnly {
		temp = "TEMP"
	}
	// Remove the extra closing brace

	component := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#DEFL#" + userID + "#" + temp,
					Placeholder: "Select your Deflector...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Deflector T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     defl == "T4L",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_tachyon_deflector_4",
								ID:   "863987172674371604",
							},
						},
						{
							Label:       "Deflector T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     defl == "T4E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_tachyon_deflector_4",
								ID:   "863987172674371604",
							},
						},
						{
							Label:       "Deflector T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     defl == "T4R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_tachyon_deflector_4",
								ID:   "863987172674371604",
							},
						},
						{
							Label:       "Deflector T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     defl == "T4C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_tachyon_deflector_4",
								ID:   "863987172674371604",
							},
						},
						{
							Label:       "Deflector T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     defl == "T3R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_tachyon_deflector_3",
								ID:   "857435256944984095",
							},
						},
						{
							Label:       "Deflector T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     defl == "T3C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_tachyon_deflector_3",
								ID:   "857435256944984095",
							},
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#METR#" + userID + "#" + temp,
					Placeholder: "Select your Metronome...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Metronome T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     metr == "T4L",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_quantum_metronome_4",
								ID:   "863987171563798528",
							},
						},
						{
							Label:       "Metronome T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     metr == "T4E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_quantum_metronome_4",
								ID:   "863987171563798528",
							},
						},
						{
							Label:       "Metronome T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     metr == "T4R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_quantum_metronome_4",
								ID:   "863987171563798528",
							},
						},
						{
							Label:       "Metronome T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     metr == "T4C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_quantum_metronome_4",
								ID:   "863987171563798528",
							},
						},
						{
							Label:       "Metronome T3E",
							Description: "Epic",
							Value:       "T3E",
							Default:     metr == "T3E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_quantum_metronome_3",
								ID:   "857435255182983179",
							},
						},
						{
							Label:       "Metronome T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     metr == "T3R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_quantum_metronome_3",
								ID:   "857435255182983179",
							},
						},
						{
							Label:       "Metronome T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     metr == "T3C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_quantum_metronome_3",
								ID:   "857435255182983179",
							},
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#COMP#" + userID + "#" + temp,
					Placeholder: "Select your Compass...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Compass T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     comp == "T4L",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_interstellar_compass_4",
								ID:   "857435263658360852",
							},
						},
						{
							Label:       "Compass T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     comp == "T4E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_interstellar_compass_4",
								ID:   "857435263658360852",
							},
						},
						{
							Label:       "Compass T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     comp == "T4R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_interstellar_compass_4",
								ID:   "857435263658360852",
							},
						},
						{
							Label:       "Compass T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     comp == "T4C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_interstellar_compass_4",
								ID:   "857435263658360852",
							},
						},
						{
							Label:       "Compass T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     comp == "T3R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_interstellar_compass_3",
								ID:   "863987172091756594",
							},
						},
						{
							Label:       "Compass T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     comp == "T3C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_interstellar_compass_3",
								ID:   "863987172091756594",
							},
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#GUSS#" + userID + "#" + temp,
					Placeholder: "Select your Gusset...",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Gusset T4L",
							Description: "Legendary",
							Value:       "T4L",
							Default:     guss == "T4L",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_ornate_gusset_4",
								ID:   "857435261024600085",
							},
						},
						{
							Label:       "Gusset T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     guss == "T4E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_ornate_gusset_4",
								ID:   "857435261024600085",
							},
						},
						{
							Label:       "Gusset T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     guss == "T4C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_ornate_gusset_4",
								ID:   "857435261024600085",
							},
						},
						{
							Label:       "Gusset T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     guss == "T3R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_ornate_gusset_3",
								ID:   "1273680487092850760",
							},
						},
						{
							Label:       "Gusset T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     guss == "T3C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_ornate_gusset_3",
								ID:   "1273680487092850760",
							},
						},
					},
				},
			},
		},
	}

	if !contractOnly {
		_, carbonfiber := ei.FindEggComponentEmoji("CARBON-FIBER")
		_, pumpkin := ei.FindEggComponentEmoji("PUMPKIN")
		component = append(component, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#COLLEGG#" + userID + "#" + temp,
					Placeholder: "Select your Colleggtibles",
					MinValues:   &minV,
					MaxValues:   2,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Carbon Fiber",
							Description: "5% Shipping",
							Value:       "CarbonFiber",
							Default:     strings.Contains(coll, "CarbonFiber"),
							Emoji: &discordgo.ComponentEmoji{
								Name: carbonfiber.Name,
								ID:   carbonfiber.ID,
							},
						},
						{
							Label:       "Pumpkin",
							Description: "5% Shipping",
							Value:       "Pumpkin",
							Default:     strings.Contains(coll, "Pumpkin"),
							Emoji: &discordgo.ComponentEmoji{
								Name: pumpkin.Name,
								ID:   pumpkin.ID,
							},
						},
					},
				},
			},
		})
	}

	return builder.String(), component
}

// SlashArtifactsCommand creates a new slash command for setting Egg, Inc name
func SlashArtifactsCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Indicate best contract artifacts you have.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "contract-only",
				Description: "Specify a set only for this contract.",
				Required:    false,
			},
		},
	}
}

func getInteractionUserID(i *discordgo.InteractionCreate) string {
	if i.GuildID == "" {
		return i.User.ID
	}
	return i.Member.User.ID
}

// HandleArtifactCommand handles the /artifacts command
func HandleArtifactCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {

	userID := getInteractionUserID(i)

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	contractOnly := false

	if opt, ok := optionMap["contract-only"]; ok {
		contractOnly = opt.BoolValue()
	}

	str, comp := getArtifactsComponents(userID, contractOnly)

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Components: comp,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	},
	)
	if err != nil {
		fmt.Println("InteractionRespond: ", err)
	}

}

// HandleArtifactReactions handles all the button reactions for a contract settings
func HandleArtifactReactions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// cs_#Name # cs_#ID # HASH
	reaction := strings.Split(i.MessageComponentData().CustomID, "#")
	cmd := strings.ToLower(reaction[1])
	userID := reaction[len(reaction)-2]
	override := reaction[len(reaction)-1]

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			//Content: "",
			//Flags: discordgo.MessageFlagsEphemeral,
		})

	data := i.MessageComponentData()

	if override == "PERM" {
		switch cmd {
		case "defl", "metr", "comp", "guss":
			if len(data.Values) > 0 {
				farmerstate.SetMiscSettingString(userID, cmd, data.Values[0])
			} else {
				farmerstate.SetMiscSettingString(userID, cmd, "") // Clear the value
			}
		case "collegg":
			farmerstate.SetMiscSettingString(userID, cmd, strings.Join(data.Values, ","))
		}
	} else {
		// TODO: Set the user artifacts for the contract
		log.Println("Contract artifacts not implemented yet.")
	}

}
