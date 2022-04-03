package kitten

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
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
func (a App) SlackCommand(ctx context.Context, w http.ResponseWriter, user, search, caption string) {
	if len(caption) == 0 {
		httpjson.Write(w, http.StatusOK, slack.NewEphemeralMessage("You must provide a caption"))
		return
	}

	httpjson.Write(w, http.StatusOK, a.getKittenBlock(ctx, user, search, caption))
}

func (a App) getKittenBlock(ctx context.Context, user, search, caption string) slack.Response {
	if search == "custom" {
		matches := customSearch.FindStringSubmatch(caption)
		if len(matches) == 0 {
			return slack.NewEphemeralMessage("You must provide a query for image in the form `my caption value #horse`")
		}

		search = matches[1]
		caption = strings.TrimSpace(strings.TrimSuffix(caption, matches[0]))
	}

	if a.isOverride(search) {
		return a.getOverrideResponse(search, caption, user)
	}

	image, err := a.unsplashApp.GetRandomImage(ctx, search)
	if err != nil {
		return slack.NewEphemeralMessage(fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
	}

	return a.getUnsplashResponse(search, image, caption, "")
}

// SlackInteract handler
func (a App) SlackInteract(ctx context.Context, user string, actions []slack.InteractiveAction) slack.Response {
	action := actions[0]
	if action.ActionID == cancelValue {
		return slack.NewEphemeralMessage("Ok, not now.")
	}

	if action.ActionID == sendValue {
		id, caption := parseBlockID(action.Value)
		image, err := a.unsplashApp.GetImage(ctx, id)
		if err != nil {
			return slack.NewEphemeralMessage(fmt.Sprintf("Unable to find asked image: %s", err))
		}

		return a.getUnsplashResponse(action.BlockID, image, caption, user)
	}

	if action.ActionID == nextValue {
		return a.getKittenBlock(ctx, "", action.BlockID, action.Value)
	}

	return slack.NewEphemeralMessage("We don't understand the action to perform.")
}

func (a App) getUnsplashResponse(search string, image unsplash.Image, caption, user string) slack.Response {
	content := slack.NewAccessory(fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(image.ID), url.QueryEscape(caption)), fmt.Sprintf("image with caption `%s` on it", caption))

	if len(user) == 0 {
		return slack.Response{
			ResponseType:    "ephemeral",
			ReplaceOriginal: true,
			Blocks: []slack.Block{
				content,
				slack.NewActions(search, cancelButton, slack.NewButtonElement("Another?", nextValue, caption, ""),
					slack.NewButtonElement("Send", sendValue, fmt.Sprintf("%s:%s", image.ID, caption), "primary")),
			},
		}
	}

	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			slack.NewSection(slack.NewText(fmt.Sprintf("<@%s> shares an image of <%s|%s> from <%s|Unsplash>", user, image.AuthorURL, image.Author, image.URL)), nil),
			content,
		},
	}
}

func (a App) getOverrideResponse(id, caption, user string) slack.Response {
	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			slack.NewSection(slack.NewText(fmt.Sprintf("<@%s> shares a meme", user)), nil),
			slack.NewAccessory(fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(id), url.QueryEscape(caption)), fmt.Sprintf("image with caption `%s` on it", caption)),
		},
	}
}

func parseBlockID(value string) (string, string) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}

	return parts[0], ""
}
