package dc

import "time"

// MessageRef is a Discord message returned by a Client call: the result of
// sending, editing, or fetching one.
type MessageRef struct {
	ID        string
	ChannelID string
	Content   string
	Author    *User
	Timestamp time.Time
}

// Channel is a Discord channel or thread.
type Channel struct {
	ID            string
	GuildID       string
	Name          string
	ParentID      string
	LastMessageID string
	IsThread      bool

	// IsCategory is true for the container channels other channels are
	// grouped under.
	IsCategory bool
}

// Mention is the markdown that links to this channel.
func (c Channel) Mention() string {
	return "<#" + c.ID + ">"
}

// Member is a user's guild-scoped identity: their nickname and roles in one
// specific guild, as opposed to the account-wide fields on User.
type Member struct {
	UserID string
	Nick   string
	Roles  []string

	// PremiumSince is when the member started boosting the guild (Discord's
	// Nitro server boost, unrelated to this bot's contract boosting). It is
	// nil for a member who is not boosting.
	PremiumSince *time.Time

	// User is the account behind the membership. It is nil when Discord sent
	// the membership without one.
	User *User
}

// Role is a guild role.
type Role struct {
	ID          string
	Name        string
	Color       int
	Position    int
	Managed     bool
	Mentionable bool
	Hoist       bool
}

// Mention is the markdown that pings this role.
func (r Role) Mention() string {
	return "<@&" + r.ID + ">"
}

// RoleParams is the set of fields used to create or edit a guild role. A nil
// Mentionable leaves the role's current mentionable setting unchanged.
type RoleParams struct {
	Name        string
	Mentionable *bool
}

// User is a Discord account.
type User struct {
	ID         string
	Username   string
	GlobalName string
	Bot        bool

	// Discriminator is the four-digit tag from the legacy username system,
	// and "0" for an account migrated to the new one.
	Discriminator string
}

// Mention formats the user as a Discord mention string.
func (u User) Mention() string {
	return "<@" + u.ID + ">"
}

// String is the account's handle: the bare username for an account migrated
// off the legacy username system, and "username#discriminator" for one still
// on it. This bot stores the result in saved contracts, so the two forms have
// to keep matching what discordgo produced.
func (u User) String() string {
	if u.Discriminator == "0" {
		return u.Username
	}
	return u.Username + "#" + u.Discriminator
}

// Guild is a Discord server.
type Guild struct {
	ID      string
	Name    string
	OwnerID string
}

// Permissions is a Discord permission bitmask for a user in a channel, as
// returned by Client.UserChannelPermissions.
type Permissions int64

// Discord permission bits this bot checks. Administrator implicitly grants
// every other permission.
const (
	permissionSendMessages        Permissions = 1 << 11
	permissionAdministrator       Permissions = 1 << 3
	permissionCreatePublicThreads Permissions = 1 << 35
)

// Administrator reports whether the permission set includes Administrator.
func (p Permissions) Administrator() bool {
	return p&permissionAdministrator != 0
}

// SendMessages reports whether the permission set includes Send Messages.
func (p Permissions) SendMessages() bool {
	return p&permissionSendMessages != 0
}

// CreatePublicThreads reports whether the permission set includes Create
// Public Threads.
func (p Permissions) CreatePublicThreads() bool {
	return p&permissionCreatePublicThreads != 0
}

// CommandRef identifies a slash command already registered with Discord, as
// returned by Client.ApplicationCommands and
// Client.BulkOverwriteApplicationCommands. Unlike Command, which describes a
// command definition to be published, CommandRef carries the ID Discord
// assigned it.
type CommandRef struct {
	ID   string
	Name string
}

// Attachment is a file a user attached to a command or a message.
type Attachment struct {
	ID          string
	Filename    string
	URL         string
	ProxyURL    string
	ContentType string
	Size        int
}
