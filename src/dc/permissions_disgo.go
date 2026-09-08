package dc

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// Permission computation for channels the gateway cache does not hold.
//
// disgo answers Caches.MemberPermissionsInChannel from the cache alone, which
// covers the common case: the guild's channels arrive in GUILD_CREATE and stay
// current from channel events. It does not cover a thread that has archived
// since the bot connected, and this bot's contracts live in threads — so a
// coordinator check on an archived contract thread would otherwise fail closed.
//
// discordgo fell back to REST for exactly this, so the fallback below keeps the
// behaviour the bot had before the port: fetch what the cache is missing and
// apply Discord's documented permission algorithm.

// memberChannelPermissions applies Discord's permission rules to values fetched
// from anywhere, so it works on REST results as well as cached ones.
//
// https://discord.com/developers/docs/topics/permissions#permission-overwrites
func memberChannelPermissions(guildID, ownerID, userID snowflake.ID, roles []discord.Role, memberRoleIDs []snowflake.ID, overwrites discord.PermissionOverwrites) discord.Permissions {
	// The owner has every permission, whatever the overwrites say.
	if userID == ownerID {
		return discord.PermissionsAll
	}

	byID := make(map[snowflake.ID]discord.Role, len(roles))
	for _, role := range roles {
		byID[role.ID] = role
	}

	// The base is @everyone plus every role the member holds. The @everyone
	// role's ID is the guild's own ID.
	permissions := byID[guildID].Permissions
	for _, roleID := range memberRoleIDs {
		if role, ok := byID[roleID]; ok {
			permissions = permissions.Add(role.Permissions)
		}
	}

	// Administrator short-circuits the overwrites entirely.
	if permissions.Has(discord.PermissionAdministrator) {
		return discord.PermissionsAll
	}

	// Overwrites apply in a fixed order: @everyone, then the union of the
	// member's role overwrites, then the member's own.
	if overwrite, ok := overwrites.Role(guildID); ok {
		permissions &= ^overwrite.Deny
		permissions |= overwrite.Allow
	}

	var allow, deny discord.Permissions
	for _, roleID := range memberRoleIDs {
		if roleID == guildID {
			continue
		}
		if overwrite, ok := overwrites.Role(roleID); ok {
			allow |= overwrite.Allow
			deny |= overwrite.Deny
		}
	}
	permissions &= ^deny
	permissions |= allow

	if overwrite, ok := overwrites.Member(userID); ok {
		permissions &= ^overwrite.Deny
		permissions |= overwrite.Allow
	}

	return permissions
}

// permissionsFromREST computes a member's permissions in a channel the cache
// does not hold, fetching the channel, guild, roles and member.
func (c *disgoClient) permissionsFromREST(userID, channelID snowflake.ID) (Permissions, error) {
	channel, err := c.bot.Rest.GetChannel(channelID)
	if err != nil {
		return 0, wrapAPIError(err)
	}
	guildChannel, ok := channel.(discord.GuildChannel)
	if !ok {
		// A DM has no roles or overwrites to compute against. Discord grants
		// the participants everything a DM supports, and the bot only asks
		// this question about guild channels.
		return 0, &APIError{
			StatusCode: 400,
			Message:    "permissions are only defined for guild channels",
		}
	}

	guildID := guildChannel.GuildID()
	guild, err := c.bot.Rest.GetGuild(guildID, false)
	if err != nil {
		return 0, wrapAPIError(err)
	}
	roles, err := c.bot.Rest.GetRoles(guildID)
	if err != nil {
		return 0, wrapAPIError(err)
	}
	member, err := c.bot.Rest.GetMember(guildID, userID)
	if err != nil {
		return 0, wrapAPIError(err)
	}

	return Permissions(memberChannelPermissions(
		guildID, guild.OwnerID, userID, roles, member.RoleIDs, guildChannel.PermissionOverwrites(),
	)), nil
}
