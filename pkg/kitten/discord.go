package kitten

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/kitten/pkg/giphy"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

const (
	captionParam     = "caption"
	searchParam      = "search"
	idParam          = "id"
	contentSeparator = ":"
)

// DiscordHandler handle discord request
func (a App) DiscordHandler(ctx context.Context, webhook discord.InteractionRequest) (discord.InteractionResponse, func(context.Context) discord.InteractionResponse) {
	replace, kind, id, search, caption, offset, err := a.parseQuery(webhook)
	if err != nil {
		return discord.NewError(replace, err), nil
	}

	if a.isOverride(search) {
		return discord.AsyncResponse(false, false), func(ctx context.Context) discord.InteractionResponse {
			return a.getDiscordOverrideResponse(ctx, webhook.Member.User.ID, search, caption)
		}
	}

	if len(id) != 0 {
		return a.handleSend(ctx, kind, id, caption, webhook.Member.User.ID)
	}

	if len(search) != 0 {
		return discord.AsyncResponse(replace, true), func(ctx context.Context) discord.InteractionResponse {
			return a.handleSearch(ctx, kind, webhook.Token, search, caption, replace, offset)
		}
	}

	return discord.NewEphemeral(replace, "Ok, not now."), nil
}

func (a App) parseQuery(webhook discord.InteractionRequest) (replace bool, kind memeKind, id string, search string, caption string, offset uint64, err error) {
	if webhook.Type == discord.ApplicationCommandInteraction {
		switch webhook.Data.Name {
		case "memegif":
			kind = gifKind
		default:
			kind = imageKind
		}

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

		parts := strings.SplitN(webhook.Data.CustomID, contentSeparator, 5)

		switch parts[0] {
		case "send":
			if len(parts) != 4 {
				err = fmt.Errorf("invalid format for sending image: `%s`", webhook.Data.CustomID)
			}
			kind = parseKind(parts[1])
			id = parts[2]
			caption = parts[3]
		case "another":
			if len(parts) != 5 {
				err = fmt.Errorf("invalid format for another image: `%s`", webhook.Data.CustomID)
			}
			kind = parseKind(parts[1])
			search = parts[2]
			caption = parts[3]
			offset, _ = strconv.ParseUint(parts[4], 10, 64)
		case "cancel":
		}
	}

	return
}

func (a App) handleSend(ctx context.Context, kind memeKind, id, caption, userID string) (discord.InteractionResponse, func(context.Context) discord.InteractionResponse) {
	switch kind {
	case gifKind:
		image, err := a.giphyApp.Get(ctx, id)
		if err != nil {
			return discord.NewError(true, err), nil
		}

		go a.giphyApp.SendAnalytics(ctx, image)

		return discord.AsyncResponse(false, false), func(ctx context.Context) discord.InteractionResponse {
			return a.getDiscordGiphyResponse(ctx, fmt.Sprintf("<@!%s> shares a meme", userID), false, image, caption)
		}
	default:
		image, err := a.unsplashApp.Get(ctx, id)
		if err != nil {
			return discord.NewError(true, err), nil
		}

		go a.unsplashApp.SendDownload(ctx, image)

		return discord.AsyncResponse(false, false), func(ctx context.Context) discord.InteractionResponse {
			return a.getDiscordUnsplashResponse(ctx, fmt.Sprintf("<@!%s> shares a meme", userID), false, image, caption)
		}
	}
}

func (a App) handleSearch(ctx context.Context, kind memeKind, interactionToken, search, caption string, replace bool, offset uint64) discord.InteractionResponse {
	var response discord.InteractionResponse
	var id string

	switch kind {
	case gifKind:
		image, err := a.giphyApp.Search(ctx, search, offset)
		if err != nil {
			return discord.NewError(replace, err)
		}
		response = a.getDiscordGiphyResponse(ctx, "", true, image, caption)
		id = image.ID
	default:
		image, err := a.unsplashApp.Search(ctx, search)
		switch err {
		case nil:
			response = a.getDiscordUnsplashResponse(ctx, "", true, image, caption)
			id = image.ID
		case giphy.ErrNotFound:
			return discord.NewEphemeral(replace, "No gif found")
		default:
			return discord.NewError(replace, err)
		}
	}

	if replace {
		response.Type = discord.UpdateMessageCallback
	}

	response.Data.Components = []discord.Component{
		{
			Type: discord.ActionRowType,
			Components: []discord.Component{
				discord.NewButton(discord.PrimaryButton, "Send", strings.Join([]string{"send", string(kind), id, caption}, contentSeparator)),
				discord.NewButton(discord.SecondaryButton, "Another?", strings.Join([]string{"another", string(kind), search, caption, strconv.FormatUint(offset+1, 10)}, contentSeparator)),
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

func (a App) getDiscordGiphyResponse(ctx context.Context, content string, ephemeral bool, image giphy.Gif, caption string) discord.InteractionResponse {
	imagePath, size, err := a.generateAndStoreGif(ctx, image.ID, image.Images["downsized"].URL, caption)
	if err != nil {
		return discord.NewError(false, fmt.Errorf("unable to generate gif: %s", err))
	}

	resp := discord.NewResponse(discord.ChannelMessageWithSource, content)

	if ephemeral {
		resp = resp.Ephemeral()
	}

	return resp.AddAttachment("meme.gif", imagePath, size).AddEmbed(discord.Embed{
		Title:  "Powered By GIPHY",
		URL:    image.URL,
		Image:  discord.NewImage("attachment://meme.gif"),
		Author: discord.NewAuthor(image.User.Username, image.User.ProfileURL),
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
