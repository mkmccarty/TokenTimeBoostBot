package boost

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/ei"
	"github.com/mkmccarty/TokenTimeBoostBot/src/guildstate"
	"google.golang.org/genai"
)

var randomThingNames []string = []string{
	// Farm Animals (50 names)
	"Cow", "Pig", "Chicken", "Sheep", "Goat", "Duck", "Horse", "Donkey", "Turkey", "Goose",
	"Rabbit", "Llama", "Alpaca", "Guinea Pig", "Yak", "Mule", "Calf", "Lamb", "Ewe", "Ram",
	"Rooster", "Hen", "Drakes", "Gander", "Hog", "Bison", "Mare", "Stallion", "Foal", "Kid",
	"Shoat", "Piglet", "Cygnet", "Gosling", "Duckling", "Poult", "Pullet", "Capon", "Gelding", "Filly",
	"Colt", "Wether", "Boar", "Sow", "Gilt", "Barrow", "Steer", "Heifer", "Bull", "Dairy Cow",

	// Farm Crops (50 names)
	"Corn", "Wheat", "Rice", "Barley", "Oats", "Soybean", "Potato", "Tomato", "Carrot", "Onion",
	"Lettuce", "Cabbage", "Broccoli", "Spinach", "Pea", "Bean", "Cucumber", "Pumpkin", "Squash", "Zucchini",
	"Strawberry", "Blueberry", "Raspberry", "Apple", "Orange", "Grape", "Peach", "Pear", "Cherry", "Almond",
	"Walnut", "Sunflower", "Canola", "Rye", "Sorghum", "Millet", "Lentil", "Chickpea", "Flax", "Quinoa",
	"Asparagus", "Beet", "Celery", "Kale", "Radish", "Sweet Potato", "Turnip", "Artichoke", "Bell Pepper", "Eggplant",

	// Flowers (51 names)
	"Rose", "Tulip", "Sunflower", "Lily", "Daisy", "Orchid", "Carnation", "Chrysanthemum", "Daffodil", "Iris",
	"Peony", "Lavender", "Poppy", "Violet", "Begonia", "Geranium", "Petunia", "Zinnia", "Marigold", "Snapdragon",
	"Hydrangea", "Freesia", "Gladiolus", "Hibiscus", "Jasmine", "Lotus", "Magnolia", "Pansy", "Primrose", "Ranunculus",
	"Sweet Pea", "Thistle", "Water Lily", "Amaryllis", "Anemone", "Aster", "Crocus", "Dahlia", "Delphinium", "Foxglove",
	"Gardenia", "Honeysuckle", "Impatiens", "Lilac", "Morning Glory", "Nasturtium", "Oleander", "Periwinkle", "Phlox", "Rhododendron",
	"Bluebonnet",

	// Mushroom Plants (49 names)
	"Portobello", "Shiitake", "Button Mushroom", "Cremini", "Oyster Mushroom", "Chanterelle", "Morel", "Truffle", "Enoki", "Maitake",
	"Lion's Mane", "Reishi", "Turkey Tail", "Puffball", "Boletus", "King Trumpet", "Shiro", "Porcini", "Amanita", "Psilocybe",
	"Tinder Fungus", "Wood Ear", "Artist's Conk", "Cinnabar Polypore", "Coral Fungi", "Earthstar", "Fairy Ring", "Fly Agaric", "Ink Cap", "Jack-o'-lantern",
	"Lawyer's Wig", "Parasol", "Shaggy Mane", "Death Cap", "Destroying Angel", "Indigo Milk Cap", "Veiled Lady", "Paddy Straw", "Blewit",
	"Bay Bolete", "Chicken of the Woods", "Giant Puffball", "Honey Fungus", "Jelly Fungus", "Velvet Shank", "Winter Fungus", "Bear's Head Tooth", "Scaly Hedgehog", "Elm Oyster",

	// Trees (60 names)
	"Oak", "Maple", "Pine", "Birch", "Willow", "Elm", "Aspen", "Cedar", "Spruce", "Fir",
	"Redwood", "Sequoia", "Cypress", "Cherry Blossom", "Dogwood", "Hawthorn", "Juniper", "Larch", "Poplar", "Sycamore",
	"Ash", "Beech", "Chestnut", "Ginkgo", "Linden", "Magnolia Tree", "Palm", "Pecan", "Sassafras", "Sweetgum",
	"Walnut Tree", "White Pine", "Red Maple", "Silver Maple", "Sugar Maple", "Eastern White Pine", "Scots Pine", "Norway Spruce", "Blue Spruce", "Bald Cypress",
	"Weeping Willow", "Paper Birch", "River Birch", "American Elm", "Leyland Cypress", "Dawn Redwood", "Bristlecone Pine", "Quaking Aspen", "Black Willow", "Box Elder",
	"Noble Fir", "Douglas Fir", "Western Red Cedar", "Eastern Hemlock", "Black Cherry", "Black Locust", "Honey Locust", "Catalpa", "Tulip Tree", "Redbud",

	// Zoo Animals (50 names)
	"Lion", "Tiger", "Elephant", "Giraffe", "Zebra", "Monkey", "Bear", "Kangaroo", "Panda", "Penguin",
	"Rhino", "Hippo", "Wolf", "Fox", "Gorilla", "Chimpanzee", "Leopard", "Cheetah", "Crocodile", "Alligator",
	"Snake", "Eagle", "Owl", "Flamingo", "Ostrich", "Camel", "Koala", "Sloth", "Meerkat", "Fennec Fox",
	"Red Panda", "Tapir", "Lemur", "Puma", "Jaguar", "Cougar", "Bison", "Warthog", "Orangutan", "Gibbon",
	"Baboon", "Mandrill", "Anteater", "Armadillo", "Okapi", "Platypus", "Komodo Dragon", "Gila Monster", "Kookaburra", "Toucan",

	// Mythical Monsters & Legendary Creatures (50 names)
	"Dragon", "Hydra", "Phoenix", "Griffin", "Kraken", "Chimera", "Basilisk", "Sphinx", "Minotaur", "Cerberus",
	"Loch Ness Monster", "Sasquatch", "Bigfoot", "Godzilla", "King Kong", "Mothman", "Chupacabra", "Jersey Devil", "Yeti", "Abominable Snowman",
	"Banshee", "Wendigo", "Valkyrie", "Leviathan", "Behemoth", "Manticore", "Cockatrice", "Wyvern", "Pegasus", "Unicorn",
	"Cyclops", "Medusa", "Gorgon", "Harpy", "Siren", "Dullahan", "Kelpie", "Selkie", "Roc", "Thunderbird",
	"Quetzalcoatl", "Jormungandr", "Fenrir", "Sleipnir", "Djinn", "Ifrit", "Salamander", "Undine", "Sylph", "Gnome",

	// Admin Choice Names (6 names)
	"TBone Alt",
	"Rumpus Mugwumpus", "Giga What?", "bussin fr fr no cap",
	"Aliens among us", "Polo Locos",
	"Toe Socks", "Meme Stock",
}

