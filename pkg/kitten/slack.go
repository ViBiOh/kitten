package kitten

import (
	"context"
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
	customSearch = regexp.MustCompile("#([0-9a-zA-Z_ ]+)$")
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
			return slack.NewEphemeralMessage("You must provide a query for image in the form `my caption value #searched_query`")
		}

		search = matches[1]
		caption = strings.TrimSpace(strings.TrimSuffix(caption, matches[0]))
	}

	var id string

	if a.isOverride(search) {
		id = search
	} else {
		switch kind {
		case gifKind:
			image, err := a.giphyApp.Search(ctx, search, offset)
			if err != nil {
				return slack.NewEphemeralMessage(fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
			}
			id = image.ID
		default:
			image, err := a.unsplashApp.Search(ctx, search)
			if err != nil {
				return slack.NewEphemeralMessage(fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
			}
			id = image.ID
		}
	}

	return a.getSlackInteractResponse(kind, id, search, caption, offset)
}

func (a App) getSlackInteractResponse(kind memeKind, id, search, caption string, offset uint64) slack.Response {
	elements := []slack.Element{cancelButton}

	if !a.isOverride(search) {
		elements = append(elements, slack.NewButtonElement("Another?", nextValue, fmt.Sprintf("%s:%s:%d", kind, caption, offset+1), ""))
	}

	elements = append(elements, slack.NewButtonElement("Send", sendValue, fmt.Sprintf("%s:%s:%s:0", kind, id, caption), "primary"))

	var accessory *slack.Accessory
	switch kind {
	case gifKind:
		accessory = a.getGifContent(id, caption)
	default:
		accessory = a.getMemeContent(id, caption)
	}

	return slack.Response{
		ResponseType:    "ephemeral",
		ReplaceOriginal: true,
		Blocks: []slack.Block{
			accessory,
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
		kind, id, caption, _ := parseValue(action.Value)
		if a.isOverride(id) {
			return a.getSlackOverrideResponse(id, caption, payload.User.ID)
		}

		switch kind {
		case imageKind:
			image, err := a.unsplashApp.Get(ctx, id)
			if err != nil {
				return slack.NewError(err)
			}

			return a.getSlackUnsplashResponse(image, caption, payload.User.ID)
		case gifKind:
			image, err := a.giphyApp.Get(ctx, id)
			if err != nil {
				return slack.NewError(err)
			}

			return a.getSlackGiphyResponse(image, caption, payload.User.ID)
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

func (a App) getSlackUnsplashResponse(image unsplash.Image, caption, user string) slack.Response {
	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			a.getMemeContent(image.ID, caption),
			slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("Powered By <%s|Unsplash>", image.URL))).AddElement(slack.NewText(fmt.Sprintf("Triggered By <@%s>", user))).AddElement(slack.NewText(fmt.Sprintf("Image by <%s|%s>", image.AuthorURL, image.Author))),
		},
	}
}

func (a App) getSlackGiphyResponse(image giphy.Gif, caption, user string) slack.Response {
	slackCtx := slack.NewContext().AddElement(slack.NewAccessory(fmt.Sprintf("%s/images/giphy_logo.png", a.website), "powered by giphy")).AddElement(slack.NewText("Powered By *GIPHY*")).AddElement(slack.NewText(fmt.Sprintf("Triggered By <@%s>", user)))

	if len(image.User.ProfileURL) > 0 {
		slackCtx = slackCtx.AddElement(slack.NewText(fmt.Sprintf("GIF by <%s|%s>", image.User.ProfileURL, image.User.Username)))
	}

	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			a.getGifContent(image.ID, caption),
			slackCtx,
		},
	}
}

func (a App) getSlackOverrideResponse(id, caption, user string) slack.Response {
	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			a.getMemeContent(id, caption),
			slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("Powered By <%s|Kitten>", a.website))).AddElement(slack.NewText(fmt.Sprintf("Triggered By <@%s>", user))),
		},
	}
}

func (a App) getMemeContent(id, caption string) *slack.Accessory {
	return slack.NewAccessory(fmt.Sprintf("%s/api?id=%s&caption=%s", a.website, url.QueryEscape(id), url.QueryEscape(caption)), fmt.Sprintf("image with caption `%s` on it", caption))
}

func (a App) getGifContent(id, caption string) *slack.Accessory {
	return slack.NewAccessory(fmt.Sprintf("%s/gif?id=%s&caption=%s", a.website, url.QueryEscape(id), url.QueryEscape(caption)), fmt.Sprintf("gif with caption `%s` on it", caption))
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
