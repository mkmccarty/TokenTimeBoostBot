package boost

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"

	"github.com/bwmarrin/discordgo"
)

type shipData struct {
	Name     string   `json:"Name"`
	Art      string   `json:"Art"`
	ArtDev   string   `json:"ArtDev"`
	Duration []string `json:"Duration"`
}

type missionData struct {
	Ships []shipData
}

const missionJSON = `{"ships":[
	{"name": "Chicken One","art":"<:chicken1:1280045945974951949>","artDev":"<:chicken1:1280390988824576061>","duration":["20m","1h","2h"]},
	{"name": "Chicken Nine","art":"<:chicken9:1280045842442616902>","artDev":"<:chicken9:1280390884575154226>","duration":["30m","1h","3h"]},
	{"name": "Chicken Heavy","art":"<:chickenheavy:1280045643922018315>","artDev":"<:chickenheavy:1280390782783590473>","duration":["45m","1h30m","4h"]},
	{"name": "BCR","art":"<:bcr:1280045542495228008>","artDev":"<:bcr:1280390686461661275>","duration":["1h30m","4h","8h"]},
	{"name": "Quintillion Chicken","art":"<:milleniumchicken:1280045411444326400>","artDev":"<:milleniumchicken:1280390575178383386>","duration":["3h","6h","12h"]},
	{"name": "Cornish-Hen Corvette","art":"<:corellihencorvette:1280045137518657536>","artDev":"<:corellihencorvette:1280390458983452742>","duration":["4h","12h","1d"]},
	{"name": "Galeggtica","art":"<:galeggtica:1280045010917527593>","artDev":"<:galeggtica:1280390347825872916>","duration":["6h","16h","1d6h"]},
	{"name": "Defihent","art":"<:defihent:1280044758001258577>","artDev":"<:defihent:1280390249943666739>","duration":["8h","1d","2d"]},
	{"name": "Voyegger","art":"<:voyegger:1280041822416273420>","artDev":"<:voyegger:1280390114094354472>","duration":["12h","1d12h","3d"]},
	{"name": "Henerprise","art":"<:henerprise:1280038539328749609>","artDev":"<:henerprise:1280390026487664704>","duration":["1d","2d","4d"]},
	{"name": "Atreggies Henliner","art":"<:atreggies:1280038674398183464>","artDev":"<:atreggies:1280389911509340240>","duration":["2d","3d","4d"]}
	]}`

var missionArt missionData

func init() {
	_ = json.Unmarshal([]byte(missionJSON), &missionArt)
}

// GetSlashVirtueCommand returns the command for the /launch-helper command
func GetSlashVirtueCommand(cmd string) *discordgo.ApplicationCommand {
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
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "reset",
				Description: "Reset stored EI number",
				Required:    false,
			},
		},
	}
}

// HandleVirtue handles the /virtue command
func HandleVirtue(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	eiID := farmerstate.GetMiscSettingString(userID, "encrypted_ei_id")

	Virtue(s, i, percent, eiID, true)
}

// Virtue processes the virtue command
func Virtue(s *discordgo.Session, i *discordgo.InteractionCreate, percent int, eiID string, okayToSave bool) {
	// Get the Egg Inc ID from the stored settings
	eggIncID := ""
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
		RequestEggIncIDModal(s, i, fmt.Sprintf("virtue#%d", 0))
		return
	}

	// Quick reply to buy us some time
	flags := discordgo.MessageFlagsIsComponentsV2
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Processing request...",
			Flags:   flags,
		},
	})

	userID := bottools.GetInteractionUserID(i)

	backup, _ := ei.GetFirstContactFromAPI(s, eggIncID, userID, okayToSave)
	str := ""
	farm := backup.GetFarms()[0]
	if farm != nil {
		farmType := farm.GetFarmType()
		if farmType == ei.FarmType_HOME {
			eggType := farm.GetEggType()
			if eggType >= ei.Egg_CURIOSITY && eggType <= ei.Egg_KINDNESS {
				str = printVirtue(backup)
				//str += fmt.Sprintf("\n-# Updated `<t:%d:f>`", bottools.NowUnix())
			}
		}
	}
	if str == "" {
		str = "Your home farm isn't currently producing Eggs of Virtue. Switch to an Egg of Virtue on your home farm to see this information."
	}
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Flags: flags,
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{Content: str},
		},
	})

}

