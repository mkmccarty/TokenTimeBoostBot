package dc

import (
	"testing"

	"github.com/disgoorg/disgo/discord"
)

// These cover the parts of the disgo rendering the message parity test cannot:
// conversions that have no discordgo counterpart to compare against, and the
// cases where the facade's component interfaces are looser than disgo's.

func TestDisgoEmojiParsesCustomID(t *testing.T) {
	custom := (&Emoji{Name: "token", ID: "1234567890123456789", Animated: true}).disgoEmoji()
	if custom.ID.String() != "1234567890123456789" {
		t.Errorf("custom emoji id = %s, want 1234567890123456789", custom.ID)
	}
	if custom.Name != "token" || !custom.Animated {
		t.Errorf("custom emoji fields wrong: %+v", custom)
	}

	unicode := (&Emoji{Name: "🚀"}).disgoEmoji()
	if unicode.ID != 0 {
		t.Errorf("unicode emoji should have no id, got %s", unicode.ID)
	}

	// A malformed id is left at zero rather than failing the whole render.
	malformed := (&Emoji{Name: "broken", ID: "not-a-snowflake"}).disgoEmoji()
	if malformed.ID != 0 {
		t.Errorf("malformed id should be dropped, got %s", malformed.ID)
	}

	var absent *Emoji
	if absent.disgoEmoji() != nil {
		t.Error("a nil emoji should render as nil")
	}
}

// A component Discord does not accept at the top level of a message cannot be
// passed as a LayoutComponent at all: the marker interfaces make it a compile
// error rather than something to check for at runtime. This test pins the
// other half — that everything Discord does accept converts.
func TestDisgoLayoutAcceptsEveryTopLevelComponent(t *testing.T) {
	topLevel := []LayoutComponent{
		TextDisplay{Content: "x"},
		Container{},
		ActionRow{},
		Section{},
		Separator{},
		MediaGallery{},
	}
	for _, component := range topLevel {
		if got := disgoLayout(component); got == nil {
			t.Errorf("%T did not render as a layout component", component)
		}
	}
}

// A component read back off a message is re-sent unchanged: the facade holds
// it opaquely rather than trying to rebuild it.
func TestRawComponentIsPassedThroughUnchanged(t *testing.T) {
	original := discord.NewContainer(
		discord.NewTextDisplay("carried over"),
		discord.NewActionRow(discord.NewPrimaryButton("Join", "fd_join#hash")),
	)

	converted := disgoLayout(rawComponent{component: original})
	container, ok := converted.(discord.ContainerComponent)
	if !ok {
		t.Fatalf("raw container rendered as %T", converted)
	}
	if len(container.Components) != 2 {
		t.Fatalf("want 2 nested components, got %d", len(container.Components))
	}
	text, ok := container.Components[0].(discord.TextDisplayComponent)
	if !ok || text.Content != "carried over" {
		t.Fatalf("nested text wrong: %#v", container.Components[0])
	}
}

func TestRawComponentWithNothingToRender(t *testing.T) {
	if got := disgoLayout(rawComponent{}); got != nil {
		t.Errorf("an empty raw component rendered as %#v", got)
	}
}

// disgo numbers both enums from 1, where the facade numbers its own from 0, so
// a missing conversion would silently shift every value.
func TestDisgoEnumsAreNotFacadeValues(t *testing.T) {
	if ButtonPrimary.disgoStyle() != discord.ButtonStylePrimary {
		t.Errorf("primary button = %d, want %d", ButtonPrimary.disgoStyle(), discord.ButtonStylePrimary)
	}
	if ButtonSecondary.disgoStyle() != discord.ButtonStyleSecondary {
		t.Errorf("secondary button = %d, want %d", ButtonSecondary.disgoStyle(), discord.ButtonStyleSecondary)
	}
	if int(ButtonPrimary) == int(discord.ButtonStylePrimary) {
		t.Error("the facade and disgo button styles happen to line up; this test no longer proves anything")
	}
	if SeparatorSpacingSmall.disgoSpacing() != discord.SeparatorSpacingSizeSmall {
		t.Errorf("small spacing = %d, want %d", SeparatorSpacingSmall.disgoSpacing(), discord.SeparatorSpacingSizeSmall)
	}
	if SeparatorSpacingLarge.disgoSpacing() != discord.SeparatorSpacingSizeLarge {
		t.Errorf("large spacing = %d, want %d", SeparatorSpacingLarge.disgoSpacing(), discord.SeparatorSpacingSizeLarge)
	}
}