var nameMutex sync.Mutex

const googleModel = "gemini-2.5-flash-lite"

var potatoFallbackRoleNames = []string{
	"Potato Crew",
	"Spud Squad",
	"Tater Team",
	"Potato Patrol",
	"Golden Potato",
	"Speculative Tater",
	"Tater Tot-alitarians",
	"Potato Posse",
	"The Mash Mob",
	"The Waffle Fry Works",
	"The Au Gratin Alliance,",
	"The Russet Rangers",
}

func isPotatoPreferredUser(guildID string, userID string) bool {
	if userID == "" {
		return false
	}

	taters := strings.TrimSpace(guildstate.GetGuildSettingString(guildID, "taters"))
	if taters == "" {
		taters = strings.TrimSpace(guildstate.GetGuildSettingString("DEFAULT", "taters"))
	}
	if taters == "" {
		return false
	}

	for _, id := range strings.Split(taters, ",") {
		if strings.TrimSpace(id) == userID {
			return true
		}
	}

	return false
}

func contractHasOtherPotatoPreferredUser(contract *Contract, guildID string, excludeUserID string) bool {
	if contract == nil {
		return false
	}
	for boosterID := range contract.Boosters {
		if boosterID == excludeUserID {
			continue
		}
		if isPotatoPreferredUser(guildID, boosterID) {
			return true
		}
	}
	return false
}

