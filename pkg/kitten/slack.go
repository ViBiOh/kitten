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

	yoloMagicWord = "yolo"
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
	customSearch = regexp.MustCompile(`\|(.+)$`)
	cancelButton = slack.NewButtonElement("Cancel", cancelValue, "", "danger")
)

// SlackCommand handler
func (s Service) SlackCommand(ctx context.Context, payload slack.SlashPayload) slack.Response {
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

	return s.getKittenBlock(ctx, kind, payload.UserID, payload.Command, payload.Text, "")
}

func (s Service) getKittenBlock(ctx context.Context, kind memeKind, user, search, caption, next string) slack.Response {
	var yolo bool

	matches := customSearch.FindStringSubmatch(caption)
	if len(matches) != 0 {
		initialSearch := search

		var err error
		search, err = sanitizeValue(matches[1])
		if err != nil {
			return slack.NewError(fmt.Errorf("sanitize value `%s`: %w", matches[1], err))
		}

		if search == yoloMagicWord {
			search = initialSearch
			yolo = true
		}

		if yolo || initialSearch == customImageCommand || initialSearch == customGifSearch {
			caption = strings.TrimSpace(strings.TrimSuffix(caption, matches[0]))
		}
	} else if search == customImageCommand || search == customGifSearch {
		return slack.NewEphemeralMessage("You must provide a query for image in the form `my caption value |searched_query`")
	}

	var id string

	switch kind {
	case gifKind:
		image, nextValue, err := s.tenorService.Search(ctx, search, next)

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
		image, err := s.unsplashService.Search(ctx, search)
		if err != nil {
			return slack.NewEphemeralMessage(fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
		}
		id = image.ID
	}

	return s.getSlackInteractResponse(kind, user, id, search, caption, next, yolo)
}

func (s Service) getSlackInteractResponse(kind memeKind, user, id, search, caption, next string, yolo bool) slack.Response {
	var accessory slack.Image
	switch kind {
	case gifKind:
		accessory = s.getGifContent(id, search, caption)
	default:
		accessory = s.getMemeContent(id, search, caption)
	}

	if yolo {
		return slack.Response{
			ResponseType: "in_channel",
			Blocks: []slack.Block{
				getSlackHeadline(user),
				accessory,
			},
		}
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
func (s Service) SlackInteract(ctx context.Context, payload slack.InteractivePayload) slack.Response {
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
			image, err := s.unsplashService.Get(ctx, id)
			if err != nil {
				return slack.NewError(err)
			}

			return s.getSlackImageResponse(image, action.BlockID, caption, payload.User.ID)
		case gifKind:
			image, err := s.tenorService.Get(ctx, id)
			if err != nil {
				return slack.NewError(err)
			}

			return s.getSlackGifReponse(image, action.BlockID, caption, payload.User.ID)
		default:
			return slack.NewEphemeralMessage("Sorry, we don't that kind of meme.")
		}
	}

	if action.ActionID == nextValue {
		kind, _, caption, next := parseValue(action.Value)
		return s.getKittenBlock(ctx, kind, payload.User.ID, action.BlockID, caption, next)
	}

	return slack.NewEphemeralMessage("We don't understand the action to perform.")
}

func (s Service) getSlackImageResponse(image unsplash.Image, search, caption, user string) slack.Response {
	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("Triggered By <@%s>", user))).AddElement(slack.NewText(fmt.Sprintf("Image By <%s|%s>", image.AuthorURL, image.Author))).AddElement(slack.NewText(fmt.Sprintf("Powered By <%s|Unsplash>", image.URL))),
			s.getMemeContent(image.ID, search, caption),
		},
	}
}

func (s Service) getSlackGifReponse(image tenor.ResponseObject, search, caption, user string) slack.Response {
	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			getSlackHeadline(user),
			s.getGifContent(image.ID, search, caption),
		},
	}
}

func getSlackHeadline(user string) slack.Context {
	slackCtx := slack.NewContext().AddElement(slack.NewText(fmt.Sprintf("Triggered By <@%s>", user)))
	slackCtx = slackCtx.AddElement(slack.NewText("Powered By *tenor*"))

	return slackCtx
}

func (s Service) getMemeContent(id, search, caption string) slack.Image {
	return slack.NewImage(fmt.Sprintf("%s/api/%s", s.website, getContent(id, search, caption)), fmt.Sprintf("image with caption `%s` on it", caption), search)
}

func (s Service) getGifContent(id, search, caption string) slack.Image {
	return slack.NewImage(fmt.Sprintf("%s/gif/%s", s.website, getContent(id, search, caption)), fmt.Sprintf("gif with caption `%s` on it", caption), search)
}

func getContent(id, search, caption string) string {
	return base64.URLEncoding.EncodeToString(fmt.Appendf(nil, "id=%s&search=%s&caption=%s", url.QueryEscape(id), url.QueryEscape(search), url.QueryEscape(caption)))
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
