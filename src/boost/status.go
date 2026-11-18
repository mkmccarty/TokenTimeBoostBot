package boost

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashStatusCommand returns the slash command definition for /status.
func GetSlashStatusCommand(cmd string) *dc.Command {
	command := dc.Command{
		Name:        cmd,
		Description: "Show your account info, linked alts, act-as links, and active contracts.",
		Contexts: []dc.InteractionContext{
			dc.ContextGuild,
			dc.ContextBotDM,
			dc.ContextPrivateChannel,
		},
		IntegrationTypes: []dc.IntegrationType{
			dc.IntegrationGuildInstall,
			dc.IntegrationUserInstall,
		},
	}
	return &command
}

// HandleStatusCommand responds ephemerally with account info, linked alts, act-as links, and active contracts.
func HandleStatusCommand(e *dc.CommandEvent) {
	if e == nil {
		return
	}
	content := buildStatusMessage(e.UserID(), e.ChannelID())
	_ = e.Respond(dc.Message{
		Content:   content,
		Ephemeral: true,
	})
}

type statusContractEntry struct {
	ContractID string
	CoopID     string
	State      int
	Links      []string
	Accounts   []string
}

func buildStatusMessage(userID, channelID string) string {
	var b strings.Builder

	b.WriteString("## Account Status\n")
	ign := strings.TrimSpace(farmerstate.GetMiscSettingString(userID, "ei_ign"))
	if ign == "" {
		ign = "Not set"
	}
	eiRegistered := "Not registered"
	if farmerstate.GetMiscSettingString(userID, "encrypted_ei_id") != "" {
		eiRegistered = "Registered"
	}
	fmt.Fprintf(&b, "- **Account:** <@%s>\n", userID)
	fmt.Fprintf(&b, "- **EI IGN:** %s\n", ign)
	fmt.Fprintf(&b, "- **EI ID:** %s\n", eiRegistered)

	controllerID := strings.TrimSpace(farmerstate.GetMiscSettingString(userID, "AltController"))
	if controllerID != "" {
		fmt.Fprintf(&b, "- **Alt Controller:** %s\n", formatStatusAccountLabel(controllerID))
	}

	if channelID != "" {
		effectiveID, expiresAt, isActAs := farmerstate.GetEffectiveUserIDAndExpiry(userID, channelID)
		if isActAs {
			expiryStr := fmt.Sprintf("<t:%d:R>", expiresAt.Unix())
			if farmerstate.IsActAsForever(expiresAt) {
				expiryStr = "forever"
			}
			fmt.Fprintf(&b, "- **Acting As (this channel):** %s (%s)\n", formatStatusAccountLabel(effectiveID), expiryStr)
		}
	}

	linkedAlts := farmerstate.GetAltControllerByMiscString("AltController", userID)
	slices.Sort(linkedAlts)

	b.WriteString("\n### Linked Alts (`AltController`)\n")
	if len(linkedAlts) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, altID := range linkedAlts {
			b.WriteString("- " + formatStatusAltDetail(altID) + "\n")
		}
	}

	actAsAlts := farmerstate.GetActAsLinks(userID)
	actAsMains := farmerstate.GetActAsMains(userID)
	slices.Sort(actAsAlts)
	slices.Sort(actAsMains)

	b.WriteString("\n### Act-As Links\n")
	if len(actAsAlts) == 0 && len(actAsMains) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, altID := range actAsAlts {
			fmt.Fprintf(&b, "- Can act as: %s\n", formatStatusAltDetail(altID))
		}
		for _, mainID := range actAsMains {
			fmt.Fprintf(&b, "- Controlled by: %s\n", formatStatusAltDetail(mainID))
		}
	}

	searchIDs := collectUniqueIDs(userID, controllerID, linkedAlts, actAsAlts, actAsMains)
	contracts := collectStatusContracts(searchIDs)

	b.WriteString("\n### Active Contracts\n")
	if len(contracts) == 0 {
		b.WriteString("- None\n")
	} else {
		for _, c := range contracts {
			label := c.ContractID
			if c.CoopID != "" {
				label = fmt.Sprintf("%s / %s", c.ContractID, c.CoopID)
			}
			stateLabel := statusContractStateLabel(c.State)
			if len(c.Links) > 0 {
				fmt.Fprintf(&b, "- %s (%s) — %s — Accounts: %s\n",
					strings.Join(c.Links, ", "), stateLabel, label, strings.Join(c.Accounts, ", "))
			} else {
				fmt.Fprintf(&b, "- %s (%s) — Accounts: %s\n",
					label, stateLabel, strings.Join(c.Accounts, ", "))
			}
		}
	}

	return b.String()
}

