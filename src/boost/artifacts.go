package boost

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

func getArtifactsPageFromContent(content string) string {
	if strings.Contains(content, "Set: IHR") {
		return "ihr"
	}
	if strings.Contains(content, "Set: Colleggtibles") {
		return "collegg"
	}
	return "delivery"
}

func populateArtifactsFromBackup(s *discordgo.Session, userID string) (string, error) {
	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")
	if eiID == "" {
		return "No saved Egg Inc ID found. Run /register first.", nil
	}

	backup, _ := ei.GetFirstContactFromAPI(s, eiID, userID, true)
	if backup == nil || backup.GetArtifactsDb() == nil {
		return "Unable to fetch backup artifacts right now.", nil
	}

	best := ei.GetBestCoopArtifactsFromInventory(backup.GetArtifactsDb().GetInventoryItems())

	// Always refresh all supported keys so stale values don't linger.
	targets := []string{"defl", "metr", "comp", "guss", "chalice", "monocle", "siab"}
	for _, key := range targets {
		val := best[key]
		if key == "siab" && val == "" {
			val = best["SIAB"]
		}
		farmerstate.SetMiscSettingString(userID, key, val)
	}

	// Keep IHR deflector aligned with best deflector from the same primary artifact DB.
	farmerstate.SetMiscSettingString(userID, "defl-ihr", best["defl"])

	updatedContracts := 0
	for _, contract := range Contracts {
		if contract == nil || contract.State == ContractStateSignup || contract.State == ContractStateCompleted || contract.State == ContractStateArchive {
			continue
		}
		if !UserInContract(contract, userID) {
			continue
		}

		contract.mutex.Lock()
		if contract.Boosters[userID] != nil {
			contract.Boosters[userID].ArtifactSet = getUserArtifacts(userID, nil)
			updatedContracts++
		}
		contract.mutex.Unlock()

		refreshBoostListMessage(s, contract, false)
	}

	if updatedContracts > 0 {
		return fmt.Sprintf("Populated from backup and updated %d running contract(s) (%s).", updatedContracts, time.Now().Format("15:04:05")), nil
	}

	return fmt.Sprintf("Populated from backup (%s).", time.Now().Format("15:04:05")), nil
}

