package boost

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func getSignupContractSettings(channelID string, id string) (string, []discordgo.MessageComponent) {
	minValues := 1

	var builder strings.Builder
	fmt.Fprintf(&builder, "Contract created in <#%s>\n", channelID)
	builder.WriteString("Use the Contract button if you have to recycle it.\n")
	builder.WriteString("Make your selections here to set your contract style. These buttons will work until the \n")
	builder.WriteString("If this contract isn't an immediate start use `/change-planned-start` to add the time to the sign-up message.\n")
	builder.WriteString("React with 🌊 to automaticaly update the thread name.")

	return builder.String(), []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "cs_#style#" + id,
					Placeholder: "Select contract styles",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Boost List Style",
							Description: "Everyone sends tokens to the current booster",
							Value:       "boostlist",
							Default:     true,
							Emoji: &discordgo.ComponentEmoji{
								Name: "📜",
							},
						},
						{
							Label:       "Banker Style",
							Description: "Everyone sends tokens to a banker.",
							Value:       "banker",
							Default:     false,
							Emoji: &discordgo.ComponentEmoji{
								Name: "💰",
							},
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "cs_#crt#" + id,
					Placeholder: "Choose your options for CRT",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "No-CRT",
							Description: "Standard vanilla option for this contract",
							Value:       "no_crt",
							Default:     true,
							Emoji: &discordgo.ComponentEmoji{
								Name: "🍦",
							},
						},
						{
							Label:       "CRT",
							Description: "Chicken Run Tango",
							Value:       "crt",
							Default:     false,
							Emoji: &discordgo.ComponentEmoji{
								Name: "🔁",
							},
						},
						{
							Label:       "CRT+selfrun",
							Description: "Less Tango Legs ",
							Value:       "self_runs",
							Default:     false,
							Emoji: &discordgo.ComponentEmoji{
								Name: "🔂",
							},
						},
					},
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "cs_#order#" + id,
					Placeholder: "Select the boosting order for this contract",
					MinValues:   &minValues,
					MaxValues:   1,
					Options: []discordgo.SelectMenuOption{
						{
							Label:       "Sign-up Order",
							Description: "Boost list is in the order people sign up",
							Value:       "signup",
							Emoji: &discordgo.ComponentEmoji{
								Name: "😑",
							},
							Default: true,
						},
						{
							Label:       "Fair Order",
							Description: "Boost order is based order history in last 5 contracts",
							Value:       "fair",
							Emoji: &discordgo.ComponentEmoji{
								Name: "😇",
							},
							Default: false,
						},
						{
							Label:       "Random Order",
							Description: "Less Tango Legs ",
							Value:       "random",
							Emoji: &discordgo.ComponentEmoji{
								Name: "🤪",
							},

							Default: false,
						},
					},
				},
			},
		},
	}

}

// GetSignupComponents returns the signup components for a contract
func GetSignupComponents(disableStartContract bool, speedrun bool) (string, []discordgo.MessageComponent) {
	var str = "Join the contract and indicate the number boost tokens you'd like."
	startLabel := "Start Boosting"
	if speedrun {
		startLabel = "Start CRT"
	} else if disableStartContract {
		startLabel = "Started"
	}
	return str, []discordgo.MessageComponent{
		// add buttons to the action row
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🧑‍🌾",
					},
					Label:    "Join",
					Style:    discordgo.PrimaryButton,
					CustomID: "fd_signupFarmer",
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "🔔",
					},
					Label:    "Join w/Ping",
					Style:    discordgo.PrimaryButton,
					CustomID: "fd_signupBell",
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "❌",
					},
					Label:    "Leave",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_signupLeave",
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "⏱️",
					},
					Label:    startLabel,
					Style:    discordgo.SuccessButton,
					CustomID: "fd_signupStart",
					Disabled: disableStartContract,
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "♻️",
					},
					Label:    "Contract",
					Style:    discordgo.DangerButton,
					Disabled: false,
					CustomID: "fd_delete",
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "5️⃣",
					},
					Label:    " Tokens",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokens5",
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "6️⃣",
					},
					Label:    " Tokens",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokens6",
				},
				discordgo.Button{
					Emoji: &discordgo.ComponentEmoji{
						Name: "8️⃣",
					},
					Label:    " Tokens",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokens8",
				},
				discordgo.Button{
					Label:    "+ Token",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokens1",
				},
				discordgo.Button{
					Label:    "- Token",
					Style:    discordgo.SecondaryButton,
					CustomID: "fd_tokens_sub",
				},
			},
		},
	}
}
