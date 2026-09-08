package dc

import (
	"testing"

	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// Discord's permission rules, applied to values fetched from anywhere. The
// cache path delegates to disgo; this is the fallback that runs when the
// channel is not cached, so it has to agree with disgo's answer.
func TestMemberChannelPermissions(t *testing.T) {
	const (
		guildID  snowflake.ID = 100 // also the @everyone role ID
		ownerID  snowflake.ID = 1
		userID   snowflake.ID = 2
		roleMod  snowflake.ID = 200
		roleTeam snowflake.ID = 300
	)

	everyone := discord.Role{ID: guildID, Permissions: discord.PermissionViewChannel | discord.PermissionSendMessages}

	cases := []struct {
		name       string
		userID     snowflake.ID
		roles      []discord.Role
		memberRole []snowflake.ID
		overwrites discord.PermissionOverwrites
		want       discord.Permissions
	}{
		{
			name:   "the owner has everything",
			userID: ownerID,
			roles:  []discord.Role{everyone},
			want:   discord.PermissionsAll,
		},
		{
			// Administrator ignores overwrites entirely, so a channel that
			// denies everyone still grants an admin.
			name:   "administrator ignores a denying overwrite",
			userID: userID,
			roles: []discord.Role{
				everyone,
				{ID: roleMod, Permissions: discord.PermissionAdministrator},
			},
			memberRole: []snowflake.ID{roleMod},
			overwrites: discord.PermissionOverwrites{
				discord.RolePermissionOverwrite{RoleID: guildID, Deny: discord.PermissionViewChannel},
			},
			want: discord.PermissionsAll,
		},
		{
			name:   "base permissions come from the everyone role",
			userID: userID,
			roles:  []discord.Role{everyone},
			want:   discord.PermissionViewChannel | discord.PermissionSendMessages,
		},
		{
			name:   "a member role adds to the base",
			userID: userID,
			roles: []discord.Role{
				everyone,
				{ID: roleTeam, Permissions: discord.PermissionCreatePublicThreads},
			},
			memberRole: []snowflake.ID{roleTeam},
			want:       discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionCreatePublicThreads,
		},
		{
			name:   "the everyone overwrite can take a permission away",
			userID: userID,
			roles:  []discord.Role{everyone},
			overwrites: discord.PermissionOverwrites{
				discord.RolePermissionOverwrite{RoleID: guildID, Deny: discord.PermissionSendMessages},
			},
			want: discord.PermissionViewChannel,
		},
		{
			// Role overwrites are applied after the everyone overwrite, so a
			// role allow restores what everyone was denied.
			name:   "a role overwrite restores what everyone was denied",
			userID: userID,
			roles: []discord.Role{
				everyone,
				{ID: roleTeam},
			},
			memberRole: []snowflake.ID{roleTeam},
			overwrites: discord.PermissionOverwrites{
				discord.RolePermissionOverwrite{RoleID: guildID, Deny: discord.PermissionSendMessages},
				discord.RolePermissionOverwrite{RoleID: roleTeam, Allow: discord.PermissionSendMessages},
			},
			want: discord.PermissionViewChannel | discord.PermissionSendMessages,
		},
		{
			// The member overwrite is applied last and beats the roles.
			name:   "a member overwrite beats a role allow",
			userID: userID,
			roles: []discord.Role{
				everyone,
				{ID: roleTeam},
			},
			memberRole: []snowflake.ID{roleTeam},
			overwrites: discord.PermissionOverwrites{
				discord.RolePermissionOverwrite{RoleID: roleTeam, Allow: discord.PermissionSendMessages},
				discord.MemberPermissionOverwrite{UserID: userID, Deny: discord.PermissionSendMessages},
			},
			want: discord.PermissionViewChannel,
		},
		{
			// Role overwrites are unioned before being applied, so an allow on
			// one role survives a deny on another.
			name:   "role allows and denies are unioned, not applied in order",
			userID: userID,
			roles: []discord.Role{
				everyone,
				{ID: roleMod},
				{ID: roleTeam},
			},
			memberRole: []snowflake.ID{roleMod, roleTeam},
			overwrites: discord.PermissionOverwrites{
				discord.RolePermissionOverwrite{RoleID: roleMod, Deny: discord.PermissionSendMessages},
				discord.RolePermissionOverwrite{RoleID: roleTeam, Allow: discord.PermissionSendMessages},
			},
			want: discord.PermissionViewChannel | discord.PermissionSendMessages,
		},
		{
			name:   "a role the member does not hold is ignored",
			userID: userID,
			roles: []discord.Role{
				everyone,
				{ID: roleMod, Permissions: discord.PermissionAdministrator},
			},
			want: discord.PermissionViewChannel | discord.PermissionSendMessages,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := memberChannelPermissions(guildID, ownerID, tc.userID, tc.roles, tc.memberRole, tc.overwrites)
			if got != tc.want {
				t.Errorf("permissions = %d, want %d", got, tc.want)
			}
		})
	}
}

