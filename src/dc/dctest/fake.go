// Package dctest provides test doubles for the dc facade.
package dctest

import (
	"strconv"
	"sync"

	"github.com/mkmccarty/TokenTimeBoostBot/src/dc"
)

// ErrNotFound is what the fake returns for an ID the test never registered.
var ErrNotFound = &dc.APIError{StatusCode: 404, Message: "not found"}

// Call is one recorded client call: the method name and its arguments in
// declaration order. Tests assert on the recorded sequence rather than on a
// mock Discord session.
type Call struct {
	Method string
	Args   []string
}

// FakeClient is a dc.Client that records what was asked of it and answers
// from fields the test sets.
//
// The embedded dc.Client is nil on purpose: it satisfies the interface at
// compile time, so a test only overrides what it exercises, and any
// unimplemented method panics loudly instead of failing quietly.
type FakeClient struct {
	dc.Client

	mu sync.Mutex
	// Calls is every recorded call, in order.
	Calls []Call

	// Guilds answers Guild by ID. A missing ID yields ErrNotFound.
	Guilds map[string]*dc.Guild
	// Channels answers Channel by ID. A missing ID yields ErrNotFound.
	Channels map[string]*dc.Channel
	// Permissions answers UserChannelPermissions, keyed "userID:channelID".
	Permissions map[string]dc.Permissions
	// Users answers User by ID. A missing ID yields ErrNotFound, which is
	// what a user the bot cannot see looks like.
	Users map[string]*dc.User
	// Members answers GuildMember and GuildMemberWithColor, keyed
	// "guildID:userID". A missing pair yields ErrNotFound.
	Members map[string]*dc.Member
	// MemberColors answers the color half of GuildMemberWithColor, keyed the
	// same way as Members.
	MemberColors map[string]int
	// Threads answers ActiveThreads by guild ID. A missing ID yields
	// ErrNotFound.
	Threads map[string][]dc.Channel

	// SendErr, EditErr and DeleteErr are returned by the message methods.
	SendErr   error
	EditErr   error
	DeleteErr error

	// NextMessageID is the ID handed back by SendMessage. It is suffixed with
	// the call count so repeated sends do not collide.
	NextMessageID string
}

// New returns a FakeClient with empty lookup tables.
func New() *FakeClient {
	return &FakeClient{
		Guilds:        map[string]*dc.Guild{},
		Channels:      map[string]*dc.Channel{},
		Permissions:   map[string]dc.Permissions{},
		Users:         map[string]*dc.User{},
		Members:       map[string]*dc.Member{},
		MemberColors:  map[string]int{},
		Threads:       map[string][]dc.Channel{},
		NextMessageID: "message",
	}
}

// WithGuild registers a guild the fake will return from Guild.
func (f *FakeClient) WithGuild(id, name string) *FakeClient {
	f.Guilds[id] = &dc.Guild{ID: id, Name: name}
	return f
}

// WithChannel registers a channel the fake will return from Channel.
func (f *FakeClient) WithChannel(id, guildID, name string) *FakeClient {
	f.Channels[id] = &dc.Channel{ID: id, GuildID: guildID, Name: name}
	return f
}

// WithThread registers an active thread the fake will return from
// ActiveThreads for the thread's guild.
func (f *FakeClient) WithThread(id, guildID, parentID, name string) *FakeClient {
	f.Threads[guildID] = append(f.Threads[guildID], dc.Channel{
		ID:       id,
		GuildID:  guildID,
		ParentID: parentID,
		Name:     name,
		IsThread: true,
	})
	return f
}

// WithPermissions sets what UserChannelPermissions reports for one user in
// one channel.
func (f *FakeClient) WithPermissions(userID, channelID string, perms dc.Permissions) *FakeClient {
	f.Permissions[userID+":"+channelID] = perms
	return f
}

// WithUser registers a user the fake will return from User.
func (f *FakeClient) WithUser(id, username, globalName string) *FakeClient {
	f.Users[id] = &dc.User{ID: id, Username: username, GlobalName: globalName}
	return f
}

// WithMember registers a guild member, and the color GuildMemberWithColor
// reports for them.
func (f *FakeClient) WithMember(guildID, userID, nick string, color int) *FakeClient {
	user, ok := f.Users[userID]
	if !ok {
		user = &dc.User{ID: userID}
	}
	f.Members[guildID+":"+userID] = &dc.Member{UserID: userID, Nick: nick, User: user}
	f.MemberColors[guildID+":"+userID] = color
	return f
}

// Called reports whether the named method was called at least once.
func (f *FakeClient) Called(method string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.Calls {
		if c.Method == method {
			return true
		}
	}
	return false
}

// CallsTo returns every recorded call to the named method.
func (f *FakeClient) CallsTo(method string) []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Call
	for _, c := range f.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// ResetCalls clears all recorded calls.
func (f *FakeClient) ResetCalls() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = nil
}

func (f *FakeClient) record(method string, args ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, Call{Method: method, Args: args})
}

// Guild returns a registered guild, or ErrNotFound.
func (f *FakeClient) Guild(guildID string) (*dc.Guild, error) {
	f.record("Guild", guildID)
	if g, ok := f.Guilds[guildID]; ok {
		return g, nil
	}
	return nil, ErrNotFound
}

