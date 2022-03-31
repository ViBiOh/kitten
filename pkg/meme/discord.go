package meme

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
func (a App) DiscordHandler(r *http.Request, webhook discord.InteractionRequest) discord.InteractionResponse {
	id, search, caption, err := a.parseQuery(webhook)
	if err != nil {
		return discord.NewEphemeral(true, err.Error())
	}

	if len(id) != 0 {
		image, err := a.unsplashApp.GetImage(r.Context(), id)
		if err != nil {
			return discord.NewEphemeral(true, err.Error())
		}
		return a.memeResponse(webhook.Member.User.ID, search, caption, image)
	}

	if len(search) != 0 {
		return a.handleSearch(r.Context(), search, caption, true)
	}

	return discord.NewEphemeral(true, "Ok, not now.")
}

func (a App) parseQuery(webhook discord.InteractionRequest) (id string, search string, caption string, err error) {
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

func (a App) handleSearch(ctx context.Context, search, caption string, replace bool) discord.InteractionResponse {
	image, err := a.unsplashApp.GetRandomImage(ctx, search)
	if err != nil {
		return discord.NewEphemeral(replace, fmt.Sprintf("Oh! It's broken 😱. Reason is: %s", err))
	}

	return a.interactiveResponse(search, caption, image, replace)
}

func (a App) interactiveResponse(search, caption string, image unsplash.Image, replace bool) discord.InteractionResponse {
	webhookType := discord.ChannelMessageWithSourceCallback
	if replace {
		webhookType = discord.UpdateMessageCallback
	}

	instance := discord.InteractionResponse{Type: webhookType}
	instance.Data.Flags = discord.EphemeralMessage
	instance.Data.Embeds = []discord.Embed{a.getImageEmbed(search, caption, image)}
	instance.Data.Components = []discord.Component{
		{
			Type: discord.ActionRowType,
			Components: []discord.Component{
				discord.NewButton(discord.PrimaryButton, "Send", fmt.Sprintf("send%s%s%s%s", contentSeparator, image.ID, contentSeparator, caption)),
				discord.NewButton(discord.SecondaryButton, "Another?", fmt.Sprintf("another%s%s%s%s", contentSeparator, search, contentSeparator, caption)),
				discord.NewButton(discord.DangerButton, "Cancel", fmt.Sprintf("cancel")),
			},
		},
	}

	return instance
}

func (a App) memeResponse(user, search, caption string, image unsplash.Image) discord.InteractionResponse {
	instance := discord.InteractionResponse{Type: discord.ChannelMessageWithSourceCallback}
	instance.Data.Content = fmt.Sprintf("<@!%s> shares an image of <%s?utm_source=SayIt&utm_medium=referral|%s> from <%s?utm_source=SayIt&utm_medium=referral|Unsplash>", user, image.AuthorURL, image.Author, image.URL)
	instance.Data.AllowedMentions = discord.AllowedMention{
		Parse: []string{},
	}
	instance.Data.Embeds = []discord.Embed{a.getImageEmbed(search, caption, image)}

	return instance
}

func (a App) getImageEmbed(search, caption string, image unsplash.Image) discord.Embed {
	return discord.Embed{
		Title: search,
		Images: []discord.Image{
			{
				URL: fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(image.ID), url.QueryEscape(caption)),
			},
		},
	}
}
