package dc

import "github.com/disgoorg/disgo/discord"

// LayoutComponent is a component that can sit at the top level of a message.
type LayoutComponent interface {
	layoutComponent()
}

// ContainerSubComponent is a component that can sit inside a Container.
type ContainerSubComponent interface {
	containerSubComponent()
}

// InteractiveComponent is a component that can sit inside an ActionRow, such
// as a Button or a SelectMenu.
type InteractiveComponent interface {
	interactiveComponent()
}

// SectionAccessory is a component that can appear beside a Section's text —
// either a Thumbnail image or a Button.
type SectionAccessory interface {
	sectionAccessory()
}

// TextDisplay renders markdown text as a components v2 element.
type TextDisplay struct {
	Content string
}

func (TextDisplay) containerSubComponent() {}

// Container groups components behind an optional colored accent bar. An
// AccentColor of 0 leaves the bar uncolored.
type Container struct {
	AccentColor int
	Components  []ContainerSubComponent
}

// Emoji identifies a Discord emoji shown on a component, such as a Button or
// a SelectOption.
type Emoji struct {
	Name     string
	ID       string
	Animated bool
}

// ButtonStyle is the visual style of a Button.
type ButtonStyle int

// Button styles.
const (
	ButtonPrimary ButtonStyle = iota
	ButtonSecondary
	ButtonSuccess
	ButtonDanger
	ButtonLink
)

// Button is a clickable component that lives inside an ActionRow, or as a
// Section's accessory.
//
// A Button styled ButtonLink carries a URL instead of a CustomID; Discord
// rejects a button that sets both.
type Button struct {
	Label    string
	CustomID string
	Style    ButtonStyle
	URL      string
	Disabled bool
	Emoji    *Emoji
}

func (Button) interactiveComponent() {}
func (Button) sectionAccessory()     {}

// SelectOption is one choice in a SelectMenu.
type SelectOption struct {
	Label       string
	Value       string
	Description string
	Default     bool
	Emoji       *Emoji
}

// SelectMenu is a dropdown component that lives inside an ActionRow.
//
// MinValues is a pointer because Discord distinguishes "unset", which it
// defaults to 1, from an explicit 0, which is how a menu is made fully
// deselectable. A plain int cannot express that difference.
type SelectMenu struct {
	CustomID    string
	Placeholder string
	MinValues   *int
	MaxValues   int
	Disabled    bool
	Options     []SelectOption
}

func (SelectMenu) interactiveComponent() {}

// ActionRow is a row of up to five interactive components. Unlike most
// components here it can appear both at the top level of a message and
// nested inside a Container.
type ActionRow struct {
	Components []InteractiveComponent
}

// SeparatorSpacing sizes the gap a Separator leaves around itself.
type SeparatorSpacing int

// Separator spacing sizes.
const (
	SeparatorSpacingSmall SeparatorSpacing = iota
	SeparatorSpacingLarge
)

// Separator is a visual divider between components.
type Separator struct {
	Divider bool
	Spacing SeparatorSpacing
}

func (Separator) containerSubComponent() {}

// Thumbnail is a small image accessory for a Section.
type Thumbnail struct {
	URL         string
	Description string
	Spoiler     bool
}

func (Thumbnail) sectionAccessory() {}

// Section pairs up to three TextDisplay components with an accessory shown
// beside them — a Thumbnail image or a Button.
type Section struct {
	Components []TextDisplay
	Accessory  SectionAccessory
}

func (Section) containerSubComponent() {}

// MediaItem is one image or video inside a MediaGallery.
type MediaItem struct {
	URL         string
	Description string
	Spoiler     bool
}

// MediaGallery displays a grid of images or videos.
type MediaGallery struct {
	Items []MediaItem
}

func (MediaGallery) containerSubComponent() {}

// rawComponent carries a component read back off an existing message. It is
// opaque: the facade cannot inspect or rebuild it, only send it again
// unchanged. ComponentEvent.MessageComponentsWithoutActionRows returns these.
type rawComponent struct {
	component discord.LayoutComponent
}

func (rawComponent) containerSubComponent() {}

// The markers below decide where each component may sit. They mirror the
// nesting rules disgo enforces: a component Discord rejects at the top level
// of a message, such as a bare Button, cannot be passed as a LayoutComponent
// at all. The facade's interfaces used to ask only for a rendering method,
// which every component had, so the compiler could not tell them apart.

func (TextDisplay) layoutComponent()  {}
func (Container) layoutComponent()    {}
func (ActionRow) layoutComponent()    {}
func (Section) layoutComponent()      {}
func (Separator) layoutComponent()    {}
func (MediaGallery) layoutComponent() {}
func (rawComponent) layoutComponent() {}

func (ActionRow) containerSubComponent() {}
