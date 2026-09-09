package dc

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// This file renders the facade's component types into disgo's. It is the disgo
// half of component.go, which still renders the discordgo half until the REST
// client and the interaction events stop speaking discordgo.
//
// The conversions are free functions with a type switch rather than methods on
// the facade's interfaces, because those interfaces are structural and looser
// than disgo's: every facade component satisfies LayoutComponent, while disgo
// reserves that role for the components Discord actually allows at the top
// level of a message. A Button, for instance, is a disgo InteractiveComponent
// and nothing else. Converting through a type switch keeps that distinction
// where disgo puts it instead of forcing a Button to claim it is a layout.

// disgoLayout converts a top-level component. It returns nil for a component
// Discord does not accept at the top level of a message — a bare Button, say —
// which the message renderer drops rather than sending a payload Discord would
// reject outright.
func disgoLayout(c LayoutComponent) discord.LayoutComponent {
	switch component := c.(type) {
	case TextDisplay:
		return component.disgoTextDisplay()
	case *TextDisplay:
		if component != nil {
			return component.disgoTextDisplay()
		}
		return nil
	case Container:
		return component.disgoContainer()
	case *Container:
		if component != nil {
			return component.disgoContainer()
		}
		return nil
	case ActionRow:
		return component.disgoActionRow()
	case *ActionRow:
		if component != nil {
			return component.disgoActionRow()
		}
		return nil
	case Section:
		return component.disgoSection()
	case *Section:
		if component != nil {
			return component.disgoSection()
		}
		return nil
	case Separator:
		return component.disgoSeparator()
	case *Separator:
		if component != nil {
			return component.disgoSeparator()
		}
		return nil
	case MediaGallery:
		return component.disgoMediaGallery()
	case *MediaGallery:
		if component != nil {
			return component.disgoMediaGallery()
		}
		return nil
	case rawComponent:
		return component.disgoLayout()
	case *rawComponent:
		if component != nil {
			return component.disgoLayout()
		}
		return nil
	default:
		return nil
	}
}

// disgoContainerSub converts a component nested inside a Container.
func disgoContainerSub(c ContainerSubComponent) discord.ContainerSubComponent {
	switch component := c.(type) {
	case TextDisplay:
		return component.disgoTextDisplay()
	case *TextDisplay:
		if component != nil {
			return component.disgoTextDisplay()
		}
		return nil
	case ActionRow:
		return component.disgoActionRow()
	case *ActionRow:
		if component != nil {
			return component.disgoActionRow()
		}
		return nil
	case Section:
		return component.disgoSection()
	case *Section:
		if component != nil {
			return component.disgoSection()
		}
		return nil
	case Separator:
		return component.disgoSeparator()
	case *Separator:
		if component != nil {
			return component.disgoSeparator()
		}
		return nil
	case MediaGallery:
		return component.disgoMediaGallery()
	case *MediaGallery:
		if component != nil {
			return component.disgoMediaGallery()
		}
		return nil
	case rawComponent:
		if layout, ok := component.disgoLayout().(discord.ContainerSubComponent); ok {
			return layout
		}
		return nil
	case *rawComponent:
		if component != nil {
			if layout, ok := component.disgoLayout().(discord.ContainerSubComponent); ok {
				return layout
			}
		}
		return nil
	default:
		return nil
	}
}

// disgoInteractive converts a component nested inside an ActionRow.
func disgoInteractive(c InteractiveComponent) discord.InteractiveComponent {
	switch component := c.(type) {
	case Button:
		return component.disgoButton()
	case *Button:
		if component != nil {
			return component.disgoButton()
		}
		return nil
	case SelectMenu:
		return component.disgoSelectMenu()
	case *SelectMenu:
		if component != nil {
			return component.disgoSelectMenu()
		}
		return nil
	default:
		return nil
	}
}

// disgoAccessory converts the component shown beside a Section's text.
func disgoAccessory(a SectionAccessory) discord.SectionAccessoryComponent {
	switch accessory := a.(type) {
	case Button:
		return accessory.disgoButton()
	case *Button:
		if accessory != nil {
			return accessory.disgoButton()
		}
		return nil
	case Thumbnail:
		return accessory.disgoThumbnail()
	case *Thumbnail:
		if accessory != nil {
			return accessory.disgoThumbnail()
		}
		return nil
	default:
		return nil
	}
}

func (t TextDisplay) disgoTextDisplay() discord.TextDisplayComponent {
	return discord.TextDisplayComponent{Content: t.Content}
}

func (c Container) disgoContainer() discord.ContainerComponent {
	// disgo types AccentColor as a plain int where 0 means no accent, so the
	// facade's zero value carries over without the pointer discordgo needed.
	container := discord.ContainerComponent{AccentColor: c.AccentColor}
	for _, sub := range c.Components {
		if converted := disgoContainerSub(sub); converted != nil {
			container.Components = append(container.Components, converted)
		}
	}
	return container
}