func roleNameReferencesPotato(roleName string) bool {
	name := strings.ToLower(roleName)
	return strings.Contains(name, "potato") || strings.Contains(name, "spud") || strings.Contains(name, "tater")
}

func getPotatoContractRoleName(contract *Contract) string {
	prompt := "Egg Inc contract"
	if contract != nil {
		switch {
		case strings.TrimSpace(contract.Description) != "":
			prompt = contract.Description
		case strings.TrimSpace(contract.Name) != "":
			prompt = contract.Name
		case strings.TrimSpace(contract.ContractID) != "":
			prompt = contract.ContractID
		}
	}

	names := fetchContractTeamNames(prompt+" Include a potato reference in the team name.", 1)
	if len(names) > 0 {
		name := strings.TrimSpace(names[0])
		if name != "" {
			if !roleNameReferencesPotato(name) {
				name = "Potato " + name
			}
			return name
		}
	}

	return potatoFallbackRoleNames[rand.IntN(len(potatoFallbackRoleNames))]
}

func uniqueRoleName(existingRoles []string, preferredName string) string {
	if preferredName == "" {
		preferredName = "Potato Crew"
	}
	if !slices.Contains(existingRoles, preferredName) {
		return preferredName
	}
	teamPreferred := "Team " + preferredName
	if !slices.Contains(existingRoles, teamPreferred) {
		return teamPreferred
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s %d", preferredName, i)
		if !slices.Contains(existingRoles, candidate) {
			return candidate
		}
	}
	return fmt.Sprintf("%s %d", preferredName, rand.IntN(1000)+1000)
}

func ensurePotatoTeamRoleForUser(s *discordgo.Session, contract *Contract, userID string) {
	if s == nil || contract == nil {
		return
	}

	roleRenamed := false

	for _, loc := range contract.Location {
		if loc == nil || loc.GuildContractRole.ID == "" {
			continue
		}
		if contractHasOtherPotatoPreferredUser(contract, loc.GuildID, userID) {
			continue
		}
		if !isPotatoPreferredUser(loc.GuildID, userID) {
			continue
		}
		if roleNameReferencesPotato(loc.GuildContractRole.Name) {
			continue
		}

		desiredName := getPotatoContractRoleName(contract)

		roles, err := s.GuildRoles(loc.GuildID)
		if err != nil {
			log.Printf("ensurePotatoTeamRoleForUser: failed to list roles for guild %s: %v", loc.GuildID, err)
			continue
		}
		existingRoles := make([]string, 0, len(roles))
		for _, role := range roles {
			existingRoles = append(existingRoles, role.Name)
		}
		newName := uniqueRoleName(existingRoles, desiredName)

		updatedRole, err := s.GuildRoleEdit(loc.GuildID, loc.GuildContractRole.ID, &discordgo.RoleParams{
			Name: newName,
		})
		if err != nil {
			log.Printf("ensurePotatoTeamRoleForUser: failed to rename role %s (%s): %v", loc.GuildContractRole.Name, loc.GuildContractRole.ID, err)
			continue
		}

		if updatedRole != nil {
			contract.mutex.Lock()
			loc.GuildContractRole = *updatedRole
			loc.RoleManagedByBot = true
			loc.RoleMention = loc.GuildContractRole.Mention()
			contract.mutex.Unlock()
			roleRenamed = true
		}
		log.Printf("Renamed contract team role to potato-themed name %q for user %s", loc.GuildContractRole.Name, userID)
	}

	if roleRenamed {
		refreshBoostListMessage(s, contract, contract.RegisteredNum == contract.CoopSize)
	}
}

