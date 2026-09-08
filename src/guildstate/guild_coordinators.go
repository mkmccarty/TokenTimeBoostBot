package guildstate

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// IsGuildCoordinator returns true if the user is a registered coordinator for the guild
// or the global bot admin.
func IsGuildCoordinator(guildID, userID string) bool {
	if userID == config.AdminUserID {
		return true
	}
	if slices.Contains(config.AdminUsers, userID) {
		return true
	}
	_, err := queries.GetGuildCoordinator(ctx, GetGuildCoordinatorParams{GuildID: guildID, UserID: userID})
	return err == nil
}

// AddGuildCoordinator registers a user as a coordinator for the guild.
func AddGuildCoordinator(guildID, userID, addedBy string) error {
	return queries.InsertGuildCoordinator(ctx, InsertGuildCoordinatorParams{
		GuildID: guildID,
		UserID:  userID,
		AddedBy: addedBy,
		AddedAt: time.Now().Unix(),
	})
}

// RemoveGuildCoordinator removes a user from the guild's coordinator list.
func RemoveGuildCoordinator(guildID, userID string) error {
	return queries.DeleteGuildCoordinator(ctx, DeleteGuildCoordinatorParams{GuildID: guildID, UserID: userID})
}

// GetCoordinatorList returns all coordinators for a guild ordered by when they were added.
func GetCoordinatorList(guildID string) ([]GuildCoordinator, error) {
	return queries.GetGuildCoordinators(ctx, guildID)
}

// SlashCoordinatorsCommand builds the /admin-coordinators slash command definition.
// DefaultMemberPermissions is set to 0 so only Discord server admins can use it.
func SlashCoordinatorsCommand(cmd string) *dc.Command {
	var adminPermission = int64(0)
	command := dc.Command{
		Name:                     cmd,
		Description:              "Manage bot coordinators for this server",
		DefaultMemberPermissions: &adminPermission,
		Contexts:                 []dc.InteractionContext{dc.ContextGuild},
		IntegrationTypes:         []dc.IntegrationType{dc.IntegrationGuildInstall},
		Options: []dc.Option{
			dc.SubCommand{
				Name:        "add",
				Description: "Add a coordinator",
				Options: []dc.Option{
					dc.UserOption{
						Name:        "user",
						Description: "User to grant coordinator access",
						Required:    true,
					},
				},
			},
			dc.SubCommand{
				Name:        "remove",
				Description: "Remove a coordinator",
				Options: []dc.Option{
					dc.UserOption{
						Name:        "user",
						Description: "User to revoke coordinator access from",
						Required:    true,
					},
				},
			},
			dc.SubCommand{
				Name:        "list",
				Description: "List all coordinators",
			},
		},
	}
	return &command
}

func HandleCoordinators(client dc.Client, e *dc.CommandEvent) {
	// Ack first to buy time for DB operations and avoid hitting the 3s limit for responding to interactions.
	if !respondDeferredEphemeral(e) {
		return
	}

	sub, ok := e.Subcommand()
	if !ok {
		followupEphemeral(e, "Please specify a subcommand.")
		return
	}
	switch sub {
	case "add":
		handleCoordinatorAdd(client, e)
	case "remove":
		handleCoordinatorRemove(client, e)
	case "list":
		handleCoordinatorList(client, e)
	default:
		followupEphemeral(e, "Unknown subcommand.")
	}
}

func handleCoordinatorAdd(client dc.Client, e *dc.CommandEvent) {
	if !isAdminCaller(client, e) {
		followupEphemeral(e, "You are not authorized to add coordinators.")
		return
	}

	user, ok := e.OptUser("add-user")
	if !ok {
		followupEphemeral(e, "No user was supplied.")
		return
	}

	if IsGuildCoordinator(e.GuildID(), user.ID) {
		followupEphemeral(e, fmt.Sprintf("<@%s> is already a coordinator.", user.ID))
		return
	}

	if err := AddGuildCoordinator(e.GuildID(), user.ID, e.UserID()); err != nil {
		log.Println("AddGuildCoordinator:", err)
		followupEphemeral(e, "Failed to add coordinator.")
		return
	}
	followupEphemeral(e, fmt.Sprintf("Added <@%s> as a coordinator.", user.ID))
}

func handleCoordinatorRemove(client dc.Client, e *dc.CommandEvent) {
	if !isAdminCaller(client, e) {
		followupEphemeral(e, "You are not authorized to remove coordinators.")
		return
	}

	user, ok := e.OptUser("remove-user")
	if !ok {
		followupEphemeral(e, "No user was supplied.")
		return
	}

	if !IsGuildCoordinator(e.GuildID(), user.ID) {
		followupEphemeral(e, fmt.Sprintf("<@%s> is not a coordinator.", user.ID))
		return
	}

	if err := RemoveGuildCoordinator(e.GuildID(), user.ID); err != nil {
		log.Println("RemoveGuildCoordinator:", err)
		followupEphemeral(e, "Failed to remove coordinator.")
		return
	}
	followupEphemeral(e, fmt.Sprintf("Removed <@%s> from coordinators.", user.ID))
}

func handleCoordinatorList(client dc.Client, e *dc.CommandEvent) {
	if !isAdminCaller(client, e) {
		followupEphemeral(e, "You are not authorized to view coordinators.")
		return
	}

	coords, err := GetCoordinatorList(e.GuildID())
	if err != nil {
		log.Println("GetCoordinatorList:", err)
		followupEphemeral(e, "Failed to retrieve coordinators.")
		return
	}

	if len(coords) == 0 {
		followupEphemeral(e, "No coordinators configured for this server.")
		return
	}

	var sb strings.Builder
	sb.WriteString("**Coordinators**:\n")
	for idx, c := range coords {
		fmt.Fprintf(&sb, "%d. <@%s> — added by <@%s> on %s\n",
			idx+1, c.UserID, c.AddedBy, bottools.WrapTimestamp(c.AddedAt, bottools.TimestampLongDateTime))
	}
	followupEphemeral(e, sb.String())
}