// disgoEmoji converts an emoji reference, returning nil when e is nil.
//
// disgo types a custom emoji's ID as a snowflake rather than a string. An ID
// that does not parse is left at zero, which is how a unicode emoji is
// identified — the same shape the facade already produces for one.
func (e *Emoji) disgoEmoji() *discord.ComponentEmoji {
	if e == nil {
		return nil
	}
	emoji := &discord.ComponentEmoji{Name: e.Name, Animated: e.Animated}
	if e.ID != "" {
		if id, err := snowflake.Parse(e.ID); err == nil {
			emoji.ID = id
		}
	}
	return emoji
}

func (s ButtonStyle) disgoStyle() discord.ButtonStyle {
	switch s {
	case ButtonPrimary:
		return discord.ButtonStylePrimary
	case ButtonSuccess:
		return discord.ButtonStyleSuccess
	case ButtonDanger:
		return discord.ButtonStyleDanger
	case ButtonLink:
		return discord.ButtonStyleLink
	default:
		return discord.ButtonStyleSecondary
	}
}

func (b Button) disgoButton() discord.ButtonComponent {
	return discord.ButtonComponent{
		Label:    b.Label,
		CustomID: b.CustomID,
		Style:    b.Style.disgoStyle(),
		URL:      b.URL,
		Disabled: b.Disabled,
		Emoji:    b.Emoji.disgoEmoji(),
	}
}

func (o SelectOption) disgoOption() discord.StringSelectMenuOption {
	return discord.StringSelectMenuOption{
		Label:       o.Label,
		Value:       o.Value,
		Description: o.Description,
		Default:     o.Default,
		Emoji:       o.Emoji.disgoEmoji(),
	}
}

func (m SelectMenu) disgoSelectMenu() discord.StringSelectMenuComponent {
	menu := discord.StringSelectMenuComponent{
		CustomID:    m.CustomID,
		Placeholder: m.Placeholder,
		MaxValues:   m.MaxValues,
		Disabled:    m.Disabled,
	}
	if m.MinValues != nil {
		minValues := *m.MinValues
		menu.MinValues = &minValues
	}
	for _, o := range m.Options {
		menu.Options = append(menu.Options, o.disgoOption())
	}
	return menu
}

func (r ActionRow) disgoActionRow() discord.ActionRowComponent {
	row := discord.ActionRowComponent{}
	for _, c := range r.Components {
		if converted := disgoInteractive(c); converted != nil {
			row.Components = append(row.Components, converted)
		}
	}
	return row
}

func (s SeparatorSpacing) disgoSpacing() discord.SeparatorSpacingSize {
	if s == SeparatorSpacingLarge {
		return discord.SeparatorSpacingSizeLarge
	}
	return discord.SeparatorSpacingSizeSmall
}

func (s Separator) disgoSeparator() discord.SeparatorComponent {
	divider := s.Divider
	return discord.SeparatorComponent{
		Divider: &divider,
		Spacing: s.Spacing.disgoSpacing(),
	}
}

func (t Thumbnail) disgoThumbnail() discord.ThumbnailComponent {
	// disgo types Description as a plain string, where discordgo used a
	// pointer to tell an empty description from an absent one. Both omit an
	// empty value on the wire, so the distinction never reached Discord.
	return discord.ThumbnailComponent{
		Media:       discord.UnfurledMediaItem{URL: t.URL},
		Description: t.Description,
		Spoiler:     t.Spoiler,
	}
}

func (s Section) disgoSection() discord.SectionComponent {
	section := discord.SectionComponent{}
	for _, c := range s.Components {
		section.Components = append(section.Components, c.disgoTextDisplay())
	}
	if s.Accessory != nil {
		section.Accessory = disgoAccessory(s.Accessory)
	}
	return section
}

func (m MediaItem) disgoItem() discord.MediaGalleryItem {
	return discord.MediaGalleryItem{
		Media:       discord.UnfurledMediaItem{URL: m.URL},
		Description: m.Description,
		Spoiler:     m.Spoiler,
	}
}

func (g MediaGallery) disgoMediaGallery() discord.MediaGalleryComponent {
	gallery := discord.MediaGalleryComponent{}
	for _, item := range g.Items {
		gallery.Items = append(gallery.Items, item.disgoItem())
	}
	return gallery
}

// disgoLayout returns a component read back off an existing message. It
// arrives already in disgo's types, so there is nothing to convert: the facade
// cannot inspect or rebuild it, only send it again unchanged.
func (r rawComponent) disgoLayout() discord.LayoutComponent {
	return r.component
}