func printVirtue(backup *ei.Backup) string {
	farm := backup.GetFarms()[0]
	eggType := farm.GetEggType()
	virtue := backup.GetVirtue()
	//pe := backup.GetGame().GetEggsOfProphecy()
	//se := backup.GetGame().GetSoulEggs()

	builder := strings.Builder{}
	if virtue == nil {
		log.Print("No virtue backup data found in Egg Inc API response")
		return builder.String()
	}
	fmt.Fprintf(&builder, "# Eggs Of Virtue\n")
	fmt.Fprintf(&builder, "Shift Count: %d\n", virtue.GetShiftCount())
	fmt.Fprintf(&builder, "Resets: %d\n", virtue.GetResets())
	fmt.Fprintf(&builder, "Inventory Score %.0f\n", virtue.GetAfx().GetInventoryScore())

	virtueEggs := []string{"CURIOSITY", "INTEGRITY", "HUMILITY", "RESILIENCE", "KINDNESS"}

	for i, egg := range virtueEggs {
		eov := virtue.GetEovEarned()[i] // Assuming Eggs is the correct field for accessing egg virtues
		delivered := virtue.GetEggsDelivered()[i]
		nextTier, eovPending, _ := getNextTierAndIndex(delivered)
		selected := ""
		if eggType == ei.Egg(int(ei.Egg_CURIOSITY)+i) {
			selected = " (selected)"
		}

		fmt.Fprintf(&builder, "%s %d (%d)  |  🚚: %s |  %s at %s%s\n",
			ei.GetBotEmojiMarkdown("egg_"+strings.ToLower(egg)),
			eov,
			eovPending-int(eov),
			ei.FormatEIValue(delivered, map[string]interface{}{"decimals": 1, "trim": true}),
			ei.GetBotEmojiMarkdown("egg_truth"),
			ei.FormatEIValue(nextTier, map[string]interface{}{"decimals": 1, "trim": true}),
			selected)
	}

	fmt.Fprintf(&builder, "### Missions on %s\n", ei.GetBotEmojiMarkdown("egg_humility"))
	artifacts := backup.GetArtifactsDb()
	missions := artifacts.GetMissionInfos()
	for _, mission := range missions {
		missionType := mission.GetType()
		if missionType == ei.MissionInfo_VIRTUE {
			shipType := mission.GetShip()
			craft := missionArt.Ships[shipType]
			art := craft.Art
			if config.IsDevBot() {
				art = craft.ArtDev
			}
			timeRemaining := mission.GetSecondsRemaining()
			fmt.Fprintf(&builder, "%s <t:%d:R> \n", art, time.Now().Unix()+int64(timeRemaining))
		}
	}
	// Line for fuel
	fuels := virtue.GetAfx().GetTankFuels()
	fuels = fuels[len(fuels)-5:]
	builder.WriteString("\n⛽️ ")
	for i, fuel := range fuels {
		fmt.Fprintf(&builder, " %s:%s",
			ei.GetBotEmojiMarkdown("egg_"+strings.ToLower(virtueEggs[i])),
			ei.FormatEIValue(fuel, map[string]interface{}{"decimals": 1, "trim": true}))
	}

	return builder.String()
}

// tierValues is a slice containing all known tiers in ascending order.
var tierValues = []float64{
	50_000_000,
	1_000_000_000,
	10_000_000_000,
	70_000_000_000,
	500_000_000_000,
	2_000_000_000_000,
	// Needs verification below this point
	7_000_000_000_000,
	20_000_000_000_000,
	60_000_000_000_000,
	150_000_000_000_000,
	500_000_000_000_000,
	1_500_000_000_000_000,
	4_000_000_000_000_000,
	10_000_000_000_000_000,
	25_000_000_000_000_000,
	50_000_000_000_000_000,
	100_000_000_000_000_000,
	200_000_000_000_000_000,
	400_000_000_000_000_000,
	600_000_000_000_000_000,
	800_000_000_000_000_000,
	1_000_000_000_000_000_000,
}