// The facade's own permission helpers read the bits this computation produces,
// so a wrong bit here would read as a wrong answer there.
func TestComputedPermissionsMatchTheFacadeAccessors(t *testing.T) {
	const guildID snowflake.ID = 100

	perms := Permissions(memberChannelPermissions(
		guildID, 1, 2,
		[]discord.Role{{ID: guildID, Permissions: discord.PermissionSendMessages | discord.PermissionCreatePublicThreads}},
		nil,
		nil,
	))

	if !perms.SendMessages() {
		t.Error("SendMessages() should be true")
	}
	if !perms.CreatePublicThreads() {
		t.Error("CreatePublicThreads() should be true")
	}
	if perms.Administrator() {
		t.Error("Administrator() should be false")
	}
}

// The fallback and the cached path must agree, or a member's permissions would
// depend on whether the channel happened to still be cached — which for a
// contract thread means whether it had archived yet.
func TestFallbackAgreesWithTheCachedComputation(t *testing.T) {
	const (
		guildID  snowflake.ID = 100
		ownerID  snowflake.ID = 1
		userID   snowflake.ID = 2
		roleTeam snowflake.ID = 300
	)

	everyone := discord.Role{ID: guildID, GuildID: guildID, Permissions: discord.PermissionViewChannel | discord.PermissionSendMessages}
	team := discord.Role{ID: roleTeam, GuildID: guildID, Permissions: discord.PermissionCreatePublicThreads}
	overwrites := discord.PermissionOverwrites{
		discord.RolePermissionOverwrite{RoleID: guildID, Deny: discord.PermissionSendMessages},
		discord.RolePermissionOverwrite{RoleID: roleTeam, Allow: discord.PermissionSendMessages},
	}
	// The overwrites live in an unexported field, so the channel is built the
	// way the gateway builds one: from Discord's own JSON.
	channel := decode[discord.GuildTextChannel](t, `{
		"id": "400", "type": 0, "guild_id": "100", "name": "contract",
		"permission_overwrites": [
			{"id": "100", "type": 0, "allow": "0", "deny": "2048"},
			{"id": "300", "type": 0, "allow": "2048", "deny": "0"}
		]
	}`)
	member := discord.Member{
		GuildID: guildID,
		User:    discord.User{ID: userID},
		RoleIDs: []snowflake.ID{roleTeam},
	}

	caches := cache.New(cache.WithCaches(cacheFlags))
	caches.AddGuild(discord.Guild{ID: guildID, OwnerID: ownerID})
	caches.AddRole(everyone)
	caches.AddRole(team)
	caches.AddMember(member)

	cached := caches.MemberPermissionsInChannel(channel, member)
	fallback := memberChannelPermissions(guildID, ownerID, userID, []discord.Role{everyone, team}, member.RoleIDs, overwrites)

	if cached != fallback {
		t.Errorf("cached path = %d, REST fallback = %d; the two must agree", cached, fallback)
	}

	// Pinned explicitly so the agreement above cannot be satisfied by both
	// paths returning nothing: everyone is denied Send Messages and the team
	// role allows it back, on top of the permissions the roles carry.
	want := discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionCreatePublicThreads
	if cached != want {
		t.Errorf("cached path = %d, want %d", cached, want)
	}
}
