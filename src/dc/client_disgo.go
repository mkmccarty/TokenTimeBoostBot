package dc

import (
	"context"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/snowflake/v2"
)

// disgoClient implements Client over a disgo bot client. Every method takes
// the facade's string IDs and parses them to snowflakes here, at the one
// boundary where the conversion belongs.
//
// A malformed ID returns an error rather than panicking: the IDs come from
// Discord and from saved contract data, so a bad one is data to report, not a
// programming mistake to crash on.
type disgoClient struct {
	bot *bot.Client
}

// newDisgoClient wraps a live disgo client as a Client.
func newDisgoClient(b *bot.Client) *disgoClient {
	return &disgoClient{bot: b}
}

// IsSnowflake reports whether s is a valid Discord snowflake ID.
func IsSnowflake(s string) bool {
	id, err := snowflake.Parse(s)
	return err == nil && id != 0
}

// parseIDs parses a list of facade IDs, returning the first failure.
func parseIDs(ids ...string) ([]snowflake.ID, error) {
	out := make([]snowflake.ID, 0, len(ids))
	for _, id := range ids {
		parsed, err := snowflake.Parse(id)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

// SendMessage posts a new message to a channel.
func (c *disgoClient) SendMessage(channelID string, m Message) (*MessageRef, error) {
	ids, err := parseIDs(channelID)
	if err != nil {
		return nil, err
	}
	msg, err := c.bot.Rest.CreateMessage(ids[0], m.toMessageCreate())
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return messageRefFrom(msg), nil
}

// EditMessage replaces the content of an existing message.
func (c *disgoClient) EditMessage(channelID, messageID string, m Message) (*MessageRef, error) {
	ids, err := parseIDs(channelID, messageID)
	if err != nil {
		return nil, err
	}
	msg, err := c.bot.Rest.UpdateMessage(ids[0], ids[1], m.toMessageUpdate())
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return messageRefFrom(msg), nil
}

// DeleteMessage removes a message.
func (c *disgoClient) DeleteMessage(channelID, messageID string) error {
	ids, err := parseIDs(channelID, messageID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.DeleteMessage(ids[0], ids[1]))
}

// GetMessage fetches a single message.
func (c *disgoClient) GetMessage(channelID, messageID string) (*MessageRef, error) {
	ids, err := parseIDs(channelID, messageID)
	if err != nil {
		return nil, err
	}
	msg, err := c.bot.Rest.GetMessage(ids[0], ids[1])
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return messageRefFrom(msg), nil
}

// PinMessage pins a message to its channel.
func (c *disgoClient) PinMessage(channelID, messageID string) error {
	ids, err := parseIDs(channelID, messageID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.PinMessage(ids[0], ids[1]))
}

// UnpinMessage unpins a message from its channel.
func (c *disgoClient) UnpinMessage(channelID, messageID string) error {
	ids, err := parseIDs(channelID, messageID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.UnpinMessage(ids[0], ids[1]))
}

// Typing sends a typing indicator to a channel.
func (c *disgoClient) Typing(channelID string) error {
	ids, err := parseIDs(channelID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.SendTyping(ids[0]))
}

// Channel fetches a channel or thread by ID. The gateway cache already holds
// every channel the bot can see and is kept current by channel events, so it
// answers without a REST call when it can.
func (c *disgoClient) Channel(channelID string) (*Channel, error) {
	ids, err := parseIDs(channelID)
	if err != nil {
		return nil, err
	}
	if cached, ok := c.bot.Caches.Channel(ids[0]); ok {
		return channelFrom(cached), nil
	}
	ch, err := c.bot.Rest.GetChannel(ids[0])
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return channelFrom(ch), nil
}

// EditChannel renames a channel or thread.
func (c *disgoClient) EditChannel(channelID, name string) (*Channel, error) {
	ids, err := parseIDs(channelID)
	if err != nil {
		return nil, err
	}
	ch, err := c.bot.Rest.UpdateChannel(ids[0], discord.GuildTextChannelUpdate{Name: &name})
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return channelFrom(ch), nil
}

// GuildChannels lists every channel in a guild.
func (c *disgoClient) GuildChannels(guildID string) ([]Channel, error) {
	ids, err := parseIDs(guildID)
	if err != nil {
		return nil, err
	}
	found, err := c.bot.Rest.GetGuildChannels(ids[0])
	if err != nil {
		return nil, wrapAPIError(err)
	}
	channels := make([]Channel, 0, len(found))
	for _, ch := range found {
		if converted := channelFrom(ch); converted != nil {
			channels = append(channels, *converted)
		}
	}
	return channels, nil
}

// Guild fetches a guild by ID.
func (c *disgoClient) Guild(guildID string) (*Guild, error) {
	ids, err := parseIDs(guildID)
	if err != nil {
		return nil, err
	}
	g, err := c.bot.Rest.GetGuild(ids[0], false)
	if err != nil {
		return nil, wrapAPIError(err)
	}
	if g == nil {
		return nil, nil
	}
	return &Guild{ID: idString(g.ID), Name: g.Name, OwnerID: idString(g.OwnerID)}, nil
}

// GuildMember fetches a user's guild-scoped identity.
func (c *disgoClient) GuildMember(guildID, userID string) (*Member, error) {
	ids, err := parseIDs(guildID, userID)
	if err != nil {
		return nil, err
	}
	m, err := c.bot.Rest.GetMember(ids[0], ids[1])
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return memberFrom(m), nil
}

// GuildMemberWithColor fetches a member along with the color their highest
// colored role gives them. The color comes from the gateway's role cache, and
// is 0 when the cache cannot supply one.
//
// The channel argument is accepted for the facade's shape but no longer used:
// a member's name color comes from their roles, which are guild-wide, and
// disgo computes it without a channel.
func (c *disgoClient) GuildMemberWithColor(guildID, userID, _ string) (*Member, int, error) {
	ids, err := parseIDs(guildID, userID)
	if err != nil {
		return nil, 0, err
	}
	m, err := c.bot.Rest.GetMember(ids[0], ids[1])
	if err != nil {
		return nil, 0, wrapAPIError(err)
	}
	color := 0
	if m != nil {
		color = highestColoredRoleColor(c.bot.Caches.MemberRoles(*m))
	}
	return memberFrom(m), color, nil
}

// GuildRoles lists every role in a guild.
func (c *disgoClient) GuildRoles(guildID string) ([]Role, error) {
	ids, err := parseIDs(guildID)
	if err != nil {
		return nil, err
	}
	found, err := c.bot.Rest.GetRoles(ids[0])
	if err != nil {
		return nil, wrapAPIError(err)
	}
	roles := make([]Role, 0, len(found))
	for i := range found {
		roles = append(roles, *roleFrom(&found[i]))
	}
	return roles, nil
}

// CreateGuildRole creates a new guild role.
func (c *disgoClient) CreateGuildRole(guildID string, params RoleParams) (*Role, error) {
	ids, err := parseIDs(guildID)
	if err != nil {
		return nil, err
	}
	role, err := c.bot.Rest.CreateRole(ids[0], discord.RoleCreate{
		Name:        params.Name,
		Mentionable: params.Mentionable != nil && *params.Mentionable,
	})
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return roleFrom(role), nil
}

// EditGuildRole updates an existing guild role.
func (c *disgoClient) EditGuildRole(guildID, roleID string, params RoleParams) (*Role, error) {
	ids, err := parseIDs(guildID, roleID)
	if err != nil {
		return nil, err
	}
	name := params.Name
	role, err := c.bot.Rest.UpdateRole(ids[0], ids[1], discord.RoleUpdate{
		Name:        &name,
		Mentionable: params.Mentionable,
	})
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return roleFrom(role), nil
}

// DeleteGuildRole deletes a guild role.
func (c *disgoClient) DeleteGuildRole(guildID, roleID string) error {
	ids, err := parseIDs(guildID, roleID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.DeleteRole(ids[0], ids[1]))
}

// AddGuildMemberRole grants a role to a guild member.
func (c *disgoClient) AddGuildMemberRole(guildID, userID, roleID string) error {
	ids, err := parseIDs(guildID, userID, roleID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.AddMemberRole(ids[0], ids[1], ids[2]))
}

// RemoveGuildMemberRole revokes a role from a guild member.
func (c *disgoClient) RemoveGuildMemberRole(guildID, userID, roleID string) error {
	ids, err := parseIDs(guildID, userID, roleID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.RemoveMemberRole(ids[0], ids[1], ids[2]))
}

// selfUser is the bot's own account, or false before the gateway reports it.
func (c *disgoClient) selfUser() (discord.OAuth2User, bool) {
	return c.bot.Caches.SelfUser()
}

// BotUsername is the display name of the account the bot is signed in as.
func (c *disgoClient) BotUsername() string {
	if user, ok := c.selfUser(); ok {
		return user.Username
	}
	return ""
}

// BotAvatarURL is the bot's own avatar at the requested pixel size.
func (c *disgoClient) BotAvatarURL(size string) string {
	user, ok := c.selfUser()
	if !ok {
		return ""
	}
	return avatarURL(user.User, size)
}

// User fetches an account by ID.
func (c *disgoClient) User(userID string) (*User, error) {
	ids, err := parseIDs(userID)
	if err != nil {
		return nil, err
	}
	u, err := c.bot.Rest.GetUser(ids[0])
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return userFrom(u), nil
}

// CreateUserChannel opens (or fetches the existing) DM channel with a user.
func (c *disgoClient) CreateUserChannel(userID string) (*Channel, error) {
	ids, err := parseIDs(userID)
	if err != nil {
		return nil, err
	}
	ch, err := c.bot.Rest.CreateDMChannel(ids[0])
	if err != nil {
		return nil, wrapAPIError(err)
	}
	if ch == nil {
		return nil, nil
	}
	return &Channel{ID: idString(ch.ID()), Name: ch.Name()}, nil
}

// UserChannelPermissions computes a user's effective permissions in a channel.
//
// disgo answers this from the gateway cache. When the channel is not cached the
// computation falls back to REST, which is what discordgo did before the port —
// see permissions_disgo.go.
func (c *disgoClient) UserChannelPermissions(userID, channelID string) (Permissions, error) {
	ids, err := parseIDs(userID, channelID)
	if err != nil {
		return 0, err
	}
	channel, ok := c.bot.Caches.Channel(ids[1])
	if !ok {
		// An archived thread drops out of the cache, and this bot's contracts
		// live in threads, so the fallback is a normal path rather than a rare
		// one.
		return c.permissionsFromREST(ids[0], ids[1])
	}
	member, ok := c.bot.Caches.Member(channel.GuildID(), ids[0])
	if !ok {
		fetched, err := c.bot.Rest.GetMember(channel.GuildID(), ids[0])
		if err != nil {
			return 0, wrapAPIError(err)
		}
		member = *fetched
	}
	return Permissions(c.bot.Caches.MemberPermissionsInChannel(channel, member)), nil
}

// RemoveMessageReaction removes one user's reaction from a message.
func (c *disgoClient) RemoveMessageReaction(channelID, messageID, emojiID, userID string) error {
	ids, err := parseIDs(channelID, messageID, userID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.RemoveUserReaction(ids[0], ids[1], emojiID, ids[2]))
}

// StartThread creates a public thread under a channel.
func (c *disgoClient) StartThread(channelID, name string, archiveDurationMinutes int) (*Channel, error) {
	ids, err := parseIDs(channelID)
	if err != nil {
		return nil, err
	}
	thread, err := c.bot.Rest.CreateThread(ids[0], discord.GuildPublicThreadCreate{
		Name:                name,
		AutoArchiveDuration: autoArchiveDuration(archiveDurationMinutes),
	})
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return channelFrom(thread), nil
}

// JoinThread adds the bot to a thread.
func (c *disgoClient) JoinThread(channelID string) error {
	ids, err := parseIDs(channelID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.JoinThread(ids[0]))
}

// AddThreadMember adds a member to a thread.
func (c *disgoClient) AddThreadMember(threadID, userID string) error {
	ids, err := parseIDs(threadID, userID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.AddThreadMember(ids[0], ids[1]))
}

// ActiveThreads lists the active threads in a guild. The argument is a guild
// ID: Discord's active-threads endpoint is guild-scoped.
func (c *disgoClient) ActiveThreads(guildID string) ([]Channel, error) {
	ids, err := parseIDs(guildID)
	if err != nil {
		return nil, err
	}
	active, err := c.bot.Rest.GetActiveGuildThreads(ids[0])
	if err != nil {
		return nil, wrapAPIError(err)
	}
	if active == nil {
		return nil, nil
	}
	channels := make([]Channel, 0, len(active.Threads))
	for i := range active.Threads {
		if converted := channelFrom(active.Threads[i]); converted != nil {
			channels = append(channels, *converted)
		}
	}
	return channels, nil
}

// SetStatus sets the bot's presence to "Playing <activityName>".
func (c *disgoClient) SetStatus(activityName string) error {
	return c.bot.SetPresence(context.Background(), gateway.WithPlayingActivity(activityName))
}

// ApplicationCommands lists the commands currently registered for an app,
// scoped to a guild (empty guildID means the global command set).
func (c *disgoClient) ApplicationCommands(appID, guildID string) ([]CommandRef, error) {
	app, err := parseIDs(appID)
	if err != nil {
		return nil, err
	}
	var found []discord.ApplicationCommand
	if guildID == "" {
		found, err = c.bot.Rest.GetGlobalCommands(app[0], false)
	} else {
		guild, guildErr := parseIDs(guildID)
		if guildErr != nil {
			return nil, guildErr
		}
		found, err = c.bot.Rest.GetGuildCommands(app[0], guild[0], false)
	}
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return commandRefsFrom(found), nil
}

// DeleteApplicationCommand removes one registered command.
func (c *disgoClient) DeleteApplicationCommand(appID, guildID, commandID string) error {
	ids, err := parseIDs(appID, commandID)
	if err != nil {
		return err
	}
	if guildID == "" {
		return wrapAPIError(c.bot.Rest.DeleteGlobalCommand(ids[0], ids[1]))
	}
	guild, err := parseIDs(guildID)
	if err != nil {
		return err
	}
	return wrapAPIError(c.bot.Rest.DeleteGuildCommand(ids[0], guild[0], ids[1]))
}

// BulkOverwriteApplicationCommands replaces an app's entire command set in
// one call.
func (c *disgoClient) BulkOverwriteApplicationCommands(appID, guildID string, commands []Command) ([]CommandRef, error) {
	app, err := parseIDs(appID)
	if err != nil {
		return nil, err
	}
	desired := make([]discord.ApplicationCommandCreate, 0, len(commands))
	for _, cmd := range commands {
		desired = append(desired, cmd.ToDisgo())
	}

	var created []discord.ApplicationCommand
	if guildID == "" {
		created, err = c.bot.Rest.SetGlobalCommands(app[0], desired)
	} else {
		guild, guildErr := parseIDs(guildID)
		if guildErr != nil {
			return nil, guildErr
		}
		created, err = c.bot.Rest.SetGuildCommands(app[0], guild[0], desired)
	}
	if err != nil {
		return nil, wrapAPIError(err)
	}
	return commandRefsFrom(created), nil
}

// ApplicationEmojis lists the emoji uploaded to an application.
func (c *disgoClient) ApplicationEmojis(appID string) ([]Emoji, error) {
	ids, err := parseIDs(appID)
	if err != nil {
		return nil, err
	}
	found, err := c.bot.Rest.GetApplicationEmojis(ids[0])
	if err != nil {
		return nil, wrapAPIError(err)
	}
	emojis := make([]Emoji, 0, len(found))
	for _, emoji := range found {
		emojis = append(emojis, Emoji{Name: emoji.Name, ID: idString(emoji.ID), Animated: emoji.Animated})
	}
	return emojis, nil
}

// ApplicationEmojiCreate uploads one emoji to an application.
func (c *disgoClient) ApplicationEmojiCreate(appID string, params EmojiParams) (*Emoji, error) {
	ids, err := parseIDs(appID)
	if err != nil {
		return nil, err
	}
	created, err := c.bot.Rest.CreateApplicationEmoji(ids[0], discord.EmojiCreate{
		Name:  params.Name,
		Image: discord.Icon{},
	})
	if err != nil {
		return nil, wrapAPIError(err)
	}
	if created == nil {
		return nil, nil
	}
	return &Emoji{Name: created.Name, ID: idString(created.ID), Animated: created.Animated}, nil
}

// compile-time assertion that disgoClient satisfies Client.
var _ Client = (*disgoClient)(nil)
