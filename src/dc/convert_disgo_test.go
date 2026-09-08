package dc

import (
	"testing"

	"github.com/disgoorg/disgo/discord"
)

// Discord paints a member's name with their highest-positioned role that has a
// color. The order roles arrive in says nothing about their position, so the
// selection has to compare positions rather than trust the sequence.
func TestHighestColoredRoleColor(t *testing.T) {
	cases := []struct {
		name  string
		roles []discord.Role
		want  int
	}{
		{
			name:  "no roles",
			roles: nil,
			want:  0,
		},
		{
			name: "one colored role",
			roles: []discord.Role{
				{Position: 3, Color: 0x00ff00},
			},
			want: 0x00ff00,
		},
		{
			name: "highest position wins, whatever the order",
			roles: []discord.Role{
				{Position: 1, Color: 0x111111},
				{Position: 9, Color: 0x999999},
				{Position: 5, Color: 0x555555},
			},
			want: 0x999999,
		},
		{
			// The bug this replaced took the last colored role it saw, so a
			// list ending on a low-positioned role returned the wrong color.
			name: "highest position wins when it comes first",
			roles: []discord.Role{
				{Position: 9, Color: 0x999999},
				{Position: 1, Color: 0x111111},
			},
			want: 0x999999,
		},
		{
			// A hoisted organisational role with no color of its own must not
			// blank out the color of a role below it.
			name: "an uncolored role never wins, however high",
			roles: []discord.Role{
				{Position: 20, Color: 0},
				{Position: 2, Color: 0x222222},
			},
			want: 0x222222,
		},
		{
			name: "no colored roles at all",
			roles: []discord.Role{
				{Position: 4, Color: 0},
				{Position: 7, Color: 0},
			},
			want: 0,
		},
		{
			// @everyone sits at position 0, so the comparison cannot start
			// there or its color would be unreachable.
			name: "the everyone role can supply the color",
			roles: []discord.Role{
				{Position: 0, Color: 0x0000ff},
			},
			want: 0x0000ff,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := highestColoredRoleColor(tc.roles); got != tc.want {
				t.Errorf("highestColoredRoleColor() = %#06x, want %#06x", got, tc.want)
			}
		})
	}
}
