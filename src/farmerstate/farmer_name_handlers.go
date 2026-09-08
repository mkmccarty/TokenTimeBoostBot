package farmerstate

import (
	"log"
	"regexp"
	"strings"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// HandleSetEggIncName handles the /seteggincname command
func HandleSetEggIncName(client dc.Client, e *dc.CommandEvent, isCoordinator func(dc.Client, string) bool) {
	// Protection against DM use
	if e.GuildID() == "" {
		_ = e.Respond(dc.Message{
			Content:   "This command can only be run in a server.",
			Ephemeral: true,
		})
		return
	}
	var eiName string
	var callerUserID = e.UserID()
	var userID = e.UserID()

	if farmer, ok := e.OptUser("discord-name"); ok {
		re := regexp.MustCompile(`[\\<>@#&!]`)
		userID = re.ReplaceAllString(farmer.Mention(), "")
	}

	var str = "Setting Egg, IGN for <@" + userID + "> to "

	if opt, ok := e.OptString("ei-ign"); ok {
		eiName = strings.TrimSpace(opt)
		str += eiName
	}

	// if eiName matches this regex ^EI[1-9]*$ then it an invalid name
	re := regexp.MustCompile(`^EI[1-9]*$`)
	if re.MatchString(eiName) {
		str = "Don't use your Egg, Inc. EI number."
	} else {
		// Is the user issuing the command a coordinator?
		if userID != callerUserID && !isCoordinator(client, callerUserID) {
			str = "This form of usage is restricted to contract coordinators and administrators."
		} else {
			SetEggIncName(userID, eiName)
		}
	}

	err := e.Respond(dc.Message{Content: str, Ephemeral: true})
	if err != nil {
		log.Println(err.Error())
	}
}
