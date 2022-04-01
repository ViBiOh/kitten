package discord

type interactionType uint

const (
	pingInteraction interactionType = 1
	// ApplicationCommandInteraction occurs when user enter a slash command
	ApplicationCommandInteraction interactionType = 2
	// MessageComponentInteraction occurs when user interact with an action
	MessageComponentInteraction interactionType = 3
)

type callbackType uint

const (
	pongCallback callbackType = 1
	// ChannelMessageWithSourceCallback answer to user
	ChannelMessageWithSourceCallback callbackType = 4
	// UpdateMessageCallback in place
	UpdateMessageCallback callbackType = 7
)

type componentType uint

const (
	// ActionRowType for row
	ActionRowType componentType = 1
	buttonType    componentType = 2
)

type buttonStyle uint

const (
	// PrimaryButton is green
	PrimaryButton buttonStyle = 1
	// SecondaryButton is grey
	SecondaryButton buttonStyle = 2
	// DangerButton is red
	DangerButton buttonStyle = 4
)

const (
	// EphemeralMessage int value
	EphemeralMessage int = 1 << 6
)

// InteractionRequest when user perform an action
type InteractionRequest struct {
	ID            string `json:"id"`
	GuildID       string `json:"guild_id"`
	Member        Member `json:"member"`
	Token         string `json:"token"`
	ApplicationID string `json:"application_id"`
	Data          struct {
		Name     string          `json:"name"`
		CustomID string          `json:"custom_id"`
		Options  []CommandOption `json:"options"`
	} `json:"data"`
	Message struct {
		Interaction struct {
			Name string `json:"name"`
		} `json:"interaction"`
	} `json:"message"`
	Type interactionType `json:"type"`
}

// Member of discord
type Member struct {
	User struct {
		ID       string `json:"id,omitempty"`
		Username string `json:"username,omitempty"`
	} `json:"user,omitempty"`
}

// InteractionResponse for responding to user
type InteractionResponse struct {
	Data struct {
		Content         string         `json:"content,omitempty"`
		AllowedMentions AllowedMention `json:"allowed_mentions"`
		Embeds          []Embed        `json:"embeds"`
		Components      []Component    `json:"components"`
		Flags           int            `json:"flags"`
	} `json:"data,omitempty"`
	Type callbackType `json:"type,omitempty"`
}

// NewEphemeral creates an ephemeral response
func NewEphemeral(replace bool, content string) InteractionResponse {
	callback := ChannelMessageWithSourceCallback
	if replace {
		callback = UpdateMessageCallback
	}

	instance := InteractionResponse{Type: callback}
	instance.Data.Content = content
	instance.Data.Flags = EphemeralMessage
	instance.Data.Embeds = []Embed{}
	instance.Data.Components = []Component{}

	return instance
}

// AllowedMention list
type AllowedMention struct {
	Parse []string `json:"parse"`
}

// Image content
type Image struct {
	URL string `json:"url,omitempty"`
}

// Author content
type Author struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Embed of content
type Embed struct {
	Thumbnail   *Embed  `json:"thumbnail,omitempty"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	URL         string  `json:"url,omitempty"`
	Fields      []Field `json:"fields,omitempty"`
	Image       Image   `json:"image,omitempty"`
	Provider    Author  `json:"provider,omitempty"`
	Author      Author  `json:"author,omitempty"`
	Color       int     `json:"color,omitempty"`
}

// SetColor define color of embed
func (e Embed) SetColor(color int) Embed {
	e.Color = color
	return e
}

// Field for embed
type Field struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

// NewField creates new field
func NewField(name, value string) Field {
	return Field{
		Name:   name,
		Value:  value,
		Inline: true,
	}
}

// Component describes an interactive component
type Component struct {
	Label      string        `json:"label,omitempty"`
	CustomID   string        `json:"custom_id,omitempty"`
	Components []Component   `json:"components,omitempty"`
	Type       componentType `json:"type,omitempty"`
	Style      buttonStyle   `json:"style,omitempty"`
}

// NewButton creates a new button
func NewButton(style buttonStyle, label, customID string) Component {
	return Component{
		Type:     buttonType,
		Style:    style,
		Label:    label,
		CustomID: customID,
	}
}

// Command configuration
type Command struct {
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Options     []CommandOption `json:"options,omitempty"`
}

// CommandOption configuration option
type CommandOption struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
	Type        int    `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
}