// Channel returns a registered channel, or ErrNotFound.
func (f *FakeClient) Channel(channelID string) (*dc.Channel, error) {
	f.record("Channel", channelID)
	if c, ok := f.Channels[channelID]; ok {
		return c, nil
	}
	return nil, ErrNotFound
}

// UserChannelPermissions returns the permissions registered for the pair, or
// zero when the test did not register any.
func (f *FakeClient) UserChannelPermissions(userID, channelID string) (dc.Permissions, error) {
	f.record("UserChannelPermissions", userID, channelID)
	return f.Permissions[userID+":"+channelID], nil
}

// User returns a registered user, or ErrNotFound.
func (f *FakeClient) User(userID string) (*dc.User, error) {
	f.record("User", userID)
	if u, ok := f.Users[userID]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}

// CreateUserChannel returns a DM channel for the user, recording the call.
func (f *FakeClient) CreateUserChannel(userID string) (*dc.Channel, error) {
	f.record("CreateUserChannel", userID)
	dmChannelID := "dm-" + userID
	if c, ok := f.Channels[dmChannelID]; ok {
		return c, nil
	}
	return &dc.Channel{
		ID:   dmChannelID,
		Name: "DM with " + userID,
	}, nil
}

// GuildMember returns a registered member, or ErrNotFound.
func (f *FakeClient) GuildMember(guildID, userID string) (*dc.Member, error) {
	f.record("GuildMember", guildID, userID)
	if m, ok := f.Members[guildID+":"+userID]; ok {
		return m, nil
	}
	return nil, ErrNotFound
}

// GuildMemberWithColor returns a registered member and their role color, or
// ErrNotFound.
func (f *FakeClient) GuildMemberWithColor(guildID, userID, channelID string) (*dc.Member, int, error) {
	f.record("GuildMemberWithColor", guildID, userID, channelID)
	m, ok := f.Members[guildID+":"+userID]
	if !ok {
		return nil, 0, ErrNotFound
	}
	return m, f.MemberColors[guildID+":"+userID], nil
}

// SendMessage records the send and returns a reference with a fresh ID.
func (f *FakeClient) SendMessage(channelID string, m dc.Message) (*dc.MessageRef, error) {
	f.record("SendMessage", channelID, m.Content)
	if f.SendErr != nil {
		return nil, f.SendErr
	}
	return &dc.MessageRef{ID: f.nextID(), ChannelID: channelID, Content: m.Content}, nil
}

// EditMessage records the edit and echoes the new content back.
func (f *FakeClient) EditMessage(channelID, messageID string, m dc.Message) (*dc.MessageRef, error) {
	f.record("EditMessage", channelID, messageID, m.Content)
	if f.EditErr != nil {
		return nil, f.EditErr
	}
	return &dc.MessageRef{ID: messageID, ChannelID: channelID, Content: m.Content}, nil
}

// DeleteMessage records the delete.
func (f *FakeClient) DeleteMessage(channelID, messageID string) error {
	f.record("DeleteMessage", channelID, messageID)
	return f.DeleteErr
}

// CreateGuildRole records the request and hands back a role with a fresh ID,
// which is what a guild that accepted the create looks like.
func (f *FakeClient) CreateGuildRole(guildID string, params dc.RoleParams) (*dc.Role, error) {
	f.record("CreateGuildRole", guildID, params.Name)
	f.mu.Lock()
	count := len(f.Calls)
	f.mu.Unlock()
	return &dc.Role{ID: "role-" + strconv.Itoa(count), Name: params.Name}, nil
}

// AddGuildMemberRole records the role grant.
func (f *FakeClient) AddGuildMemberRole(guildID, userID, roleID string) error {
	f.record("AddGuildMemberRole", guildID, userID, roleID)
	return nil
}

// RemoveGuildMemberRole records the role removal.
func (f *FakeClient) RemoveGuildMemberRole(guildID, userID, roleID string) error {
	f.record("RemoveGuildMemberRole", guildID, userID, roleID)
	return nil
}

// DeleteGuildRole records the role delete.
func (f *FakeClient) DeleteGuildRole(guildID, roleID string) error {
	f.record("DeleteGuildRole", guildID, roleID)
	return nil
}

// GuildRoles reports no roles, which is what a guild the test did not set up
// looks like.
func (f *FakeClient) GuildRoles(guildID string) ([]dc.Role, error) {
	f.record("GuildRoles", guildID)
	return nil, nil
}

func (f *FakeClient) nextID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.NextMessageID + "-" + strconv.Itoa(len(f.Calls))
}

// ActiveThreads answers from Threads, which is keyed by guild ID. A guild the
// test never registered yields ErrNotFound, which is what Discord's
// guild-scoped endpoint returns for an ID that is not a guild.
func (f *FakeClient) ActiveThreads(guildID string) ([]dc.Channel, error) {
	f.record("ActiveThreads", guildID)
	threads, ok := f.Threads[guildID]
	if !ok {
		return nil, ErrNotFound
	}
	return threads, nil
}
