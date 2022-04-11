package kitten

import (
	"context"
	"fmt"
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
		return discord.AsyncResponse(false, false), func() discord.InteractionResponse {
			return a.getDiscordOverrideResponse(ctx, webhook.Member.User.ID, search, caption)
		}
	}

	if len(id) != 0 {
		image, err := a.unsplashApp.GetImage(ctx, id)
		if err != nil {
			return discord.NewError(replace, err), nil
		}

		return discord.AsyncResponse(false, false), func() discord.InteractionResponse {
			return a.getDiscordUnsplashResponse(context.Background(), fmt.Sprintf("<@!%s> shares a meme", webhook.Member.User.ID), false, image, caption)
		}
	}

	if len(search) != 0 {
		return discord.AsyncResponse(replace, true), func() discord.InteractionResponse {
			return a.handleSearch(context.Background(), webhook.Token, search, caption, replace)
		}
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

	response := a.getDiscordUnsplashResponse(ctx, "", true, image, caption)
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

func (a App) getDiscordUnsplashResponse(ctx context.Context, content string, ephemeral bool, image unsplash.Image, caption string) discord.InteractionResponse {
	imagePath, size, err := a.generateAndStoreImage(ctx, image.ID, image.Raw, caption)
	if err != nil {
		return discord.NewError(false, fmt.Errorf("unable to generate image: %s", err))
	}

	resp := discord.NewResponse(discord.ChannelMessageWithSource, content)

	if ephemeral {
		resp = resp.Ephemeral()
	}

	return resp.AddAttachment("image.jpeg", imagePath, size).AddEmbed(discord.Embed{
		Title:  "Unsplash image",
		URL:    image.URL,
		Image:  discord.NewImage("attachment://image.jpeg"),
		Author: discord.NewAuthor(image.Author, image.AuthorURL),
	})
}

func (a App) getDiscordOverrideResponse(ctx context.Context, user, id, caption string) discord.InteractionResponse {
	imagePath, size, err := a.generateAndStoreImage(ctx, id, a.getOverride(id), caption)
	if err != nil {
		return discord.NewError(false, fmt.Errorf("unable to generate image: %s", err))
	}

	return discord.NewResponse(discord.ChannelMessageWithSource, fmt.Sprintf("<@!%s> shares a meme", user)).
		AddEmbed(discord.Embed{
			Title: id,
			Image: discord.NewImage("attachment://image.jpeg"),
		}).AddAttachment("image.jpeg", imagePath, size)
}