// getNextTierAndIndex finds the next tier for a given value.
// It returns the next tier's value, the index of the tier just passed, and an error.
func getNextTierAndIndex(currentValue float64) (float64, int, error) {
	// If the value is less than the first tier, the first tier is the next one.
	if currentValue < tierValues[0] {
		return tierValues[0], 0, nil // -1 indicates no tier has been passed yet.
	}

	// Iterate through the ordered tiers to find the correct position for the currentValue.
	for i, tier := range tierValues {
		if currentValue < tier {
			// The current value is less than this tier, so this is the next tier.
			// The previous index (i-1) is the one the user has reached.
			return tier, i, nil
		}
	}

	// If we reach here loop, adding 200_000_000_000_000_000 to the last tier until we find a tier greater than currentValue.
	lastTier := tierValues[len(tierValues)-1]
	increment := 200_000_000_000_000_000.0
	for {
		lastTier += increment
		if currentValue < lastTier {
			return lastTier, len(tierValues) - 1, nil // Return the last known index.
		}
	}

	// If the loop completes, it means the currentValue is greater than or equal to the last tier.
	// We return 0, the last known index, and an error.
	//return 0, len(tierValues), fmt.Errorf("current value is beyond the last known tier")
}

/*


type Research struct {
	ID       string
	Name     string
	MaxLevel int
	PerLevel float64
}

type ResearchInstance struct {
	Research
	Level int
}

type Farm interface {
	Researches([]Research) []ResearchInstance
}

const baseSoulEggBonus = 0.1

var soulEggBonusRelevantResearches = []Research{
	{
		ID:       "soul_eggs",
		Name:     "Soul Food",
		MaxLevel: 140,
		PerLevel: 0.01,
	},
}

const baseProphecyEggBonus = 0.05

var prophecyEggBonusRelevantResearches = []Research{
	{
		ID:       "prophecy_bonus",
		Name:     "Prophecy Bonus",
		MaxLevel: 5,
		PerLevel: 0.01,
	},
}

func soulEggBonusResearches(farm Farm) []ResearchInstance {
	return farm.Researches(soulEggBonusRelevantResearches)
};
}

export function prophecyEggBonusResearches(farm: Farm): ResearchInstance[] {
  return farm.researches(prophecyEggBonusRelevantResearches);
}

export function bareSoulEggBonus(farm: Farm, researches: ResearchInstance[]): number {
  return baseSoulEggBonus + researches.reduce((effect, r) => effect + r.perLevel * r.level, 0);
}

function soulEggBonus(farm: Farm, researches: ResearchInstance[]): number {
  return (
    baseSoulEggBonus +
    researches.reduce((effect, r) => effect + r.perLevel * r.level, 0) +
    farm.artifactSet.soulEggBonus
  );
}

export function bareProphecyEggBonus(farm: Farm, researches: ResearchInstance[]): number {
  return baseProphecyEggBonus + researches.reduce((effect, r) => effect + r.perLevel * r.level, 0);
}

function prophecyEggBonus(farm: Farm, researches: ResearchInstance[]): number {
  return (
    baseProphecyEggBonus +
    researches.reduce((effect, r) => effect + r.perLevel * r.level, 0) +
    farm.artifactSet.prophecyEggBonus
  );
}

func getEarningsBonus(*ei.Backup_Farm farm, soulEggBonusRelevantResearches: ResearchInstance[], prophecyEggBonusRelevantResearches: ResearchInstance[]): float64 {

  const soulEggsCount = farm.progress.soulEggsD || 0;
  const prophecyEggsCount = getNumProphecyEggs(farm.backup);



}


  farm: Farm,
  soulEggBonusResearches: ResearchInstance[],
  prophecyEggBonusResearches: ResearchInstance[]
): number {
  const soulEggsCount = farm.progress.soulEggsD || 0;
  const prophecyEggsCount = getNumProphecyEggs(farm.backup);
  return (
    soulEggsCount *
    soulEggBonus(farm, soulEggBonusResearches) *
    (1 + prophecyEggBonus(farm, prophecyEggBonusResearches)) ** prophecyEggsCount
  );
}*/
