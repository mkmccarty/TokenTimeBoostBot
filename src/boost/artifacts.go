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
	defl := ""
	metr := ""
	comp := ""
	guss := ""
	coll := ""

	temp := "PERM"
	if contractOnly {
		temp = "TEMP"
		contract := FindContract(channelID)
		if contract != nil {
			if userInContract(contract, userID) {
				for a := range contract.Boosters[userID].ArtifactSet.Artifacts {
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Deflector") {
						defl = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Metronome") {
						metr = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Compass") {
						comp = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Gusset") {
						guss = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
				}
			}
		} else {
			return "No contract exists in this channel", nil
		}
	} else {
		defl = farmerstate.GetMiscSettingString(userID, "defl")
		metr = farmerstate.GetMiscSettingString(userID, "metr")
		comp = farmerstate.GetMiscSettingString(userID, "comp")
		guss = farmerstate.GetMiscSettingString(userID, "guss")
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
							Default:     defl == "T4L",
							Emoji: &discordgo.ComponentEmoji{
								Name: "DT4La",
								ID:   "1279530304612204574",
							},
						},
						{
							Label:       "Deflector T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     defl == "T4E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "DT4Ea",
								ID:   "1279530331820523551",
							},
						},
						{
							Label:       "Deflector T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     defl == "T4R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "DT4Ra",
								ID:   "1279530356965380177",
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
								Name: "DT3Ra",
								ID:   "1279530534539890748",
							},
						},
						{
							Label:       "Deflector T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     defl == "T3C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_tachyon_deflector_3",
								ID:   "1279201840755642479",
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
								Name: "MT4La",
								ID:   "1279530626470383618",
							},
						},
						{
							Label:       "Metronome T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     metr == "T4E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "MT4Ea",
								ID:   "1279530656614977649",
							},
						},
						{
							Label:       "Metronome T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     metr == "T4R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "MT4Ra",
								ID:   "1279530692392390656",
							},
						},
						{
							Label:       "Metronome T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     metr == "T4C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_quantum_metronome_4",
								ID:   "1279205882097565788",
							},
						},
						{
							Label:       "Metronome T3E",
							Description: "Epic",
							Value:       "T3E",
							Default:     metr == "T3E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "MT3Ea",
								ID:   "1279530849339052072",
							},
						},
						{
							Label:       "Metronome T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     metr == "T3R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "MT3Ra",
								ID:   "1279530956377554955",
							},
						},
						{
							Label:       "Metronome T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     metr == "T3C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_quantum_metronome_3",
								ID:   "1279205835893117032",
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
								Name: "CT4La",
								ID:   "1279531047251607605",
							},
						},
						{
							Label:       "Compass T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     comp == "T4E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "CT4Ea",
								ID:   "1279531101672702022",
							},
						},
						{
							Label:       "Compass T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     comp == "T4R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "CT4Ra",
								ID:   "1279532221455269949",
							},
						},
						{
							Label:       "Compass T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     comp == "T4C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_interstellar_compass_4",
								ID:   "1279205584645652491",
							},
						},
						{
							Label:       "Compass T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     comp == "T3R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "CT3Ra",
								ID:   "1279532322802237614",
							},
						},
						{
							Label:       "Compass T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     comp == "T3C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_interstellar_compass_3",
								ID:   "1279205540731551854",
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
								Name: "GT4Ra",
								ID:   "1279532396802474014"},
						},
						{
							Label:       "Gusset T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     guss == "T4E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "GT4Ea",
								ID:   "1279532417627197502",
							},
						},
						{
							Label:       "Gusset T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     guss == "T4C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_ornate_gusset_4",
								ID:   "1279206200541450364",
							},
						},
						{
							Label:       "Gusset T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     guss == "T3R",
							Emoji: &discordgo.ComponentEmoji{
								Name: "GT3Ra",
								ID:   "1279532512007295008",
							},
						},
						{
							Label:       "Gusset T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     guss == "T3C",
							Emoji: &discordgo.ComponentEmoji{
								Name: "afx_ornate_gusset_3",
								ID:   "1279206172355727514",
							},
						},
						{
							Label:       "Gusset T2E",
							Description: "Epic",
							Value:       "T2E",
							Default:     guss == "T2E",
							Emoji: &discordgo.ComponentEmoji{
								Name: "GT2Ea",
								ID:   "1279532584904298588",
							},
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

		component = append(component, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "as_#COLLEGG#" + userID + "#" + temp,
					Placeholder: "Select your Colleggtibles",
					MinValues:   &minV,
					MaxValues:   6,
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
