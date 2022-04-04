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
	if search == "meme" {
		matches := customSearch.FindStringSubmatch(caption)
		if len(matches) == 0 {
			return slack.NewEphemeralMessage("You must provide a query for image in the form `my caption value #horse`")
		}

		search = matches[1]
		caption = strings.TrimSpace(strings.TrimSuffix(caption, matches[0]))
	}

	var image unsplash.Image
	var err error

	if !a.isOverride(search) {
		image, err = a.unsplashApp.GetRandomImage(ctx, search)
		if err != nil {
			return slack.NewEphemeralMessage(fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
		}
	}

	return a.getSlackResponse(image, search, caption, "")
}

// SlackInteract handler
func (a App) SlackInteract(ctx context.Context, user string, actions []slack.InteractiveAction) slack.Response {
	action := actions[0]
	if action.ActionID == cancelValue {
		return slack.NewEphemeralMessage("Ok, not now.")
	}

	if action.ActionID == sendValue {
		var image unsplash.Image
		id, caption := parseBlockID(action.Value)

		if !a.isOverride(id) {
			var err error

			image, err = a.unsplashApp.GetImage(ctx, id)
			if err != nil {
				return slack.NewEphemeralMessage(fmt.Sprintf("unable to find asked image: %s", err))
			}
		}

		return a.getSlackResponse(image, action.BlockID, caption, user)
	}

	if action.ActionID == nextValue {
		return a.getKittenBlock(ctx, "", action.BlockID, action.Value)
	}

	return slack.NewEphemeralMessage("We don't understand the action to perform.")
}

func (a App) getSlackResponse(image unsplash.Image, search, caption, user string) slack.Response {
	if len(user) == 0 {
		elements := []slack.Element{cancelButton}

		imageID := search
		if !a.isOverride(search) {
			elements = append(elements, slack.NewButtonElement("Another?", nextValue, caption, ""))
			imageID = image.ID
		}

		elements = append(elements, slack.NewButtonElement("Send", sendValue, fmt.Sprintf("%s:%s", imageID, caption), "primary"))

		return slack.Response{
			ResponseType:    "ephemeral",
			ReplaceOriginal: true,
			Blocks: []slack.Block{
				a.getMemeContent(image, search, caption),
				slack.NewActions(search, elements...),
			},
		}
	}

	return slack.Response{
		ResponseType:   "in_channel",
		DeleteOriginal: true,
		Blocks: []slack.Block{
			a.getSlackTitle(image, user, search),
			a.getMemeContent(image, search, caption),
		},
	}
}

func (a App) getSlackTitle(image unsplash.Image, user, search string) slack.Block {
	if a.isOverride(search) {
		return slack.NewSection(slack.NewText(fmt.Sprintf("<@%s> shares a meme", user)), nil)
	}

	return slack.NewSection(slack.NewText(fmt.Sprintf("<@%s> shares an image of <%s|%s> from <%s|Unsplash>", user, image.AuthorURL, image.Author, image.URL)), nil)
}

func (a App) getMemeContent(image unsplash.Image, search, caption string) *slack.Accessory {
	var imageID string

	if !a.isOverride(search) {
		imageID = image.ID
	} else {
		imageID = search
	}

	return slack.NewAccessory(fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(imageID), url.QueryEscape(caption)), fmt.Sprintf("image with caption `%s` on it", caption))
}

func parseBlockID(value string) (string, string) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}

	return parts[0], ""
}