func getArtifactsComponents(userID string, channelID string, contractOnly bool, page string, backupButtonLabel string) (string, []discordgo.MessageComponent) {
	minValues := 0
	minV := 0
	if page == "" {
		page = "delivery"
	}
	page = strings.ToLower(page)
	if page != "delivery" && page != "ihr" && page != "collegg" {
		page = "delivery"
	}

	// is this channelID a thread
	as := getUserArtifacts(userID, nil)

	var builder strings.Builder
	if !contractOnly {
		fmt.Fprintf(&builder, "Select your global coop artifacts <@%s>\nELR: %1.3f", userID, as.LayRate)
	} else {
		fmt.Fprintf(&builder, "Adjust your coop artifact overrides for this contract <@%s>\n ELR: %2.3f  SR:%2.3f", userID, as.LayRate, as.ShipRate)
	}

	// These are the global settings
	deflector := ""
	metronome := ""
	compass := ""
	gusset := ""
	ihrDeflector := ""
	chalice := ""
	monocle := ""
	siab := ""
	coll := ""

	temp := "PERM"
	if contractOnly {
		temp = "TEMP"
		contract := FindContract(channelID)
		if contract != nil {
			if UserInContract(contract, userID) {
				for a := range contract.Boosters[userID].ArtifactSet.Artifacts {
					if contract.Boosters[userID].ArtifactSet.Artifacts[a].Type == "IHR Deflector" {
						ihrDeflector = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
						continue
					}
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
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Chalice") {
						chalice = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "Monocle") {
						monocle = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
					}
					if strings.Contains(contract.Boosters[userID].ArtifactSet.Artifacts[a].Type, "SIAB") {
						siab = contract.Boosters[userID].ArtifactSet.Artifacts[a].Quality
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
		ihrDeflector = farmerstate.GetMiscSettingString(userID, "defl-ihr")
		chalice = farmerstate.GetMiscSettingString(userID, "chalice")
		monocle = farmerstate.GetMiscSettingString(userID, "monocle")
		siab = farmerstate.GetMiscSettingString(userID, "siab")
		if siab == "" {
			siab = farmerstate.GetMiscSettingString(userID, "SIAB")
		}
		coll = farmerstate.GetMiscSettingString(userID, "collegg")

		// Need to perform a conversion on what's in coll.
		// CarbonFiber,Chocolate,Easter,Firework,Pumpkin,Waterballoon,Lithium
		coll = strings.ToUpper(coll)
		coll = strings.ReplaceAll(coll, "CARBONFIBER", "CARBON FIBER")
		coll = strings.ReplaceAll(coll, "FLAMERETARDANT", "FLAME RETARDANT")
	}
	builder.WriteString("\nSet: ")
	switch page {
	case "ihr":
		builder.WriteString("IHR")
	case "collegg":
		builder.WriteString("Colleggtibles")
	default:
		builder.WriteString("Delivery")
	}

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
							Emoji:       ei.GetBotComponentEmoji("defl_T4L")},
						{
							Label:       "Deflector T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     deflector == "T4E",
							Emoji:       ei.GetBotComponentEmoji("defl_T4E"),
						},
						{
							Label:       "Deflector T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     deflector == "T4R",
							Emoji:       ei.GetBotComponentEmoji("defl_T4R"),
						},
						{
							Label:       "Deflector T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     deflector == "T4C",
							Emoji:       ei.GetBotComponentEmoji("defl_T4C"),
						},
						{
							Label:       "Deflector T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     deflector == "T3R",
							Emoji:       ei.GetBotComponentEmoji("defl_T3R"),
						},
						{
							Label:       "Deflector T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     deflector == "T3C",
							Emoji:       ei.GetBotComponentEmoji("defl_T3C"),
						},
						{
							Label:       "None",
							Description: "No Deflector equipped",
							Value:       "NONE",
							Default:     deflector == "NONE" || deflector == "",
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
							Emoji:       ei.GetBotComponentEmoji("metr_T4L"),
						},
						{
							Label:       "Metronome T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     metronome == "T4E",
							Emoji:       ei.GetBotComponentEmoji("metr_T4E"),
						},
						{
							Label:       "Metronome T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     metronome == "T4R",
							Emoji:       ei.GetBotComponentEmoji("metr_T4R"),
						},
						{
							Label:       "Metronome T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     metronome == "T4C",
							Emoji:       ei.GetBotComponentEmoji("metr_T4C"),
						},
						{
							Label:       "Metronome T3E",
							Description: "Epic",
							Value:       "T3E",
							Default:     metronome == "T3E",
							Emoji:       ei.GetBotComponentEmoji("metr_T3E"),
						},
						{
							Label:       "Metronome T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     metronome == "T3R",
							Emoji:       ei.GetBotComponentEmoji("metr_T3R"),
						},
						{
							Label:       "Metronome T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     metronome == "T3C",
							Emoji:       ei.GetBotComponentEmoji("metr_T3C"),
						},
						{
							Label:       "None",
							Description: "No Metronome equipped",
							Value:       "NONE",
							Default:     metronome == "NONE" || metronome == "",
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
							Emoji:       ei.GetBotComponentEmoji("comp_T4L"),
						},
						{
							Label:       "Compass T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     compass == "T4E",
							Emoji:       ei.GetBotComponentEmoji("comp_T4E"),
						},
						{
							Label:       "Compass T4R",
							Description: "Rare",
							Value:       "T4R",
							Default:     compass == "T4R",
							Emoji:       ei.GetBotComponentEmoji("comp_T4R"),
						},
						{
							Label:       "Compass T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     compass == "T4C",
							Emoji:       ei.GetBotComponentEmoji("comp_T4C"),
						},
						{
							Label:       "Compass T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     compass == "T3R",
							Emoji:       ei.GetBotComponentEmoji("comp_T3R"),
						},
						{
							Label:       "Compass T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     compass == "T3C",
							Emoji:       ei.GetBotComponentEmoji("comp_T3C"),
						},
						{
							Label:       "None",
							Description: "No Compass equipped",
							Value:       "NONE",
							Default:     compass == "NONE" || compass == "",
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
							Emoji:       ei.GetBotComponentEmoji("gusset_T4L"),
						},
						{
							Label:       "Gusset T4E",
							Description: "Epic",
							Value:       "T4E",
							Default:     gusset == "T4E",
							Emoji:       ei.GetBotComponentEmoji("gusset_T4E"),
						},
						{
							Label:       "Gusset T4C",
							Description: "Common",
							Value:       "T4C",
							Default:     gusset == "T4C",
							Emoji:       ei.GetBotComponentEmoji("gusset_T4C"),
						},
						{
							Label:       "Gusset T3R",
							Description: "Rare",
							Value:       "T3R",
							Default:     gusset == "T3R",
							Emoji:       ei.GetBotComponentEmoji("gusset_T3R"),
						},
						{
							Label:       "Gusset T3C",
							Description: "Common",
							Value:       "T3C",
							Default:     gusset == "T3C",
							Emoji:       ei.GetBotComponentEmoji("gusset_T3C"),
						},
						{
							Label:       "Gusset T2E",
							Description: "Epic",
							Value:       "T2E",
							Default:     gusset == "T2E",
							Emoji:       ei.GetBotComponentEmoji("gusset_T2E"),
						},
						{
							Label:       "None",
							Description: "No Gusset equipped",
							Value:       "NONE",
							Default:     gusset == "NONE" || gusset == "",
						},
					},
				},
			},
		},
	}

	if !contractOnly && page == "ihr" {
		component = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "as_#DEFL-IHR#" + userID + "#" + temp,
						Placeholder: "Select your IHR Deflector...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options: []discordgo.SelectMenuOption{
							{Label: "IHR Deflector T4L", Description: "Legendary", Value: "T4L", Default: ihrDeflector == "T4L", Emoji: ei.GetBotComponentEmoji("defl_T4L")},
							{Label: "IHR Deflector T4E", Description: "Epic", Value: "T4E", Default: ihrDeflector == "T4E", Emoji: ei.GetBotComponentEmoji("defl_T4E")},
							{Label: "IHR Deflector T4R", Description: "Rare", Value: "T4R", Default: ihrDeflector == "T4R", Emoji: ei.GetBotComponentEmoji("defl_T4R")},
							{Label: "IHR Deflector T4C", Description: "Common", Value: "T4C", Default: ihrDeflector == "T4C", Emoji: ei.GetBotComponentEmoji("defl_T4C")},
							{Label: "IHR Deflector T3R", Description: "Rare", Value: "T3R", Default: ihrDeflector == "T3R", Emoji: ei.GetBotComponentEmoji("defl_T3R")},
							{Label: "IHR Deflector T3C", Description: "Common", Value: "T3C", Default: ihrDeflector == "T3C", Emoji: ei.GetBotComponentEmoji("defl_T3C")},
							{Label: "None", Description: "No IHR Deflector equipped", Value: "NONE", Default: ihrDeflector == "NONE" || ihrDeflector == ""},
						},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "as_#CHALICE#" + userID + "#" + temp,
						Placeholder: "Select your Chalice...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options: []discordgo.SelectMenuOption{
							{Label: "Chalice T4L", Description: "Legendary", Value: "T4L", Default: chalice == "T4L", Emoji: ei.GetBotComponentEmoji("chalice_T4L")},
							{Label: "Chalice T4E", Description: "Epic", Value: "T4E", Default: chalice == "T4E", Emoji: ei.GetBotComponentEmoji("chalice_T4E")},
							{Label: "Chalice T4C", Description: "Common", Value: "T4C", Default: chalice == "T4C", Emoji: ei.GetBotComponentEmoji("chalice_T4C")},
							{Label: "Chalice T3E", Description: "Epic", Value: "T3E", Default: chalice == "T3E", Emoji: ei.GetBotComponentEmoji("chalice_T3E")},
							{Label: "Chalice T3R", Description: "Rare", Value: "T3R", Default: chalice == "T3R", Emoji: ei.GetBotComponentEmoji("chalice_T3R")},
							{Label: "Chalice T3C", Description: "Common", Value: "T3C", Default: chalice == "T3C", Emoji: ei.GetBotComponentEmoji("chalice_T3C")},
							{Label: "Chalice T2E", Description: "Epic", Value: "T2E", Default: chalice == "T2E", Emoji: ei.GetBotComponentEmoji("chalice_T2E")},
							{Label: "Chalice T2C", Description: "Common", Value: "T2C", Default: chalice == "T2C", Emoji: ei.GetBotComponentEmoji("chalice_T2C")},
							{Label: "Chalice T1C", Description: "Common", Value: "T1C", Default: chalice == "T1C", Emoji: ei.GetBotComponentEmoji("chalice_T1C")},
							{Label: "None", Description: "No Chalice equipped", Value: "NONE", Default: chalice == "NONE" || chalice == ""},
						},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "as_#MONOCLE#" + userID + "#" + temp,
						Placeholder: "Select your Monocle...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options: []discordgo.SelectMenuOption{
							{Label: "Monocle T4L", Description: "Legendary", Value: "T4L", Default: monocle == "T4L", Emoji: ei.GetBotComponentEmoji("monocle_T4L")},
							{Label: "Monocle T4E", Description: "Epic", Value: "T4E", Default: monocle == "T4E", Emoji: ei.GetBotComponentEmoji("monocle_T4E")},
							{Label: "Monocle T4C", Description: "Common", Value: "T4C", Default: monocle == "T4C", Emoji: ei.GetBotComponentEmoji("monocle_T4C")},
							{Label: "Monocle T3C", Description: "Common", Value: "T3C", Default: monocle == "T3C", Emoji: ei.GetBotComponentEmoji("monocle_T3C")},
							{Label: "Monocle T2C", Description: "Common", Value: "T2C", Default: monocle == "T2C", Emoji: ei.GetBotComponentEmoji("monocle_T2C")},
							{Label: "Monocle T1C", Description: "Common", Value: "T1C", Default: monocle == "T1C", Emoji: ei.GetBotComponentEmoji("monocle_T1C")},
							{Label: "None", Description: "No Monocle equipped", Value: "NONE", Default: monocle == "NONE" || monocle == ""},
						},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "as_#SIAB#" + userID + "#" + temp,
						Placeholder: "Select your Ship In A Bottle...",
						MinValues:   &minValues,
						MaxValues:   1,
						Options: []discordgo.SelectMenuOption{
							{Label: "SIAB T4L", Description: "Legendary", Value: "T4L", Default: siab == "T4L", Emoji: ei.GetBotComponentEmoji("SIAB_T4L")},
							{Label: "SIAB T4E", Description: "Epic", Value: "T4E", Default: siab == "T4E", Emoji: ei.GetBotComponentEmoji("SIAB_T4E")},
							{Label: "SIAB T4R", Description: "Rare", Value: "T4R", Default: siab == "T4R", Emoji: ei.GetBotComponentEmoji("SIAB_T4R")},
							{Label: "SIAB T4C", Description: "Common", Value: "T4C", Default: siab == "T4C", Emoji: ei.GetBotComponentEmoji("SIAB_T4C")},
							{Label: "SIAB T3R", Description: "Rare", Value: "T3R", Default: siab == "T3R", Emoji: ei.GetBotComponentEmoji("SIAB_T3R")},
							{Label: "SIAB T3C", Description: "Common", Value: "T3C", Default: siab == "T3C", Emoji: ei.GetBotComponentEmoji("SIAB_T3C")},
							{Label: "SIAB T2C", Description: "Common", Value: "T2C", Default: siab == "T2C", Emoji: ei.GetBotComponentEmoji("SIAB_T2C")},
							{Label: "SIAB T1C", Description: "Common", Value: "T1C", Default: siab == "T1C", Emoji: ei.GetBotComponentEmoji("SIAB_T1C")},
							{Label: "None", Description: "No SIAB equipped", Value: "NONE", Default: siab == "NONE" || siab == ""},
						},
					},
				},
			},
		}
	}

	if !contractOnly && page == "collegg" {
		keys := make([]string, 0, len(ei.CustomEggMap))
		for k := range ei.CustomEggMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var eggOptions []discordgo.SelectMenuOption
		for _, k := range keys {
			eggOptions = append(eggOptions, discordgo.SelectMenuOption{
				Label:       ei.CustomEggMap[k].Name,
				Description: ei.CustomEggMap[k].Description,
				Value:       strings.ReplaceAll(ei.CustomEggMap[k].Name, " ", ""),
				Default:     strings.Contains(coll, ei.CustomEggMap[k].Name),
				Emoji:       ei.GetBotComponentEmoji("egg_" + ei.CustomEggMap[k].ID),
			})
		}

		component = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "as_#COLLEGG#" + userID + "#" + temp,
						Placeholder: "Select your Colleggtibles",
						MinValues:   &minV,
						MaxValues:   len(ei.CustomEggMap),
						Options:     eggOptions,
					},
				},
			},
		}
	}

	if !contractOnly {
		deliveryStyle := discordgo.SecondaryButton
		ihrStyle := discordgo.SecondaryButton
		colleggStyle := discordgo.SecondaryButton
		hasBackup := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id") != ""
		switch page {
		case "delivery":
			deliveryStyle = discordgo.PrimaryButton
		case "ihr":
			ihrStyle = discordgo.PrimaryButton
		default:
			colleggStyle = discordgo.PrimaryButton
		}

		navButtons := []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Delivery Set",
				Style:    deliveryStyle,
				CustomID: "as_#PAGEDEL#" + userID + "#" + temp,
			},
			discordgo.Button{
				Label:    "IHR Set",
				Style:    ihrStyle,
				CustomID: "as_#PAGEIHR#" + userID + "#" + temp,
			},
			discordgo.Button{
				Label:    "Colleggtibles",
				Style:    colleggStyle,
				CustomID: "as_#PAGECOL#" + userID + "#" + temp,
			},
		}

		if hasBackup {
			label := backupButtonLabel
			if label == "" {
				label = "Populate from Backup"
			}
			navButtons = append(navButtons, discordgo.Button{
				Label:    label,
				Style:    discordgo.SuccessButton,
				CustomID: "as_#POPBACKUP#" + userID + "#" + temp,
			})
		}

		component = append(component, discordgo.ActionsRow{Components: navButtons})
	}

	return builder.String(), component
}

