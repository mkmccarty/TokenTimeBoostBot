package boost

import "github.com/bwmarrin/discordgo"

// HandleHelpCommand will handle the help command
func HandleHelpCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	str := "Context sensitive help"

	userID := ""
	if i.GuildID == "" {
		userID = i.User.ID
	} else {
		userID = i.Member.User.ID
	}

	str = GetHelp(s, i.GuildID, i.ChannelID, userID)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    str,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: []discordgo.MessageComponent{}},
	})
}

// GetHelp will return the help string for the contract
func GetHelp(s *discordgo.Session, guildID string, channelID string, userID string) string {
	str := "# Boost Bot Help"
	var contract = FindContract(channelID)
	if contract == nil {
		// No contract, show help for creating a contract
		// Anyone can do this so just give the basic instructions
		str += `
		## Create a contract

		> **/contract**
		> * *contract-id* : Contract name
		> * *coop-id* : Coop name
		> * *coop-size* : Number of farmers for the coop. (optional)
		> * *boost-order* : (opt)
		>  * *Sign-up Order* : Default. Boosters are ordered in the order they join.
		>  * *Random Order* : Randomized when the contract starts. After 20 minutes the order changes to Sign-up.
		>  * *Fair Order* : Fair based on position percentile of each farmers last 5 contracts. Those with no history use 50th percentile. After 20 minutes the order changes to Sign-up.
		> *ping-role* : (opt) Default is @here. Role to ping when a new booster is up.
		`
		return str
	}

	contractCreator := creatorOfContract(contract, userID)

	if contractCreator {

		if contract.State == ContractStateSignup {
			str += `
			## Start the contract
			
			Press the Green Button to move from the Sign-up phase to the Boost phase.

			`
		}

		// Important commands for contract creators
		str += `## Coodinator Commands

		> **/join** : Add a farmer to the contract.
		> * *farmer* : Mention or guest farmer name.
		> * *token-count* : (opt) Tokens this farmer wants to boost.
		> * *boost-order* : (opt) position to add this farmer to the contract.
		> **/prune** : Remove a booster from the contract.
		> * *farmer* : Mention or guest farmer name.
		> **/change** : Alter aspects of a running contract
		> * *contract-id* : Change the contract-id.
		> * *coop-id* : Change the coop-id.
		> * *ping-role* : Change the ping role to something else. The @here role cannot be selected.
		> * *current-booster* : Change the current booster to a different farmer.
		>  * User mention or guest farmer name.
		> * *one-boost-position* : Move a booster to a different position in the boost order.
		>  * Provide a mention or guest name and a position number. i.e "@farmer 3" or "guestfarmer 5"
		> * *boost-order* : Change the entire boost order list.
		>  * Provide a new boost order. Every booster must be included in the new order.
		>  * Comma separated list of digits and/or ranges. i.e. "1,3,5,2,4" or "1,2,5-3" or "4,5,1-3".
		>  * Range can be specified with a hypthen: 1-5
		>  * Reverse range can be specified with range: 5-1
		> **/bump** : Redraw the Boost List message.
		> **/seteggincname** : Set users Egg, Inc game name.
		>  * *ei-name* : Include @mention in field to target a different user "@mention ei-name"
		
		`
	}

	if !userInContract(contract, userID) {
		str += `## Join the contract

		See the pinned message for buttons to *Join*, *Join w/Ping* or *Leave* the contract.
		You can set your boost tokens wanted by selecting :five: :six: or :eight: and adjusting it with the +Token and -Token buttons.
		
		`
		// No point in showing the rest of the help
		return str
	}

	// Basics for those Boosting
	boosterStr := `## Booster Commands

	> **/boost** : Out of order boosting, mark yourself as boosted.
	> **/unboost** : Mark a booster as unboosted.
	> * *farmer* : Mention or guest farmer name.
	> **/coopeta** : Display a discord message with a discord timestamp of the contract completion time.
	> * *rate* : Hourly production rate of the contract (i.e. 15.7q)
	> * *timespan* : Time remaining on the contract (i.e. 2d3h15m)
	> **/seteggincname** : Use to set your Egg, Inc game name.
	> * *ei-name* : (opt) Your game name.
`
	if (len(str) + len(boosterStr)) < 2000 {
		str += boosterStr
	}

	return str
}