func isDiscordSnowflake(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func formatStatusAccountLabel(id string) string {
	if isDiscordSnowflake(id) {
		return fmt.Sprintf("<@%s>", id)
	}
	return fmt.Sprintf("`%s`", id)
}

func formatStatusAltDetail(id string) string {
	label := formatStatusAccountLabel(id)
	ign := strings.TrimSpace(farmerstate.GetMiscSettingString(id, "ei_ign"))
	var notes []string
	if ign != "" && !strings.EqualFold(ign, id) {
		notes = append(notes, fmt.Sprintf("IGN: %s", ign))
	}
	if farmerstate.GetMiscSettingString(id, "encrypted_ei_id") != "" {
		notes = append(notes, "EI ID: Registered")
	}
	if len(notes) == 0 {
		return label
	}
	return fmt.Sprintf("%s (%s)", label, strings.Join(notes, ", "))
}

func collectUniqueIDs(userID, controllerID string, groups ...[]string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	add(userID)
	add(controllerID)
	for _, g := range groups {
		for _, id := range g {
			add(id)
		}
	}
	return out
}

func collectStatusContracts(accountIDs []string) []statusContractEntry {
	idSet := make(map[string]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		idSet[id] = struct{}{}
	}

	ContractsMutex.RLock()
	defer ContractsMutex.RUnlock()

	var entries []statusContractEntry
	for _, c := range Contracts {
		if c == nil {
			continue
		}
		var matched []string
		matchedSet := make(map[string]struct{})

		for _, id := range accountIDs {
			if b, ok := c.Boosters[id]; ok && b != nil {
				matchedSet[id] = struct{}{}
				matched = append(matched, formatStatusAccountLabel(id))
			}
		}
		for _, b := range c.Boosters {
			if b == nil {
				continue
			}
			if _, already := matchedSet[b.UserID]; already {
				continue
			}
			if _, ok := idSet[b.AltController]; ok {
				matchedSet[b.UserID] = struct{}{}
				matched = append(matched, formatStatusAccountLabel(b.UserID))
			}
		}
		for _, id := range c.WaitlistBoosters {
			if _, ok := idSet[id]; ok {
				if _, already := matchedSet[id]; !already {
					matchedSet[id] = struct{}{}
					matched = append(matched, formatStatusAccountLabel(id)+" (waitlist)")
				}
			}
		}

		if len(matched) == 0 {
			continue
		}

		var links []string
		for _, loc := range c.Location {
			if loc == nil {
				continue
			}
			switch {
			case loc.GuildID != "" && loc.ChannelID != "":
				url := fmt.Sprintf("https://discord.com/channels/%s/%s", loc.GuildID, loc.ChannelID)
				if loc.ListMsgID != "" {
					url += "/" + loc.ListMsgID
				}
				linkText := loc.ChannelMention
				if linkText == "" {
					linkText = fmt.Sprintf("<#%s>", loc.ChannelID)
				}
				links = append(links, fmt.Sprintf("[%s](%s)", linkText, url))
			case loc.ChannelMention != "":
				links = append(links, loc.ChannelMention)
			case loc.ChannelID != "":
				links = append(links, fmt.Sprintf("<#%s>", loc.ChannelID))
			}
		}

		entries = append(entries, statusContractEntry{
			ContractID: c.ContractID,
			CoopID:     c.CoopID,
			State:      c.State,
			Links:      links,
			Accounts:   matched,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ContractID != entries[j].ContractID {
			return entries[i].ContractID < entries[j].ContractID
		}
		return entries[i].CoopID < entries[j].CoopID
	})

	return entries
}

func statusContractStateLabel(state int) string {
	switch state {
	case ContractStateSignup:
		return "Signup"
	case ContractStateFastrun:
		return "Fastrun"
	case ContractStateWaiting:
		return "Waiting"
	case ContractStateCompleted:
		return "Completed"
	case ContractStateArchive:
		return "Archive"
	case ContractStateBanker:
		return "Banker"
	default:
		return "Active"
	}
}
