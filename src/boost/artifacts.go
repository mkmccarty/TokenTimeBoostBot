package boost

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

func getArtifactsComponents(userID string, channelID string, contractOnly bool) (string, []discordgo.MessageComponent) {
	minValues := 0
	minV := 0

	// is this channelID a thread

	var builder strings.Builder
	if !contractOnly {
		fmt.Fprintf(&builder, "Select your global coop artifacts <@%s>", userID)
	} else {
		fmt.Fprintf(&builder, "Adjust your coop artifact overrides for this contract <@%s>", userID)
	}

	// These are the global settings
	deflector := ""
	metronome := ""
	compass := ""
	gusset := ""
	coll := ""

	temp := "PERM"
	if contractOnly {
		temp = "TEMP"
		contract := FindContract(channelID)
		if contract != nil {
			if userInContract(contract, userID) {
				for a := range contract.Boosters[userID].ArtifactSet.Artifacts {
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Deflector") {
						deflector = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Metronome") {
						metronome = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Compass") {
						compass = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Gusset") {
						gusset = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
				}
			}
		} else {
			return "No contract exists in this channel", nil
		}
	} else {
		deflector = farmerstate.GetMiscSettingString(userID, "defl")
		metronome = farmerstate.GetMiscSettingString(userID, "metr")
		compass = farmerstate.GetMiscSettingString(userID, "comp")
		gusset = farmerstate.GetMiscSettingString(userID, "guss")
		coll = farmerstate.GetMiscSettingString(userID, "collegg")
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
							Default:     deflector == "T4L",
							Emoji:       ei.GetBotComponentEmoji("DT4La")},
						{
							Label:       "Deflector T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     deflector == "T4E",
							Emoji:       ei.GetBotComponentEmoji("DT4Ea"),
						},
						{
							Label:       "Deflector T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     deflector == "T4R",
							Emoji:       ei.GetBotComponentEmoji("DT4Ra"),
						},
						{
							Label:       "Deflector T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     deflector == "T4C",
							Emoji:       ei.GetBotComponentEmoji("afx_tachyon_deflector_4"),
						},
						{
							Label:       "Deflector T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     deflector == "T3R",
							Emoji:       ei.GetBotComponentEmoji("DT3Ra"),
						},
						{
							Label:       "Deflector T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     deflector == "T3C",
							Emoji:       ei.GetBotComponentEmoji("afx_tachyon_deflector_3"),
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
							Default:     metronome == "T4L",
							Emoji:       ei.GetBotComponentEmoji("MT4La"),
						},
						{
							Label:       "Metronome T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     metronome == "T4E",
							Emoji:       ei.GetBotComponentEmoji("MT4Ea"),
						},
						{
							Label:       "Metronome T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     metronome == "T4R",
							Emoji:       ei.GetBotComponentEmoji("MT4Ra"),
						},
						{
							Label:       "Metronome T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     metronome == "T4C",
							Emoji:       ei.GetBotComponentEmoji("afx_quantum_metronome_4"),
						},
						{
							Label:       "Metronome T3E",
							Description: "Epic",
							Value:       "T3E",
							Default:     metronome == "T3E",
							Emoji:       ei.GetBotComponentEmoji("MT3Ea"),
						},
						{
							Label:       "Metronome T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     metronome == "T3R",
							Emoji:       ei.GetBotComponentEmoji("MT3Ra"),
						},
						{
							Label:       "Metronome T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     metronome == "T3C",
							Emoji:       ei.GetBotComponentEmoji("afx_quantum_metronome_3"),
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
							Default:     compass == "T4L",
							Emoji:       ei.GetBotComponentEmoji("CT4La"),
						},
						{
							Label:       "Compass T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     compass == "T4E",
							Emoji:       ei.GetBotComponentEmoji("CT4Ea"),
						},
						{
							Label:       "Compass T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     compass == "T4R",
							Emoji:       ei.GetBotComponentEmoji("CT4Ra"),
						},
						{
							Label:       "Compass T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     compass == "T4C",
							Emoji:       ei.GetBotComponentEmoji("afx_interstellar_compass_4"),
						},
						{
							Label:       "Compass T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     compass == "T3R",
							Emoji:       ei.GetBotComponentEmoji("CT3Ra"),
						},
						{
							Label:       "Compass T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     compass == "T3C",
							Emoji:       ei.GetBotComponentEmoji("afx_interstellar_compass_3"),
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
							Default:     gusset == "T4L",
							Emoji:       ei.GetBotComponentEmoji("GT4Ra"),
						},
						{
							Label:       "Gusset T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     gusset == "T4E",
							Emoji:       ei.GetBotComponentEmoji("GT4Ea"),
						},
						{
							Label:       "Gusset T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     gusset == "T4C",
							Emoji:       ei.GetBotComponentEmoji("afx_ornate_gusset_4"),
						},
						{
							Label:       "Gusset T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     gusset == "T3R",
							Emoji:       ei.GetBotComponentEmoji("GT3Ra"),
						},
						{
							Label:       "Gusset T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     gusset == "T3C",
							Emoji:       ei.GetBotComponentEmoji("afx_ornate_gusset_3"),
						},
						{
							Label:       "Gusset T2E",
							Description: "Epic",
							Value:       "T2E",
							Default:     gusset == "T2E",
							Emoji:       ei.GetBotComponentEmoji("GT2Ea"),
						},
					},
				},
			},
		},
	}

	if !contractOnly {
		_, carbonfiber := ei.FindEggComponentEmoji("CARBON-FIBER")
		_, chocolate := ei.FindEggComponentEmoji("CHOCOLATE")
		_, easter := ei.FindEggComponentEmoji("EASTER")
		_, firework := ei.FindEggComponentEmoji("FIREWORK")
		_, pumpkin := ei.FindEggComponentEmoji("PUMPKIN")
		_, waterballoon := ei.FindEggComponentEmoji("WATERBALLOON")
		_, lithium := ei.FindEggComponentEmoji("LITHIUM")

		component = append(component, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#COLLEGG#" + userID + "#" + temp,
					Placeholder: "Select your Colleggtibles",
					MinValues:   &minV,
					MaxValues:   7,
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
							Label:       "Chocolate",
							Description: "3x Away Earnings",
							Value:       "Chocolate",
							Default:     strings.Contains(coll, "Chocolate"),
							Emoji: &discordgo.ComponentEmoji{
								Name: chocolate.Name,
								ID:   chocolate.ID,
							},
						},
						{
							Label:       "Easter",
							Description: "5% Internal Hatchery Rate",
							Value:       "Easter",
							Default:     strings.Contains(coll, "Easter"),
							Emoji: &discordgo.ComponentEmoji{
								Name: easter.Name,
								ID:   easter.ID,
							},
						},
						{
							Label:       "Firework",
							Description: "5% Earnings",
							Value:       "Firework",
							Default:     strings.Contains(coll, "Firework"),
							Emoji: &discordgo.ComponentEmoji{
								Name: firework.Name,
								ID:   firework.ID,
							},
						},
						{
							Label:       "Lithium",
							Description: "-10% Vehicle Cost",
							Value:       "Lithium",
							Default:     strings.Contains(coll, "Lithium"),
							Emoji: &discordgo.ComponentEmoji{
								Name: lithium.Name,
								ID:   lithium.ID,
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
						{
							Label:       "Waterballoon",
							Description: "95% Research Cost",
							Value:       "Waterballoon",
							Default:     strings.Contains(coll, "Waterballoon"),
							Emoji: &discordgo.ComponentEmoji{
								Name: waterballoon.Name,
								ID:   waterballoon.ID,
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

	contractOnly := false

	str, comp := getArtifactsComponents(userID, i.ChannelID, contractOnly)

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
	//override := reaction[len(reaction)-1]

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})

	data := i.MessageComponentData()

	setValue := true
	if len(data.Values) == 0 {
		setValue = false
	}

	//if override == "PERM" {
	switch cmd {
	case "defl", "metr", "comp", "guss":
		if setValue {
			farmerstate.SetMiscSettingString(userID, cmd, data.Values[0])
		} else {
			farmerstate.SetMiscSettingString(userID, cmd, "") // Clear the value
		}
	case "collegg":
		farmerstate.SetMiscSettingString(userID, cmd, strings.Join(data.Values, ","))
	}
	//} else {
	contract := FindContract(i.ChannelID)
	if contract != nil {
		if userInContract(contract, userID) {
			// User in this contract
			currentSet := contract.Boosters[userID].ArtifactSet

			var prefix string
			switch cmd {
			case "defl":
				prefix = "D-"
			case "metr":
				prefix = "M-"
			case "comp":
				prefix = "C-"
			case "guss":
				prefix = "G-"
			}
			var newArtifact *ei.Artifact
			if len(data.Values) == 0 {
				newArtifact = ei.ArtifactMap[prefix+"NONE"]
			} else {
				newArtifact = ei.ArtifactMap[prefix+data.Values[0]]
			}
			// Check if the artifact already exists in the current set
			exists := false
			for i, artifact := range currentSet.Artifacts {
				if artifact.Type == newArtifact.Type {
					exists = true
					if setValue {
						currentSet.Artifacts[i] = *newArtifact
					} else {
						// Removing this artifact
						currentSet.Artifacts = append(currentSet.Artifacts[:i], currentSet.Artifacts[i+1:]...)
					}
					break
				}
			}
			// If the artifact doesn't exist, add it to the current set
			if !exists {
				currentSet.Artifacts = append(currentSet.Artifacts, *newArtifact)
			}

			contract.Boosters[userID].ArtifactSet = getUserArtifacts(userID, &currentSet)

			refreshBoostListMessage(s, contract)

		}
	}
	//}
	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			//Content: "",
			//Flags: discordgo.MessageFlagsEphemeral,
		})

}
