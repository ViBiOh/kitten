package kitten

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/kitten/pkg/discord"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

const (
	captionParam     = "caption"
	searchParam      = "search"
	contentSeparator = ":"

	memeName = "meme"
)

// Commands configuration
var Commands = map[string]discord.Command{
	memeName: {
		Name:        memeName,
		Description: "Generate a meme with caption from Unsplash",
		Options: []discord.CommandOption{
			{
				Name:        searchParam,
				Description: "Searched image",
				Type:        3, // https://discord.com/developers/docs/interactions/slash-commands#applicationcommandoptiontype
				Required:    true,
			},
			{
				Name:        captionParam,
				Description: "Caption to add",
				Type:        3, // https://discord.com/developers/docs/interactions/slash-commands#applicationcommandoptiontype
				Required:    true,
			},
		},
	},
}

// DiscordHandler handle discord request
func (a App) DiscordHandler(r *http.Request, webhook discord.InteractionRequest) (discord.InteractionResponse, func() discord.InteractionResponse) {
	replace, id, search, caption, err := a.parseQuery(webhook)
	if err != nil {
		return discord.NewEphemeral(replace, err.Error()), nil
	}

	if len(id) != 0 {
		image, err := a.unsplashApp.GetImage(r.Context(), id)
		if err != nil {
			return discord.NewEphemeral(replace, err.Error()), nil
		}

		return a.memeResponse(webhook.Member.User.ID, search, caption, image), nil
	}

	if len(search) != 0 {
		return a.handleSearch(r.Context(), webhook.Token, search, caption, replace)
	}

	return discord.NewEphemeral(replace, "Ok, not now."), nil
}

func (a App) parseQuery(webhook discord.InteractionRequest) (replace bool, id string, search string, caption string, err error) {
	if webhook.Type == discord.ApplicationCommandInteraction {
		for _, option := range webhook.Data.Options {
			if strings.EqualFold(option.Name, searchParam) {
				search = option.Value
			} else if strings.EqualFold(option.Name, captionParam) {
				caption = option.Value
			}
		}

		return
	}

	if webhook.Type == discord.MessageComponentInteraction {
		replace = true

		parts := strings.Split(webhook.Data.CustomID, contentSeparator)

		switch parts[0] {
		case "send":
			if len(parts) != 3 {
				err = fmt.Errorf("invalid format for sending image: `%s`", webhook.Data.CustomID)
			}
			id = parts[1]
			caption = parts[2]
		case "another":
			if len(parts) != 3 {
				err = fmt.Errorf("invalid format for another image: `%s`", webhook.Data.CustomID)
			}
			search = parts[1]
			caption = parts[2]
		case "cancel":
		}
	}

	return
}

func (a App) handleSearch(ctx context.Context, interactionToken, search, caption string, replace bool) (discord.InteractionResponse, func() discord.InteractionResponse) {
	image, err := a.unsplashApp.GetRandomImage(ctx, search)
	if err != nil {
		return discord.NewEphemeral(replace, fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err)), nil
	}

	return a.asyncResponse(replace), func() discord.InteractionResponse {
		output, err := a.generateImage(context.Background(), image.Raw, caption)
		if err != nil {
			logger.Error("unable to generate image for id `%s`: %s", image.ID, err)
		} else {
			a.storeInCache(image.ID, "", caption, output)
		}

		return a.interactiveResponse(search, caption, image, replace)
	}
}

func (a App) asyncResponse(replace bool) discord.InteractionResponse {
	response := discord.InteractionResponse{
		Type: discord.DeferredChannelMessageWithSourceCallback,
	}

	if replace {
		response.Type = discord.DeferredUpdateMessageCallback
	}

	response.Data.Flags = discord.EphemeralMessage

	return response
}

func (a App) interactiveResponse(search, caption string, image unsplash.Image, replace bool) discord.InteractionResponse {
	response := a.basicResponse(search, caption, image)
	response.Data.Flags = discord.EphemeralMessage
	if replace {
		response.Type = discord.UpdateMessageCallback
	}

	response.Data.Components = []discord.Component{
		{
			Type: discord.ActionRowType,
			Components: []discord.Component{
				discord.NewButton(discord.PrimaryButton, "Send", fmt.Sprintf("send%s%s%s%s", contentSeparator, image.ID, contentSeparator, caption)),
				discord.NewButton(discord.SecondaryButton, "Another?", fmt.Sprintf("another%s%s%s%s", contentSeparator, search, contentSeparator, caption)),
				discord.NewButton(discord.DangerButton, "Cancel", fmt.Sprintf("cancel")),
			},
		},
	}

	return response
}

func (a App) memeResponse(user, search, caption string, image unsplash.Image) discord.InteractionResponse {
	response := a.basicResponse(search, caption, image)
	response.Data.Content = fmt.Sprintf("<@!%s> shares a meme", user)

	return response
}

func (a App) basicResponse(search, caption string, image unsplash.Image) discord.InteractionResponse {
	instance := discord.InteractionResponse{Type: discord.ChannelMessageWithSourceCallback}
	instance.Data.AllowedMentions = discord.AllowedMention{
		Parse: []string{},
	}
	instance.Data.Embeds = []discord.Embed{a.getImageEmbed(search, caption, image)}

	return instance
}

func (a App) getImageEmbed(search, caption string, image unsplash.Image) discord.Embed {
	return discord.Embed{
		Title: "Unsplash image",
		URL:   image.URL,
		Image: discord.Image{
			URL: fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(image.ID), url.QueryEscape(caption)),
		},
		Author: discord.Author{
			Name: image.Author,
			URL:  image.AuthorURL,
		},
	}
}
