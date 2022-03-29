package meme

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kitten/pkg/slack"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

const (
	cancelValue = "cancel"
	nextValue   = "next"
	sendValue   = "send"
)

var cancelButton = slack.NewButtonElement("Annuler", cancelValue, "", "danger")

// SlackCommand handler
func (a App) SlackCommand(ctx context.Context, w http.ResponseWriter, search, caption string) {
	response := a.getKittenBlock(ctx, search, caption)

	if payload, err := json.Marshal(response); err == nil {
		logger.Info("%s", payload)
	}

	httpjson.Write(w, http.StatusOK, response)
}

func (a App) getKittenBlock(ctx context.Context, search, caption string) slack.Response {
	image, err := a.unsplashApp.GetRandomImage(ctx, search)
	if err != nil {
		return slack.NewEphemeralMessage(fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
	}

	return a.getKittenResponse(search, image, caption, "")
}

// SlackInteract handler
func (a App) SlackInteract(ctx context.Context, user string, actions []slack.InteractiveAction) slack.Response {
	action := actions[0]
	if action.ActionID == cancelValue {
		return slack.NewEphemeralMessage("Ok, not now.")
	}

	id, caption := parseBlockID(action.Value)

	if action.ActionID == sendValue {
		image, err := a.unsplashApp.GetImage(ctx, id)
		if err != nil {
			return slack.NewEphemeralMessage(fmt.Sprintf("Unable to find asked image: %s", err))
		}

		return a.getKittenResponse(action.BlockID, image, caption, user)
	}

	if action.ActionID == nextValue {
		return a.getKittenBlock(ctx, action.BlockID, caption)
	}

	return slack.NewEphemeralMessage("On ne comprend pas l'action à effectuer")
}

func (a App) getKittenResponse(search string, image unsplash.Image, caption, user string) slack.Response {
	content := slack.NewAccessory(fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(image.ID), url.QueryEscape(caption)), fmt.Sprintf("image with caption `%s` on git", caption))

	if len(user) == 0 {
		return slack.Response{
			ResponseType:    "ephemeral",
			ReplaceOriginal: true,
			Blocks: []slack.Block{
				content,
				slack.NewActions(search, cancelButton, slack.NewButtonElement("Another?", nextValue, fmt.Sprintf("%s@%s", search, caption), ""),
					slack.NewButtonElement("Send", sendValue, fmt.Sprintf("%s@%s", image.ID, caption), "primary")),
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

func parseBlockID(value string) (string, string) {
	parts := strings.SplitN(value, "@", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}

	return parts[0], ""
}
