package dc

// Client is the bot's REST surface onto Discord: everything outside an
// interaction (see event.go for interaction-scoped calls such as responding
// to a command).
type Client interface {
	// SendMessage posts a new message to a channel.
	SendMessage(channelID string, m Message) (*MessageRef, error)
	// EditMessage replaces the content of an existing message.
	EditMessage(channelID, messageID string, m Message) (*MessageRef, error)
	// DeleteMessage removes a message.
	DeleteMessage(channelID, messageID string) error
	// GetMessage fetches a single message.
	GetMessage(channelID, messageID string) (*MessageRef, error)
	// PinMessage pins a message to its channel.
	PinMessage(channelID, messageID string) error
	// UnpinMessage unpins a message from its channel.
	UnpinMessage(channelID, messageID string) error
	// Typing sends a typing indicator to a channel.
	Typing(channelID string) error

	// Channel fetches a channel or thread by ID.
	Channel(channelID string) (*Channel, error)
	// EditChannel renames a channel or thread.
	EditChannel(channelID, name string) (*Channel, error)
	// GuildChannels lists every channel in a guild.
	GuildChannels(guildID string) ([]Channel, error)

	// Guild fetches a guild by ID.
	Guild(guildID string) (*Guild, error)

	// GuildMember fetches a user's guild-scoped identity.
	GuildMember(guildID, userID string) (*Member, error)
	// GuildMemberWithColor fetches a member along with the color their highest
	// colored role gives them in a channel. The color comes from the gateway
	// state cache, which the fetched member is recorded in first so a member
	// the bot has not seen still resolves. The color is 0 when the state
	// cannot supply one.
	GuildMemberWithColor(guildID, userID, channelID string) (*Member, int, error)
	// GuildRoles lists every role in a guild.
	GuildRoles(guildID string) ([]Role, error)
	// CreateGuildRole creates a new guild role.
	CreateGuildRole(guildID string, params RoleParams) (*Role, error)
	// EditGuildRole updates an existing guild role.
	EditGuildRole(guildID, roleID string, params RoleParams) (*Role, error)
	// DeleteGuildRole deletes a guild role.
	DeleteGuildRole(guildID, roleID string) error
	// AddGuildMemberRole grants a role to a guild member.
	AddGuildMemberRole(guildID, userID, roleID string) error
	// RemoveGuildMemberRole revokes a role from a guild member.
	RemoveGuildMemberRole(guildID, userID, roleID string) error

	// BotUsername is the display name of the account the bot is signed in as.
	// It is empty until the gateway connection reports the account.
	BotUsername() string
	// BotAvatarURL is the bot's own avatar at the requested pixel size, such
	// as "256". It is empty until the gateway connection reports the account.
	BotAvatarURL(size string) string

	// User fetches an account by ID.
	User(userID string) (*User, error)
	// CreateUserChannel opens (or fetches the existing) DM channel with a user.
	CreateUserChannel(userID string) (*Channel, error)
	// UserChannelPermissions computes a user's effective permissions in a channel.
	UserChannelPermissions(userID, channelID string) (Permissions, error)

	// RemoveMessageReaction removes one user's reaction from a message.
	RemoveMessageReaction(channelID, messageID, emojiID, userID string) error

	// StartThread creates a public thread under a channel.
	StartThread(channelID, name string, archiveDurationMinutes int) (*Channel, error)
	// JoinThread adds the bot to a thread.
	JoinThread(channelID string) error
	// ActiveThreads lists the active threads in a guild. Discord's
	// active-threads endpoint is guild-scoped, so callers that want the
	// threads under one channel filter the result on ParentID.
	ActiveThreads(guildID string) ([]Channel, error)

	// SetStatus sets the bot's presence to "Playing <activityName>".
	SetStatus(activityName string) error

	// ApplicationCommands lists the commands currently registered for an app,
	// scoped to a guild (empty guildID means the global command set).
	ApplicationCommands(appID, guildID string) ([]CommandRef, error)
	// DeleteApplicationCommand removes one registered command.
	DeleteApplicationCommand(appID, guildID, commandID string) error
	// BulkOverwriteApplicationCommands replaces an app's entire command set in
	// one call.
	BulkOverwriteApplicationCommands(appID, guildID string, commands []Command) ([]CommandRef, error)

	// ApplicationEmojis lists the emoji uploaded to an application.
	ApplicationEmojis(appID string) ([]Emoji, error)
	// ApplicationEmojiCreate uploads one emoji to an application.
	ApplicationEmojiCreate(appID string, params EmojiParams) (*Emoji, error)
}

// EmojiParams is an emoji upload: a name and a data URI holding the image,
// e.g. "data:image/png;base64,<encoded>".
type EmojiParams struct {
	Name  string
	Image string
}