func ensurePotatoTeamRoleForUserAsync(s *discordgo.Session, contract *Contract, userID string) {
	if s == nil || contract == nil || userID == "" {
		return
	}

	needsPotatoRole := false
	for _, loc := range contract.Location {
		if loc == nil {
			continue
		}
		if contractHasOtherPotatoPreferredUser(contract, loc.GuildID, userID) {
			continue
		}
		if isPotatoPreferredUser(loc.GuildID, userID) {
			needsPotatoRole = true
			break
		}
	}
	if !needsPotatoRole {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("ensurePotatoTeamRoleForUserAsync panic for user %s: %v", userID, r)
			}
		}()
		ensurePotatoTeamRoleForUser(s, contract, userID)
	}()
}

// getContractRoleName generates a thematic role name for the given contract ID
func getContractRoleName(contractID string) string {
	roleNames := randomThingNames

	for _, c := range ei.EggIncContracts {
		if c.ID == contractID {
			if len(c.TeamNames) > 0 {
				roleNames = c.TeamNames
			}
			break
		}
	}

	// Get existing roles in a default guild (since we don't have context here)
	// For naming purposes, we'll just use the first available name
	var existingRoles []string
	// In the context of renaming, we don't need to check all guilds -
	// we just need a thematic name that's different from the current one

	tryCount := 0
	prefix := ""
	var teamName string

	// Create a list of unused role names (just pick from the theme list)
	var unusedRoleNames []string
	for _, name := range roleNames {
		if !slices.Contains(existingRoles, fmt.Sprintf("%s%s", prefix, name)) {
			unusedRoleNames = append(unusedRoleNames, name)
		}
	}
	rand.Shuffle(len(unusedRoleNames), func(i, j int) {
		unusedRoleNames[i], unusedRoleNames[j] = unusedRoleNames[j], unusedRoleNames[i]
	})

	if len(unusedRoleNames) == 0 {
		// All names are taken; fall back to a generated team name
		teamName = bottools.GetRandomName(0)
		prefix = "Team "
	} else {
		lastChance := false
		for {
			name := unusedRoleNames[tryCount]
			if !slices.Contains(existingRoles, prefix+name) {
				// Found an unused name
				teamName = name
				break
			}
			tryCount++
			if tryCount == len(unusedRoleNames) {
				if lastChance {
					break
				}
				prefix = "Team "
				// Filter out names that are already taken with the new prefix
				filteredNames := make([]string, 0, len(unusedRoleNames))
				for _, name := range unusedRoleNames {
					if !slices.Contains(existingRoles, prefix+name) {
						filteredNames = append(filteredNames, name)
					}
				}
				unusedRoleNames = filteredNames
				rand.Shuffle(len(unusedRoleNames), func(i, j int) {
					unusedRoleNames[i], unusedRoleNames[j] = unusedRoleNames[j], unusedRoleNames[i]
				})
				if len(unusedRoleNames) == 0 {
					break
				}
				tryCount = 0
				lastChance = true
			}
		}
		if teamName == "" {
			teamName = bottools.GetRandomName(0)
		}
	}

	return fmt.Sprintf("%s%s", prefix, teamName)
}