// SlashArtifactsCommand creates a new slash command for setting Egg, Inc name
func SlashArtifactsCommand(cmd string) *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        cmd,
		Description: "Indicate best contract artifacts you have.",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
			discordgo.ApplicationIntegrationUserInstall,
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

	contractOnly := false

	str, comp := getArtifactsComponents(userID, i.ChannelID, contractOnly, "delivery")

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
		log.Println("InteractionRespond: ", err)
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

	setValue := len(data.Values) != 0
	page := getArtifactsPageFromContent(i.Message.Content)
	switch cmd {
	case "pageihr", "defl-ihr", "chalice", "monocle", "siab":
		page = "ihr"
	case "pagecol", "collegg":
		page = "collegg"
	}

	//if override == "PERM" {
	statusPrefix := ""
	switch cmd {
	case "popbackup":
		loadingStr, loadingComp := getArtifactsComponents(userID, i.ChannelID, false, page, "Loading Backup...")
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content:    &loadingStr,
			Components: &loadingComp,
		})
		status, err := populateArtifactsFromBackup(s, userID)
		if err != nil {
			log.Printf("populateArtifactsFromBackup: %v", err)
		}
		statusPrefix = status
	case "defl", "metr", "comp", "guss", "defl-ihr", "chalice", "monocle", "siab":
		if setValue {
			farmerstate.SetMiscSettingString(userID, cmd, data.Values[0])
		} else {
			farmerstate.SetMiscSettingString(userID, cmd, "") // Clear the value
		}
	case "collegg":
		farmerstate.SetMiscSettingString(userID, cmd, strings.Join(data.Values, ","))
	}

	// Redraw the artifact list
	str, comp := getArtifactsComponents(userID, i.ChannelID, false, page, "Backup Loaded")
	if statusPrefix != "" {
		str = statusPrefix + "\n" + str
	}

	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    &str,
		Components: &comp,
	})
	if err != nil {
		log.Println("InteractionResponseEdit: ", err)
	}

	//} else {
	contract := FindContract(i.ChannelID)
	if contract != nil {
		if UserInContract(contract, userID) {
			if cmd != "defl" && cmd != "metr" && cmd != "comp" && cmd != "guss" && cmd != "defl-ihr" && cmd != "chalice" && cmd != "monocle" && cmd != "siab" {
				goto done
			}
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
			case "defl-ihr":
				prefix = "ID-"
			case "chalice":
				prefix = "CH-"
			case "monocle":
				prefix = "MO-"
			case "siab":
				prefix = "SIAB-"
			}
			var newArtifact *ei.Artifact
			if len(data.Values) == 0 {
				newArtifact = ei.GetArtifactByKey(prefix + "NONE")
			} else {
				newArtifact = ei.GetArtifactByKey(prefix + data.Values[0])
			}

			// Check if artifact was found in map
			if newArtifact != nil {
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

				refreshBoostListMessage(s, contract, false)
			}

		}
	}

done:
	//}
	_, _ = s.FollowupMessageCreate(i.Interaction, true,
		&discordgo.WebhookParams{
			//Content: "",
			//Flags: discordgo.MessageFlagsEphemeral,
		})

}
