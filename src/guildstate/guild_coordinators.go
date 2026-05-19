package guildstate

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/mkmccarty/TokenTimeBoostBot/src/bottools"
	"github.com/mkmccarty/TokenTimeBoostBot/src/config"
)

// IsGuildCoordinator returns true if the user is a registered coordinator for the guild
// or the global bot admin.
func IsGuildCoordinator(guildID, userID string) bool {
	if userID == config.AdminUserID {
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
func SlashCoordinatorsCommand(cmd string) *discordgo.ApplicationCommand {
	var adminPermission = int64(0)
	return &discordgo.ApplicationCommand{
		Name:                     cmd,
		Description:              "Manage bot coordinators for this server",
		DefaultMemberPermissions: &adminPermission,
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationGuildInstall,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "add",
				Description: "Add a coordinator",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "user",
						Description: "User to grant coordinator access",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "remove",
				Description: "Remove a coordinator",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "user",
						Description: "User to revoke coordinator access from",
						Required:    true,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "List all coordinators",
			},
		},
	}
}

// HandleCoordinators dispatches the /admin-coordinators subcommands.
func HandleCoordinators(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Ack first to buy time for DB operations and avoid hitting the 3s limit for responding to interactions.
	if !respondDeferredEphemeral(s, i) {
		return
	}

	// Options
	data := i.ApplicationCommandData()
	if len(data.Options) == 0 {
		followupEphemeral(s, i, "Please specify a subcommand.")
		return
	}
	switch data.Options[0].Name {
	case "add":
		handleCoordinatorAdd(s, i, data.Options[0])
	case "remove":
		handleCoordinatorRemove(s, i, data.Options[0])
	case "list":
		handleCoordinatorList(s, i)
	default:
		followupEphemeral(s, i, "Unknown subcommand.")
	}
}

func handleCoordinatorAdd(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	if !isAdminCaller(s, i) {
		followupEphemeral(s, i, "You are not authorized to add coordinators.")
		return
	}

	user := sub.Options[0].UserValue(s)

	if IsGuildCoordinator(i.GuildID, user.ID) {
		followupEphemeral(s, i, fmt.Sprintf("<@%s> is already a coordinator.", user.ID))
		return
	}

	callerID := getInteractionUserID(i)
	if err := AddGuildCoordinator(i.GuildID, user.ID, callerID); err != nil {
		log.Println("AddGuildCoordinator:", err)
		followupEphemeral(s, i, "Failed to add coordinator.")
		return
	}
	followupEphemeral(s, i, fmt.Sprintf("Added <@%s> as a coordinator.", user.ID))
}

func handleCoordinatorRemove(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	if !isAdminCaller(s, i) {
		followupEphemeral(s, i, "You are not authorized to remove coordinators.")
		return
	}

	user := sub.Options[0].UserValue(s)

	if !IsGuildCoordinator(i.GuildID, user.ID) {
		followupEphemeral(s, i, fmt.Sprintf("<@%s> is not a coordinator.", user.ID))
		return
	}

	if err := RemoveGuildCoordinator(i.GuildID, user.ID); err != nil {
		log.Println("RemoveGuildCoordinator:", err)
		followupEphemeral(s, i, "Failed to remove coordinator.")
		return
	}
	followupEphemeral(s, i, fmt.Sprintf("Removed <@%s> from coordinators.", user.ID))
}

func handleCoordinatorList(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !isAdminCaller(s, i) {
		followupEphemeral(s, i, "You are not authorized to view coordinators.")
		return
	}

	coords, err := GetCoordinatorList(i.GuildID)
	if err != nil {
		log.Println("GetCoordinatorList:", err)
		followupEphemeral(s, i, "Failed to retrieve coordinators.")
		return
	}

	if len(coords) == 0 {
		followupEphemeral(s, i, "No coordinators configured for this server.")
		return
	}

	var sb strings.Builder
	sb.WriteString("**Coordinators**:\n")
	for idx, c := range coords {
		fmt.Fprintf(&sb, "%d. <@%s> — added by <@%s> on %s\n",
			idx+1, c.UserID, c.AddedBy, bottools.WrapTimestamp(c.AddedAt, bottools.TimestampLongDateTime))
	}
	followupEphemeral(s, i, sb.String())
}
