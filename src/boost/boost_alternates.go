package boost

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
	"github.com/mkmccarty/TokenTimeBoostBot/src/farmerstate"
)

// GetSlashBoostOrderHelpersCommand returns the definition of the /boost-order-helpers command.
func GetSlashBoostOrderHelpersCommand(cmd string) *dc.Command {
	command := guildOnlyCommand(cmd, "Manage helper status for boost ordering")
	command.Options = []dc.Option{
		dc.SubCommand{
			Name:        "set",
			Description: "Designate one or more farmers as helpers in this contract",
			Options: []dc.Option{
				dc.StringOption{
					Name:        "farmers",
					Description: "List, mentions, or boost numbers of farmers to mark as helpers (leave blank for yourself)",
					Required:    false,
				},
				dc.BoolOption{
					Name:        "sticky",
					Description: "Make helper setting sticky for future contract joins",
					Required:    false,
				},
				dc.StringOption{
					Name:        "role",
					Description: "Role of farmer in boost order. Default is Helper.",
					Required:    false,
					Choices: []dc.Choice[string]{
						{Name: "Helper", Value: "Helper"},
						{Name: "Main", Value: "Main"},
					},
				},
			},
		},
		dc.SubCommand{
			Name:        "clear",
			Description: "Remove helper designation from one or more farmers, or 'all'",
			Options: []dc.Option{
				dc.StringOption{
					Name:        "farmers",
					Description: "Farmers to remove helper status from, 'all', or leave blank for yourself and alts",
					Required:    false,
				},
				dc.BoolOption{
					Name:        "sticky",
					Description: "Remove sticky helper setting for future contract joins",
					Required:    false,
				},
			},
		},
		dc.SubCommand{
			Name:        "list",
			Description: "List current helper designations in this contract",
		},
	}
	return &command
}

// canManageBooster returns true if callerID is authorized to manage targetID's helper status.
func canManageBooster(client dc.Client, contract *Contract, callerID string, targetID string) bool {
	if creatorOfContract(client, contract, callerID) {
		return true
	}
	if callerID == targetID {
		return true
	}
	if b := contract.Boosters[targetID]; b != nil && b.AltController == callerID {
		return true
	}
	if pb := contract.Boosters[callerID]; pb != nil && slices.Contains(pb.Alts, targetID) {
		return true
	}
	return false
}

