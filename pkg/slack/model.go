package slack

import "fmt"

// EmptySection for not found
var EmptySection = Section{}

// Block response for slack
type Block any

// Element response for slack
type Element any

// Text Slack's model
type Text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewText creates Text
func NewText(text string) Text {
	return Text{
		Type: "mrkdwn",
		Text: text,
	}
}

// NewPlainText creates PlainText
func NewPlainText(text string) Text {
	return Text{
		Type: "plain_text",
		Text: text,
	}
}

// Accessory Slack's model
type Accessory struct {
	Type  string `json:"type"`
	Image string `json:"image_url"`
	Alt   string `json:"alt_text"`
}

// NewAccessory creates Accessory
func NewAccessory(image, alt string) *Accessory {
	return &Accessory{
		Type:  "image",
		Image: image,
		Alt:   alt,
	}
}

// ButtonElement response for slack
type ButtonElement struct {
	Type     string `json:"type"`
	Text     Text   `json:"text"`
	ActionID string `json:"action_id"`
	Value    string `json:"value,omitempty"`
	Style    string `json:"style,omitempty"`
}

// NewButtonElement creates ButtonElement
func NewButtonElement(text string, actionID, value, style string) Element {
	return ButtonElement{
		Type:     "button",
		Text:     NewPlainText(text),
		ActionID: actionID,
		Value:    value,
		Style:    style,
	}
}

// Actions response for slack
type Actions struct {
	Type     string    `json:"type"`
	BlockID  string    `json:"block_id,omitempty"`
	Elements []Element `json:"elements"`
}

// NewActions creates Actions
func NewActions(blockID string, elements ...Element) Block {
	return Actions{
		Type:     "actions",
		Elements: elements,
		BlockID:  blockID,
	}
}

// Section response for slack
type Section struct {
	Accessory *Accessory `json:"accessory,omitempty"`
	Type      string     `json:"type"`
	Text      Text       `json:"text"`
}

// NewSection creates Section
func NewSection(text Text, accessory *Accessory) Block {
	return Section{
		Type:      "section",
		Text:      text,
		Accessory: accessory,
	}
}

// Response response content
type Response struct {
	ResponseType    string  `json:"response_type,omitempty"`
	Text            string  `json:"text,omitempty"`
	Blocks          []Block `json:"blocks,omitempty"`
	ReplaceOriginal bool    `json:"replace_original,omitempty"`
	DeleteOriginal  bool    `json:"delete_original,omitempty"`
}

// SlashPayload receives by a slash command
type SlashPayload struct {
	ChannelID   string `json:"channel_id"`
	Command     string `json:"command"`
	ResponseURL string `json:"response_url"`
	Text        string `json:"text"`
	Token       string `json:"token"`
	UserID      string `json:"user_id"`
}

// InteractiveAction response from slack
type InteractiveAction struct {
	Type     string `json:"type"`
	BlockID  string `json:"block_id,omitempty"`
	ActionID string `json:"action_id,omitempty"`
	Value    string `json:"value,omitempty"`
}

// InteractivePayload response from slack
type InteractivePayload struct {
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	Container struct {
		ChannelID string `json:"channel_id"`
	} `json:"container"`
	Type        string              `json:"type"`
	ResponseURL string              `json:"response_url"`
	Actions     []InteractiveAction `json:"actions"`
}

// NewError creates ephemeral error response
func NewError(err error) Response {
	return NewEphemeralMessage(fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
}

// NewEphemeralMessage creates ephemeral text response
func NewEphemeralMessage(message string) Response {
	return Response{
		ResponseType:    "ephemeral",
		Text:            message,
		ReplaceOriginal: true,
	}
}
