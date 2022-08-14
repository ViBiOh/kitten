package kitten

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/kitten/pkg/tenor"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

type memeKind string

const (
	unkownKind memeKind = "unknown"
	imageKind  memeKind = "image"
	gifKind    memeKind = "gif"
)

func parseKind(value string) memeKind {
	switch value {
	case "image":
		return imageKind
	case "gif":
		return gifKind
	default:
		return unkownKind
	}
}

const (
	customImageCommand = "meme"
	customGifSearch    = "memegif"

	cancelValue = "cancel"
	nextValue   = "next"
	sendValue   = "send"
)

var (
	customSearch = regexp.MustCompile(`\|([0-9a-zA-Z -]+)$`)
	cancelButton = slack.NewButtonElement("Cancel", cancelValue, "", "danger")
)

// SlackCommand handler
func (a App) SlackCommand(ctx context.Context, payload slack.SlashPayload) slack.Response {
	if len(payload.Text) == 0 {
		return slack.NewEphemeralMessage("You must provide a caption")
	}

	var kind memeKind
	switch payload.Command {
	case customGifSearch:
		kind = gifKind
	default:
		kind = imageKind
	}

	return a.getKittenBlock(ctx, kind, payload.Command, payload.Text, "")
}

func (a App) getKittenBlock(ctx context.Context, kind memeKind, search, caption string, next string) slack.Response {
	if search == customImageCommand || search == customGifSearch {
		matches := customSearch.FindStringSubmatch(caption)
		if len(matches) == 0 {
			return slack.NewEphemeralMessage("You must provide a query for image in the form `my caption value |searched_query`")
		}

		search = matches[1]
		caption = strings.TrimSpace(strings.TrimSuffix(caption, matches[0]))
	}

	var id string

	switch kind {
	case gifKind:
		image, nextValue, err := a.tenorApp.Search(ctx, search, next)

		switch err {
		case nil:
			id = image.ID
			next = nextValue
		case tenor.ErrNotFound:
			return slack.NewEphemeralMessage("No gif found")
		default:
			return slack.NewEphemeralMessage(fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
		}

	default:
		image, err := a.unsplashApp.Search(ctx, search)
		if err != nil {
			return slack.NewEphemeralMessage(fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
		}
		id = image.ID
	}

	return a.getSlackInteractResponse(kind, id, search, caption, next)
}

func (a App) getSlackInteractResponse(kind memeKind, id, search, caption string, next string) slack.Response {
	var accessory slack.Image
	switch kind {
	case gifKind:
		accessory = a.getGifContent(id, search, caption)
	default:
		accessory = a.getMemeContent(id, search, caption)
	}

	return slack.Response{
		ResponseType:    "ephemeral",
		ReplaceOriginal: true,
		Blocks: []slack.Block{
			accessory,
			slack.NewActions(search,
				cancelButton,
				slack.NewButtonElement("Another?", nextValue, fmt.Sprintf("%s:%s:%s", kind, caption, next), ""),
				slack.NewButtonElement("Send", sendValue, fmt.Sprintf("%s:%s:%s: ", kind, id, caption), "primary"),
			),
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
		kind, id, caption, _ := parseValue(action.Value)

		switch kind {
		case imageKind:
			image, err := a.unsplashApp.Get(ctx, id)
			if err != nil {
				return slack.NewError(err)
			}

			return a.getSlackImageResponse(image, action.BlockID, caption, payload.User.ID)
		case gifKind:
			image, err := a.tenorApp.Get(ctx, id)
			if err != nil {
				return slack.NewError(err)
			}

			return a.getSlackGifReponse(image, action.BlockID, caption, payload.User.ID)
		default:
			return slack.NewEphemeralMessage("Sorry, we don't that kind of meme.")
		}
	}

	if action.ActionID == nextValue {
		kind, _, caption, next := parseValue(action.Value)
		return a.getKittenBlock(ctx, kind, action.BlockID, caption, next)
	}

	return slack.NewEphemeralMessage("We don't understand the action to perform.")
}

func (a App) getSlackImageResponse(image unsplash.Image, search, caption, user string) slack.Response {
	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("Triggered By <@%s>", user))).AddElement(slack.NewText(fmt.Sprintf("Image By <%s|%s>", image.AuthorURL, image.Author))).AddElement(slack.NewText(fmt.Sprintf("Powered By <%s|Unsplash>", image.URL))),
			a.getMemeContent(image.ID, search, caption),
		},
	}
}

func (a App) getSlackGifReponse(image tenor.ResponseObject, search, caption, user string) slack.Response {
	slackCtx := slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("Triggered By <@%s>", user)))
	slackCtx = slackCtx.AddElement(slack.NewText("Powered By *tenor*"))

	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			slackCtx,
			a.getGifContent(image.ID, search, caption),
		},
	}
}

func (a App) getMemeContent(id, search, caption string) slack.Image {
	return slack.NewImage(fmt.Sprintf("%s/api/%s", a.website, getContent(id, search, caption)), fmt.Sprintf("image with caption `%s` on it", caption), search)
}

func (a App) getGifContent(id, search, caption string) slack.Image {
	return slack.NewImage(fmt.Sprintf("%s/gif/%s", a.website, getContent(id, search, caption)), fmt.Sprintf("gif with caption `%s` on it", caption), search)
}

func getContent(id, search, caption string) string {
	return base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("id=%s&search=%s&caption=%s", url.QueryEscape(id), url.QueryEscape(search), url.QueryEscape(caption))))
}

func parseValue(value string) (memeKind, string, string, string) {
	parts := strings.SplitN(value, ":", 4)
	if len(parts) == 4 {
		return parseKind(parts[0]), parts[1], parts[2], ""
	}
	if len(parts) == 3 {
		return parseKind(parts[0]), "", parts[1], parts[2]
	}

	return "", "", "", ""
}
