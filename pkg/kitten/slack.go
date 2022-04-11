package kitten

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ViBiOh/kitten/pkg/slack"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

const (
	cancelValue = "cancel"
	nextValue   = "next"
	sendValue   = "send"
)

var (
	customSearch = regexp.MustCompile("#([a-zA-Z_ ]+)$")
	cancelButton = slack.NewButtonElement("Cancel", cancelValue, "", "danger")
)

// SlackCommand handler
func (a App) SlackCommand(ctx context.Context, payload slack.SlashPayload) slack.Response {
	if len(payload.Text) == 0 {
		return slack.NewEphemeralMessage("You must provide a caption")
	}

	return a.getKittenBlock(ctx, payload.Command, payload.Text)
}

func (a App) getKittenBlock(ctx context.Context, search, caption string) slack.Response {
	if search == "meme" {
		matches := customSearch.FindStringSubmatch(caption)
		if len(matches) == 0 {
			return slack.NewEphemeralMessage("You must provide a query for image in the form `my caption value #searched_query`")
		}

		search = matches[1]
		caption = strings.TrimSpace(strings.TrimSuffix(caption, matches[0]))
	}

	var id string

	if a.isOverride(search) {
		id = search
	} else if image, err := a.unsplashApp.GetRandomImage(ctx, search); err != nil {
		return slack.NewError(err)
	} else {
		id = image.ID
	}

	return a.getSlackInteractResponse(id, search, caption)
}

func (a App) getSlackInteractResponse(id, search, caption string) slack.Response {
	elements := []slack.Element{cancelButton}

	if !a.isOverride(search) {
		elements = append(elements, slack.NewButtonElement("Another?", nextValue, caption, ""))
	}

	elements = append(elements, slack.NewButtonElement("Send", sendValue, fmt.Sprintf("%s:%s", id, caption), "primary"))

	return slack.Response{
		ResponseType:    "ephemeral",
		ReplaceOriginal: true,
		Blocks: []slack.Block{
			a.getMemeContent(id, caption),
			slack.NewActions(search, elements...),
		},
	}
}

// SlackInteract handler
func (a App) SlackInteract(ctx context.Context, payload slack.InteractivePayload) slack.Response {
	if len(payload.Actions) == 0 {
		return slack.NewEphemeralMessage("No action provided")
	}

	action := payload.Actions[0]
	if action.ActionID == cancelValue {
		return slack.NewEphemeralMessage("Ok, not now.")
	}

	if action.ActionID == sendValue {
		id, caption := parseBlockID(action.Value)

		if a.isOverride(id) {
			return a.getSlackOverrideResponse(ctx, id, caption, payload.User.ID, payload.Container.ChannelID)
		}

		image, err := a.unsplashApp.GetImage(ctx, id)
		if err != nil {
			return slack.NewError(err)
		}

		return a.getSlackUnsplashResponse(ctx, image, caption, payload.User.ID, payload.Container.ChannelID)
	}

	if action.ActionID == nextValue {
		return a.getKittenBlock(ctx, action.BlockID, action.Value)
	}

	return slack.NewEphemeralMessage("We don't understand the action to perform.")
}

func (a App) getSlackUnsplashResponse(ctx context.Context, image unsplash.Image, caption, user, channelID string) slack.Response {
	imagePath, _, err := a.generateAndStoreImage(ctx, image.ID, image.Raw, caption)
	if err != nil {
		return slack.NewError(fmt.Errorf("unable to generate image: %s", err))
	}

	return slack.Response{
		DeleteOriginal: true,
		File: &slack.File{
			Channels:       []string{channelID},
			Filename:       "image.jpeg",
			Filepath:       imagePath,
			InitialComment: fmt.Sprintf("<@%s> shares an image of <%s|%s> from <%s|Unsplash>", user, image.AuthorURL, image.Author, image.URL),
		},
	}
}

func (a App) getSlackOverrideResponse(ctx context.Context, id, caption, user, channelID string) slack.Response {
	imagePath, _, err := a.generateAndStoreImage(ctx, id, a.getOverride(id), caption)
	if err != nil {
		return slack.NewError(fmt.Errorf("unable to generate image: %s", err))
	}

	return slack.Response{
		DeleteOriginal: true,
		File: &slack.File{
			Channels:       []string{channelID},
			Filename:       "image.jpeg",
			Filepath:       imagePath,
			InitialComment: fmt.Sprintf("<@%s> shares a meme", user),
		},
	}
}

func (a App) getMemeContent(id, caption string) *slack.Accessory {
	return slack.NewAccessory(fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(id), url.QueryEscape(caption)), fmt.Sprintf("image with caption `%s` on it", caption))
}

func parseBlockID(value string) (string, string) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return parts[0], ""
}
