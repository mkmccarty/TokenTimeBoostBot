package boost

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// GetSlashBoostOrderAltsCommand returns the definition of the /boost-order-alts command.
func GetSlashBoostOrderAltsCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Organizer command to designate alternate status for boost ordering")
	command.Options = []dc.Option{
		dc.SubCommand{
			Name:        "set",
			Description: "Designate one or more farmers as alternates in this contract",
			Options: []dc.Option{
				dc.StringOption{
					Name:        "farmers",
					Description: "List, mentions, or boost numbers of farmers to mark as alternates (e.g. 1 3 5, @Player, Guest1)",
					Required:    true,
				},
			},
		},
		dc.SubCommand{
			Name:        "clear",
			Description: "Remove alternate designation from one or more farmers, or 'all'",
			Options: []dc.Option{
				dc.StringOption{
					Name:        "farmers",
					Description: "List, mentions, or boost numbers of farmers to remove alternate status from, or 'all'",
					Required:    true,
				},
			},
		},
		dc.SubCommand{
			Name:        "list",
			Description: "List current alternate designations in this contract",
		},
	}
	return &command
}

// HandleBoostOrderAltsCommand handles the /boost-order-alts command.
func HandleBoostOrderAltsCommand(client dc.Client, e *dc.CommandEvent) {
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}

	contract := FindContract(e.ChannelID())
	if contract == nil {
		_ = e.Respond(dc.Message{
			Content:   "Contract not found in this channel.",
			Ephemeral: true,
		})
		return
	}

	userID := e.UserID()
	if !creatorOfContract(client, contract, userID) {
		_ = e.Respond(dc.Message{
			Content:   "Only contract coordinators or channel admins can manage contract alternates.",
			Ephemeral: true,
		})
		return
	}

	subcommand := ""
	if path := e.SubcommandPath(); len(path) > 0 {
		subcommand = path[0]
	}

	switch subcommand {
	case "set":
		farmersInput, _ := e.OptString("farmers")
		matchedIDs, notFound := parseContractFarmerList(contract, farmersInput)

		if len(matchedIDs) == 0 && len(notFound) > 0 {
			_ = e.Respond(dc.Message{
				Content:   fmt.Sprintf("Could not find the following farmers in this contract: %s", strings.Join(notFound, ", ")),
				Ephemeral: true,
			})
			return
		}

		contract.mutex.Lock()
		var assignedNames []string
		for _, uID := range matchedIDs {
			if b := contract.Boosters[uID]; b != nil {
				b.IsAlt = true
				name := b.Nick
				if name == "" {
					name = b.Name
				}
				if name == "" {
					name = uID
				}
				assignedNames = append(assignedNames, name)
			}
		}
		contract.mutex.Unlock()

		if contract.BoostOrder == ContractOrderESC || contract.BoostOrder == ContractOrderESCGG {
			reorderBoosters(contract)
		}
		saveData(contract.ContractHash)
		refreshBoostListMessage(client, contract, false)

		msg := fmt.Sprintf("✅ Designated as alternates for boost ordering: **%s**", strings.Join(assignedNames, ", "))
		if len(notFound) > 0 {
			msg += fmt.Sprintf("\n-# Not found in contract: %s", strings.Join(notFound, ", "))
		}
		_ = e.Respond(dc.Message{Content: msg, Ephemeral: true})

	case "clear":
		farmersInput, _ := e.OptString("farmers")
		trimmed := strings.TrimSpace(strings.ToLower(farmersInput))

		contract.mutex.Lock()
		var clearedNames []string
		if trimmed == "all" {
			for _, b := range contract.Boosters {
				if b.IsAlt {
					b.IsAlt = false
					name := b.Nick
					if name == "" {
						name = b.Name
					}
					if name == "" {
						name = b.UserID
					}
					clearedNames = append(clearedNames, name)
				}
			}
		} else {
			matchedIDs, _ := parseContractFarmerList(contract, farmersInput)
			for _, uID := range matchedIDs {
				if b := contract.Boosters[uID]; b != nil {
					b.IsAlt = false
					name := b.Nick
					if name == "" {
						name = b.Name
					}
					if name == "" {
						name = uID
					}
					clearedNames = append(clearedNames, name)
				}
			}
		}
		contract.mutex.Unlock()

		if contract.BoostOrder == ContractOrderESC || contract.BoostOrder == ContractOrderESCGG {
			reorderBoosters(contract)
		}
		saveData(contract.ContractHash)
		refreshBoostListMessage(client, contract, false)

		if len(clearedNames) == 0 {
			_ = e.Respond(dc.Message{
				Content:   "No matching alternates to clear.",
				Ephemeral: true,
			})
			return
		}

		_ = e.Respond(dc.Message{
			Content:   fmt.Sprintf("Cleared alternate designation for: **%s**", strings.Join(clearedNames, ", ")),
			Ephemeral: true,
		})

	case "list":
		contract.mutex.Lock()
		var alts []string
		var mains []string
		for _, uID := range contract.Order {
			b := contract.Boosters[uID]
			if b == nil {
				continue
			}
			name := b.Nick
			if name == "" {
				name = b.Name
			}
			if name == "" {
				name = uID
			}
			if b.IsAlt || b.AltController != "" {
				alts = append(alts, name)
			} else {
				mains = append(mains, name)
			}
		}
		contract.mutex.Unlock()

		var sb strings.Builder
		sb.WriteString("## 🧑‍🌾 Contract Alternate Statuses\n")
		if len(alts) > 0 {
			fmt.Fprintf(&sb, "**Alternates (%d):** %s\n", len(alts), strings.Join(alts, ", "))
		} else {
			sb.WriteString("**Alternates (0):** None designated\n")
		}
		if len(mains) > 0 {
			fmt.Fprintf(&sb, "**Standard Players (%d):** %s\n", len(mains), strings.Join(mains, ", "))
		}

		_ = e.Respond(dc.Message{
			Content:   sb.String(),
			Ephemeral: true,
		})
	default:
		_ = e.Respond(dc.Message{
			Content:   "Unknown subcommand. Use `set`, `clear`, or `list`.",
			Ephemeral: true,
		})
	}
}

