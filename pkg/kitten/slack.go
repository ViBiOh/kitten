package kitten

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/kitten/pkg/giphy"
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
	customSearch = regexp.MustCompile(`\|([0-9a-zA-Z_ -]+)$`)
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

	return a.getKittenBlock(ctx, kind, payload.Command, payload.Text, 0)
}

func (a App) getKittenBlock(ctx context.Context, kind memeKind, search, caption string, offset uint64) slack.Response {
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
		image, err := a.giphyApp.Search(ctx, search, offset)

		switch err {
		case nil:
			id = image.ID
		case giphy.ErrNotFound:
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

	return a.getSlackInteractResponse(kind, id, search, caption, offset)
}

func (a App) getSlackInteractResponse(kind memeKind, id, search, caption string, offset uint64) slack.Response {
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
				slack.NewButtonElement("Another?", nextValue, fmt.Sprintf("%s:%s:%d", kind, caption, offset+1), ""),
				slack.NewButtonElement("Send", sendValue, fmt.Sprintf("%s:%s:%s:0", kind, id, caption), "primary"),
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

			return a.getSlackUnsplashResponse(image, action.BlockID, caption, payload.User.ID)
		case gifKind:
			image, err := a.giphyApp.Get(ctx, id)
			if err != nil {
				return slack.NewError(err)
			}

			return a.getSlackGiphyResponse(image, action.BlockID, caption, payload.User.ID)
		default:
			return slack.NewEphemeralMessage("Sorry, we don't that kind of meme.")
		}
	}

	if action.ActionID == nextValue {
		kind, _, caption, offset := parseValue(action.Value)
		return a.getKittenBlock(ctx, kind, action.BlockID, caption, offset)
	}

	return slack.NewEphemeralMessage("We don't understand the action to perform.")
}

func (a App) getSlackUnsplashResponse(image unsplash.Image, search, caption, user string) slack.Response {
	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("Triggered By <@%s>", user))).AddElement(slack.NewText(fmt.Sprintf("Image By <%s|%s>", image.AuthorURL, image.Author))).AddElement(slack.NewText(fmt.Sprintf("Powered By <%s|Unsplash>", image.URL))),
			a.getMemeContent(image.ID, search, caption),
		},
	}
}

func (a App) getSlackGiphyResponse(image giphy.Gif, search, caption, user string) slack.Response {
	slackCtx := slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("Triggered By <@%s>", user)))
	if len(image.User.ProfileURL) > 0 {
		slackCtx = slackCtx.AddElement(slack.NewText(fmt.Sprintf("GIF By <%s|%s>", image.User.ProfileURL, image.User.Username)))
	}
	slackCtx = slackCtx.AddElement(slack.NewAccessory(fmt.Sprintf("%s/images/giphy_logo.png", a.website), "powered by giphy")).AddElement(slack.NewText("Powered By *GIPHY*"))

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
	return slack.NewImage(fmt.Sprintf("%s/api/%s", a.website, getContent(id, caption)), fmt.Sprintf("image with caption `%s` on it", caption), search)
}

func (a App) getGifContent(id, search, caption string) slack.Image {
	return slack.NewImage(fmt.Sprintf("%s/gif/%s", a.website, getContent(id, caption)), fmt.Sprintf("gif with caption `%s` on it", caption), search)
}

func getContent(id, caption string) string {
	return base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("id=%s&caption=%s", url.QueryEscape(id), url.QueryEscape(caption))))
}

func parseValue(value string) (memeKind, string, string, uint64) {
	parts := strings.SplitN(value, ":", 4)
	if len(parts) == 4 {
		return parseKind(parts[0]), parts[1], parts[2], 0
	}
	if len(parts) == 3 {
		offset, _ := strconv.ParseUint(parts[2], 10, 64)
		return parseKind(parts[0]), "", parts[1], offset
	}

	return "", "", "", 0
}