// HandleBoostOrderHelpersCommand handles the /boost-order-helpers command.
func HandleBoostOrderHelpersCommand(client dc.Client, e *dc.CommandEvent) {
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
	isCoord := creatorOfContract(client, contract, userID)

	subcommand := ""
	if path := e.SubcommandPath(); len(path) > 0 {
		subcommand = path[0]
	}

	switch subcommand {
	case "set":
		farmersInput, _ := e.OptString("farmers")
		farmersInput = strings.TrimSpace(farmersInput)
		sticky, _ := e.OptBool("sticky")
		roleOpt, hasRole := e.OptString("role")
		isHelper := true
		if hasRole && strings.EqualFold(roleOpt, "Main") {
			isHelper = false
		}

		var matchedIDs []string
		var notFound []string

		if farmersInput == "" {
			if contract.Boosters[userID] != nil {
				matchedIDs = []string{userID}
			} else {
				if isCoord {
					_ = e.Respond(dc.Message{
						Content:   "Please specify one or more farmers to mark as helpers.",
						Ephemeral: true,
					})
				} else {
					_ = e.Respond(dc.Message{
						Content:   "You are not in this contract.",
						Ephemeral: true,
					})
				}
				return
			}
		} else {
			matchedIDs, notFound = parseContractFarmerList(contract, farmersInput)
		}

		if len(matchedIDs) == 0 && len(notFound) > 0 {
			_ = e.Respond(dc.Message{
				Content:   fmt.Sprintf("Could not find the following farmers in this contract: %s", strings.Join(notFound, ", ")),
				Ephemeral: true,
			})
			return
		}

		if len(matchedIDs) == 0 {
			_ = e.Respond(dc.Message{
				Content:   "No farmers specified to mark as helpers.",
				Ephemeral: true,
			})
			return
		}

		if !isCoord {
			for _, targetID := range matchedIDs {
				if !canManageBooster(client, contract, userID, targetID) {
					_ = e.Respond(dc.Message{
						Content:   "Only contract coordinators or channel admins can manage helper status for other farmers.",
						Ephemeral: true,
					})
					return
				}
			}
		}

		contract.mutex.Lock()
		var assignedNames []string
		for _, uID := range matchedIDs {
			if b := contract.Boosters[uID]; b != nil {
				b.IsHelper = isHelper
				b.IsAlt = isHelper
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

		reorderBoosters(contract)
		saveData(contract.ContractHash)
		refreshBoostListMessage(client, contract, false)

		if sticky {
			for _, uID := range matchedIDs {
				farmerstate.SetOrderHelper(uID, isHelper)
			}
		}

		var msg string
		if isHelper {
			if len(assignedNames) == 1 {
				msg = fmt.Sprintf("✅ Designated as helper for boost ordering: **%s**", assignedNames[0])
			} else {
				msg = fmt.Sprintf("✅ Designated as helpers for boost ordering: **%s**", strings.Join(assignedNames, ", "))
			}
			if sticky {
				msg += "\n-# Saved as sticky helper for future contracts."
			}
		} else {
			if len(assignedNames) == 1 {
				msg = fmt.Sprintf("✅ Designated as Main for boost ordering: **%s**", assignedNames[0])
			} else {
				msg = fmt.Sprintf("✅ Designated as Main for boost ordering: **%s**", strings.Join(assignedNames, ", "))
			}
			if sticky {
				msg += "\n-# Removed sticky helper setting for future contracts."
			}
		}
		if len(notFound) > 0 {
			msg += fmt.Sprintf("\n-# Not found in contract: %s", strings.Join(notFound, ", "))
		}
		_ = e.Respond(dc.Message{Content: msg, Ephemeral: true})

	case "clear":
		farmersInput, _ := e.OptString("farmers")
		sticky, _ := e.OptBool("sticky")
		trimmed := strings.TrimSpace(strings.ToLower(farmersInput))

		var matchedIDs []string
		var notFound []string
		isAll := false

		if trimmed == "" {
			if contract.Boosters[userID] != nil {
				matchedIDs = []string{userID}
				for altID, b := range contract.Boosters {
					if altID == userID {
						continue
					}
					if b.AltController == userID || slices.Contains(contract.Boosters[userID].Alts, altID) {
						if !slices.Contains(matchedIDs, altID) {
							matchedIDs = append(matchedIDs, altID)
						}
					}
				}
			} else {
				if isCoord {
					_ = e.Respond(dc.Message{
						Content:   "Please specify one or more farmers to remove helper status from, or 'all'.",
						Ephemeral: true,
					})
				} else {
					_ = e.Respond(dc.Message{
						Content:   "You are not in this contract.",
						Ephemeral: true,
					})
				}
				return
			}
		} else if trimmed == "all" {
			if !isCoord {
				_ = e.Respond(dc.Message{
					Content:   "Only contract coordinators or channel admins can clear all helpers.",
					Ephemeral: true,
				})
				return
			}
			isAll = true
		} else {
			matchedIDs, notFound = parseContractFarmerList(contract, farmersInput)
			if len(matchedIDs) == 0 && len(notFound) > 0 {
				_ = e.Respond(dc.Message{
					Content:   fmt.Sprintf("Could not find the following farmers in this contract: %s", strings.Join(notFound, ", ")),
					Ephemeral: true,
				})
				return
			}
			if len(matchedIDs) == 0 {
				_ = e.Respond(dc.Message{
					Content:   "No matching farmers found in this contract to clear.",
					Ephemeral: true,
				})
				return
			}
			if !isCoord {
				for _, targetID := range matchedIDs {
					if !canManageBooster(client, contract, userID, targetID) {
						_ = e.Respond(dc.Message{
							Content:   "Only contract coordinators or channel admins can manage helper status for other farmers.",
							Ephemeral: true,
						})
						return
					}
				}
			}
		}

		contract.mutex.Lock()
		var clearedNames []string
		var clearedIDs []string
		if isAll {
			for uID, b := range contract.Boosters {
				if b.IsHelper || b.IsAlt {
					b.IsHelper = false
					b.IsAlt = false
					name := b.Nick
					if name == "" {
						name = b.Name
					}
					if name == "" {
						name = b.UserID
					}
					clearedNames = append(clearedNames, name)
					clearedIDs = append(clearedIDs, uID)
				}
			}
		} else {
			for _, uID := range matchedIDs {
				if b := contract.Boosters[uID]; b != nil {
					if b.IsHelper || b.IsAlt {
						b.IsHelper = false
						b.IsAlt = false
						name := b.Nick
						if name == "" {
							name = b.Name
						}
						if name == "" {
							name = uID
						}
						clearedNames = append(clearedNames, name)
						clearedIDs = append(clearedIDs, uID)
					}
				}
			}
		}
		contract.mutex.Unlock()

		if sticky {
			for _, uID := range clearedIDs {
				farmerstate.SetOrderHelper(uID, false)
			}
		}

		if len(clearedNames) == 0 {
			if sticky && len(matchedIDs) > 0 {
				clearedSticky := false
				for _, uID := range matchedIDs {
					if farmerstate.GetOrderHelper(uID) {
						farmerstate.SetOrderHelper(uID, false)
						clearedSticky = true
					}
				}
				if clearedSticky {
					_ = e.Respond(dc.Message{
						Content:   "Removed sticky helper setting for future contracts.",
						Ephemeral: true,
					})
					return
				}
			}
			if trimmed == "" {
				if len(matchedIDs) > 1 {
					_ = e.Respond(dc.Message{
						Content:   "Neither you nor your linked alts are currently designated as helpers.",
						Ephemeral: true,
					})
				} else {
					_ = e.Respond(dc.Message{
						Content:   "You are not currently designated as a helper.",
						Ephemeral: true,
					})
				}
			} else {
				_ = e.Respond(dc.Message{
					Content:   "No matching helpers to clear.",
					Ephemeral: true,
				})
			}
			return
		}

		reorderBoosters(contract)
		saveData(contract.ContractHash)
		refreshBoostListMessage(client, contract, false)

		msg := fmt.Sprintf("Cleared helper designation for: **%s**", strings.Join(clearedNames, ", "))
		if sticky {
			msg += "\n-# Removed sticky helper setting for future contracts."
		}
		if len(notFound) > 0 {
			msg += fmt.Sprintf("\n-# Not found in contract: %s", strings.Join(notFound, ", "))
		}
		_ = e.Respond(dc.Message{
			Content:   msg,
			Ephemeral: true,
		})

	case "list":
		contract.mutex.Lock()
		var helpers []string
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
			if isBoosterHelper(b) {
				helpers = append(helpers, name)
			} else {
				mains = append(mains, name)
			}
		}
		contract.mutex.Unlock()

		var sb strings.Builder
		sb.WriteString("## 🧑‍🌾 Contract Helper Statuses\n")
		if len(helpers) > 0 {
			fmt.Fprintf(&sb, "**Helpers (%d):** %s\n", len(helpers), strings.Join(helpers, ", "))
		} else {
			sb.WriteString("**Helpers (0):** None designated\n")
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
