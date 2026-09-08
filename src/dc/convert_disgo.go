package dc

import (
	"strconv"

	"github.com/disgoorg/disgo/discord"
)

// Converters from disgo's types into the facade's. They live together because
// they are the one place the two vocabularies meet: disgo models IDs as
// snowflakes and channels as an interface hierarchy, and the facade models
// both as plain data.

func messageRefFrom(msg *discord.Message) *MessageRef {
	if msg == nil {
		return nil
	}
	return &MessageRef{
		ID:        idString(msg.ID),
		ChannelID: idString(msg.ChannelID),
		Content:   msg.Content,
		Author:    userFrom(&msg.Author),
		Timestamp: msg.CreatedAt,
	}
}

func userFrom(u *discord.User) *User {
	if u == nil {
		return nil
	}
	globalName := ""
	if u.GlobalName != nil {
		globalName = *u.GlobalName
	}
	return &User{
		ID:            idString(u.ID),
		Username:      u.Username,
		GlobalName:    globalName,
		Bot:           u.Bot,
		Discriminator: u.Discriminator,
	}
}

// channelFrom flattens disgo's channel interface hierarchy into the facade's
// one struct. The optional interfaces are asserted rather than switched on
// every concrete type, so a channel kind disgo adds later still converts.
func channelFrom(ch discord.Channel) *Channel {
	if ch == nil {
		return nil
	}
	out := &Channel{
		ID:         idString(ch.ID()),
		Name:       ch.Name(),
		IsCategory: ch.Type() == discord.ChannelTypeGuildCategory,
	}
	switch ch.Type() {
	case discord.ChannelTypeGuildPublicThread,
		discord.ChannelTypeGuildPrivateThread,
		discord.ChannelTypeGuildNewsThread:
		out.IsThread = true
	}
	if guildChannel, ok := ch.(discord.GuildChannel); ok {
		out.GuildID = idString(guildChannel.GuildID())
		if parentID := guildChannel.ParentID(); parentID != nil {
			out.ParentID = idString(*parentID)
		}
	}
	if messageChannel, ok := ch.(discord.GuildMessageChannel); ok {
		if lastID := messageChannel.LastMessageID(); lastID != nil {
			out.LastMessageID = idString(*lastID)
		}
	}
	return out
}

// resolvedMemberFrom converts the membership Discord resolves alongside an
// interaction, which carries the member plus the permissions it computed.
func resolvedMemberFrom(m *discord.ResolvedMember) *Member {
	if m == nil {
		return nil
	}
	return memberFrom(&m.Member)
}

func memberFrom(m *discord.Member) *Member {
	if m == nil {
		return nil
	}
	member := &Member{
		UserID:       idString(m.User.ID),
		User:         userFrom(&m.User),
		PremiumSince: m.PremiumSince,
	}
	if m.Nick != nil {
		member.Nick = *m.Nick
	}
	for _, roleID := range m.RoleIDs {
		member.Roles = append(member.Roles, idString(roleID))
	}
	return member
}

func roleFrom(r *discord.Role) *Role {
	if r == nil {
		return nil
	}
	return &Role{
		ID:          idString(r.ID),
		Name:        r.Name,
		Color:       r.Color,
		Position:    r.Position,
		Managed:     r.Managed,
		Mentionable: r.Mentionable,
		Hoist:       r.Hoist,
	}
}

func attachmentFrom(att *discord.Attachment) *Attachment {
	if att == nil {
		return nil
	}
	out := &Attachment{
		ID:       idString(att.ID),
		Filename: att.Filename,
		URL:      att.URL,
		ProxyURL: att.ProxyURL,
		Size:     att.Size,
	}
	if att.ContentType != nil {
		out.ContentType = *att.ContentType
	}
	return out
}

func commandRefsFrom(cmds []discord.ApplicationCommand) []CommandRef {
	out := make([]CommandRef, 0, len(cmds))
	for _, cmd := range cmds {
		out = append(out, CommandRef{ID: idString(cmd.ID()), Name: cmd.Name()})
	}
	return out
}

// avatarURL renders the bot's own avatar at a pixel size. The facade passes
// the size as a string because that is the shape its callers already had.
func avatarURL(u discord.User, size string) string {
	pixels, err := strconv.Atoi(size)
	if err != nil || pixels <= 0 {
		return u.EffectiveAvatarURL()
	}
	return u.EffectiveAvatarURL(discord.WithSize(pixels))
}

// autoArchiveDuration maps a thread's archive window from the minutes the
// facade takes to the fixed set Discord accepts, rounding up to the next
// allowed value so a thread never archives sooner than asked.
func autoArchiveDuration(minutes int) discord.AutoArchiveDuration {
	switch {
	case minutes <= 60:
		return discord.AutoArchiveDuration1h
	case minutes <= 1440:
		return discord.AutoArchiveDuration24h
	case minutes <= 4320:
		return discord.AutoArchiveDuration3d
	default:
		return discord.AutoArchiveDuration1w
	}
}

// highestColoredRoleColor is the color Discord paints a member's name: the
// color of their highest-positioned role that has one.
//
// A role whose color is 0 has no color of its own and never wins, however high
// it sits — that is how a hoisted organisational role leaves the name colored
// by something lower down. Position is compared against the best seen so far
// rather than against zero, so the @everyone role at position 0 can still
// supply the color when it is the only colored role.
func highestColoredRoleColor(roles []discord.Role) int {
	color := 0
	highest := -1
	for _, role := range roles {
		if role.Color == 0 || role.Position <= highest {
			continue
		}
		highest = role.Position
		color = role.Color
	}
	return color
}
