package kitten

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ViBiOh/kitten/pkg/discord"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

const (
	captionParam     = "caption"
	searchParam      = "search"
	idParam          = "id"
	contentSeparator = ":"

	memeName       = "meme"
	memeWithIDName = "memedi"
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
	memeWithIDName: {
		Name:        memeWithIDName,
		Description: "Generate a meme with caption from Unsplash Image ID",
		Options: []discord.CommandOption{
			{
				Name:        idParam,
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
func (a App) DiscordHandler(ctx context.Context, webhook discord.InteractionRequest) (discord.InteractionResponse, func() discord.InteractionResponse) {
	replace, id, search, caption, err := a.parseQuery(webhook)
	if err != nil {
		return discord.NewError(replace, err), nil
	}

	if a.isOverride(search) {
		return a.overrideResponse(webhook.Member.User.ID, search, caption), nil
	}

	if len(id) != 0 {
		image, err := a.unsplashApp.GetImage(ctx, id)
		if err != nil {
			return discord.NewError(replace, err), nil
		}

		return a.memeResponse(webhook.Member.User.ID, caption, image), nil
	}

	if len(search) != 0 {
		return a.handleSearch(ctx, webhook.Token, search, caption, replace), nil
	}

	return discord.NewEphemeral(replace, "Ok, not now."), nil
}

func (a App) parseQuery(webhook discord.InteractionRequest) (replace bool, id string, search string, caption string, err error) {
	if webhook.Type == discord.ApplicationCommandInteraction {
		for _, option := range webhook.Data.Options {
			switch option.Name {
			case idParam:
				id = option.Value
			case searchParam:
				search = option.Value
			case captionParam:
				caption = option.Value
			}
		}

		return
	}

	if webhook.Type == discord.MessageComponentInteraction {
		replace = true

		parts := strings.SplitN(webhook.Data.CustomID, contentSeparator, 3)

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

func (a App) handleSearch(ctx context.Context, interactionToken, search, caption string, replace bool) discord.InteractionResponse {
	image, err := a.unsplashApp.GetRandomImage(ctx, search)
	if err != nil {
		return discord.NewError(replace, err)
	}

	response := a.unsplashResponse(caption, image)
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

func (a App) memeResponse(user, caption string, image unsplash.Image) discord.InteractionResponse {
	response := a.unsplashResponse(caption, image)
	response.Data.Content = fmt.Sprintf("<@!%s> shares a meme", user)

	return response
}

func (a App) unsplashResponse(caption string, image unsplash.Image) discord.InteractionResponse {
	return discord.NewResponse(discord.ChannelMessageWithSource, "").AddEmbed(discord.Embed{
		Title: "Unsplash image",
		URL:   image.URL,
		Image: discord.NewImage(fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(image.ID), url.QueryEscape(caption))),
		Author: discord.Author{
			Name: image.Author,
			URL:  image.AuthorURL,
		},
	})
}

func (a App) overrideResponse(user, id, caption string) discord.InteractionResponse {
	return discord.NewResponse(discord.ChannelMessageWithSource, fmt.Sprintf("<@!%s> shares a meme", user)).AddEmbed(discord.Embed{
		Title: id,
		Image: discord.NewImage(fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(id), url.QueryEscape(caption))),
	})
}