// parseContractFarmerList extracts userIDs in the contract matching the input string.
func parseContractFarmerList(contract *Contract, input string) ([]string, []string) {
	if contract == nil {
		return nil, nil
	}

	// Split by comma, semicolon, space, newline
	tokens := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})

	var matchedIDs []string
	var notFound []string

	for _, token := range tokens {
		t := strings.TrimSpace(token)
		if t == "" {
			continue
		}

		foundID := ""

		// Check for mention
		if mentionID, isMention := parseMentionUserID(t); isMention {
			if contract.Boosters[mentionID] != nil {
				foundID = mentionID
			}
		} else if strings.HasPrefix(t, "<@") && strings.HasSuffix(t, ">") {
			rawID := strings.TrimPrefix(strings.TrimSuffix(t, ">"), "<@")
			rawID = strings.TrimPrefix(rawID, "!")
			if contract.Boosters[rawID] != nil {
				foundID = rawID
			}
		}

		cleanToken := strings.TrimPrefix(t, "@")

		// Check direct UserID
		if foundID == "" && contract.Boosters[cleanToken] != nil {
			foundID = cleanToken
		}

		// Search by Nick / Name / UserName / GlobalName (case-insensitive)
		if foundID == "" {
			tLower := strings.ToLower(cleanToken)
			for uID, b := range contract.Boosters {
				if strings.EqualFold(b.Nick, tLower) ||
					strings.EqualFold(b.Name, tLower) ||
					strings.EqualFold(b.UserName, tLower) ||
					strings.EqualFold(b.GlobalName, tLower) {
					foundID = uID
					break
				}
			}
		}

		// If still not found and token is a number, check if it matches a boost list position (1-100)
		if foundID == "" {
			if num, err := strconv.Atoi(cleanToken); err == nil {
				if num >= 1 && num <= len(contract.Order) && num <= 100 {
					posID := contract.Order[num-1]
					if contract.Boosters[posID] != nil {
						foundID = posID
					}
				}
			}
		}

		if foundID != "" {
			if !slices.Contains(matchedIDs, foundID) {
				matchedIDs = append(matchedIDs, foundID)
			}
		} else {
			notFound = append(notFound, t)
		}
	}

	return matchedIDs, notFound
}