// Return a new contract role for the given guild
func getContractRole(s *discordgo.Session, guildID string, contract *Contract) error {
	var role *discordgo.Role
	var err error
	var teamName string
	nameMutex.Lock()
	defer nameMutex.Unlock()

	roles, err := s.GuildRoles(guildID)
	if err != nil {
		return err
	}

	roleNames := randomThingNames

	for _, c := range ei.EggIncContracts {
		if c.ID == contract.ContractID {
			if len(c.TeamNames) > 0 {
				roleNames = c.TeamNames
			}
			break
		}
	}

	// remove anything from roles where the name does not start with "Team"
	var existingRoles []string
	for _, role := range roles {
		existingRoles = append(existingRoles, role.Name)
	}

	tryCount := 0
	prefix := ""

	// Create a list of unused role names
	var unusedRoleNames []string
	for _, name := range roleNames {
		if !slices.Contains(existingRoles, fmt.Sprintf("%s%s", prefix, name)) {
			unusedRoleNames = append(unusedRoleNames, name)
		}
	}
	rand.Shuffle(len(unusedRoleNames), func(i, j int) {
		unusedRoleNames[i], unusedRoleNames[j] = unusedRoleNames[j], unusedRoleNames[i]
	})

	if len(unusedRoleNames) == 0 {
		// All names are taken; fall back to a generated team name
		teamName = bottools.GetRandomName(0)
		prefix = "Team "
	} else {
		lastChance := false
		for {
			name := unusedRoleNames[tryCount]
			if !slices.Contains(existingRoles, prefix+name) {
				// Found an unused name
				teamName = name
				break
			}
			tryCount++
			if tryCount == len(unusedRoleNames) {
				if lastChance {
					break
				}
				prefix = "Team "
				// Filter out names that are already taken with the new prefix
				filteredNames := make([]string, 0, len(unusedRoleNames))
				for _, name := range unusedRoleNames {
					if !slices.Contains(existingRoles, prefix+name) {
						filteredNames = append(filteredNames, name)
					}
				}
				unusedRoleNames = filteredNames
				rand.Shuffle(len(unusedRoleNames), func(i, j int) {
					unusedRoleNames[i], unusedRoleNames[j] = unusedRoleNames[j], unusedRoleNames[i]
				})
				if len(unusedRoleNames) == 0 {
					break
				}
				tryCount = 0
				lastChance = true
			}
		}
		if teamName == "" {
			teamName = bottools.GetRandomName(0)
		}
	}

	mentionable := true
	role, err = s.GuildRoleCreate(guildID, &discordgo.RoleParams{
		Name:        fmt.Sprintf("%s%s", prefix, teamName),
		Mentionable: &mentionable,
	})
	if err != nil {
		return err
	}

	for _, loc := range contract.Location {
		if loc.GuildID == guildID {

			loc.GuildContractRole = *role
			loc.RoleManagedByBot = true
			return nil
		}
	}

	return fmt.Errorf("no contract role found")
}

// fetchContractTeamNames fetches team names from the Gemini API based on contract description
func fetchContractTeamNames(prompt string, quantity int) []string {
	if config.GoogleAPIKey == "" {
		return nil
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "My Egg Inc contract today wants \"%s\". Return a list of %d team names in a comma separated list with no other context.", prompt, quantity)

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  config.GoogleAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Print(err)
		return nil
	}

	resp, err := client.Models.GenerateContent(ctx, googleModel, genai.Text(builder.String()), nil)
	if err != nil {
		log.Print(err)
		return nil
	}

	var respStr strings.Builder
	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			fmt.Fprint(&respStr, part.Text)
		}
	}

	text := strings.ReplaceAll(respStr.String(), "widget", "token")

	parts := strings.Split(text, ",")
	var names []string
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name != "" {
			names = append(names, name)
		}
	}

	return names
}

// IsRoleCreatedByBot checks if a role name was created by the bot by verifying if it matches
// one of the role names from the bot's source lists (randomThingNames or contract team names)
func IsRoleCreatedByBot(roleName string) bool {
	// Check against the default randomThingNames list
	if slices.Contains(randomThingNames, roleName) {
		return true
	}

	// Check against team names from all known contracts
	for _, contract := range ei.EggIncContracts {
		if len(contract.TeamNames) > 0 && slices.Contains(contract.TeamNames, roleName) {
			return true
		}
	}

	// Check if the role name is a "Team" prefixed version of any known role name
	if strings.HasPrefix(roleName, "Team ") {
		unprefixedName := strings.TrimPrefix(roleName, "Team ")

		// Check against randomThingNames
		if slices.Contains(randomThingNames, unprefixedName) {
			return true
		}

		// Check against contract team names
		for _, contract := range ei.EggIncContracts {
			if len(contract.TeamNames) > 0 && slices.Contains(contract.TeamNames, unprefixedName) {
				return true
			}
		}
	}

	return false
}
