package kitten

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/httputils/v4/pkg/sha"
	"github.com/ViBiOh/kitten/pkg/tenor"
	"github.com/ViBiOh/kitten/pkg/unsplash"
	"github.com/ViBiOh/kitten/pkg/version"
)

const (
	captionParam     = "caption"
	searchParam      = "search"
	idParam          = "id"
	contentSeparator = ":"
)

// DiscordHandler handle discord request
func (a App) DiscordHandler(ctx context.Context, webhook discord.InteractionRequest) (discord.InteractionResponse, func(context.Context) discord.InteractionResponse) {
	replace, kind, id, search, caption, next, err := a.parseQuery(ctx, webhook)
	if err != nil {
		return discord.NewError(replace, err), nil
	}

	if len(id) != 0 {
		return a.handleDiscordSend(ctx, kind, id, search, caption, webhook.Member.User.ID)
	}

	if len(search) != 0 {
		return discord.AsyncResponse(replace, true), func(ctx context.Context) discord.InteractionResponse {
			return a.handleDiscordSearch(ctx, kind, webhook.Token, search, caption, replace, next)
		}
	}

	return discord.NewEphemeral(replace, "Ok, not now."), nil
}

func (a App) parseQuery(ctx context.Context, webhook discord.InteractionRequest) (replace bool, kind memeKind, id string, search string, caption string, next string, err error) {
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

		var content string

		content, err = a.redisApp.Load(ctx, version.Redis(webhook.Data.CustomID))
		if err != nil {
			return
		}

		parts := strings.SplitN(content, contentSeparator, 5)

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
			next = parts[4]
		case "cancel":
		}
	}

	return
}

func (a App) handleDiscordSend(ctx context.Context, kind memeKind, id, search, caption, userID string) (discord.InteractionResponse, func(context.Context) discord.InteractionResponse) {
	switch kind {
	case gifKind:
		image, err := a.tenorApp.Get(ctx, id)
		if err != nil {
			return discord.NewError(true, err), nil
		}

		go a.tenorApp.SendAnalytics(context.Background(), image, search)

		return discord.AsyncResponse(false, false), func(ctx context.Context) discord.InteractionResponse {
			return a.getDiscordGifResponse(ctx, fmt.Sprintf("<@!%s> shares a meme", userID), false, image, caption)
		}
	default:
		image, err := a.unsplashApp.Get(ctx, id)
		if err != nil {
			return discord.NewError(true, err), nil
		}

		go a.unsplashApp.SendDownload(context.Background(), image)

		return discord.AsyncResponse(false, false), func(ctx context.Context) discord.InteractionResponse {
			return a.getDiscordUnsplashResponse(ctx, fmt.Sprintf("<@!%s> shares a meme", userID), false, image, caption)
		}
	}
}

func (a App) handleDiscordSearch(ctx context.Context, kind memeKind, interactionToken, search, caption string, replace bool, next string) discord.InteractionResponse {
	var response discord.InteractionResponse
	var id string

	switch kind {
	case gifKind:
		image, nextValue, err := a.tenorApp.Search(ctx, search, next)
		if err != nil {
			return discord.NewError(replace, err)
		}
		response = a.getDiscordGifResponse(ctx, "", true, image, caption)
		id = image.ID
		next = nextValue
	default:
		image, err := a.unsplashApp.Search(ctx, search)
		switch err {
		case nil:
			response = a.getDiscordUnsplashResponse(ctx, "", true, image, caption)
			id = image.ID
		default:
			return discord.NewError(replace, err)
		}
	}

	if replace {
		response.Type = discord.UpdateMessageCallback
	}

	sendContent := strings.Join([]string{"send", string(kind), id, caption}, contentSeparator)
	sendSha := sha.New(sendContent)
	if err := a.redisApp.Store(ctx, version.Redis(sendSha), sendContent, time.Hour); err != nil {
		return discord.NewError(replace, err)
	}

	nextContent := strings.Join([]string{"another", string(kind), search, caption, next}, contentSeparator)
	nextSha := sha.New(nextContent)
	if err := a.redisApp.Store(ctx, version.Redis(nextSha), nextContent, time.Hour); err != nil {
		return discord.NewError(replace, err)
	}

	response.Data.Components = []discord.Component{
		{
			Type: discord.ActionRowType,
			Components: []discord.Component{
				discord.NewButton(discord.PrimaryButton, "Send", sendSha),
				discord.NewButton(discord.SecondaryButton, "Another?", nextSha),
				discord.NewButton(discord.DangerButton, "Cancel", "cancel"),
			},
		},
	}

	return response
}

func (a App) getDiscordUnsplashResponse(ctx context.Context, content string, ephemeral bool, image unsplash.Image, caption string) discord.InteractionResponse {
	imagePath, size, err := a.generateAndStoreImage(ctx, image.ID, image.Raw, caption)
	if err != nil {
		return discord.NewError(false, fmt.Errorf("generate image: %s", err))
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

func (a App) getDiscordGifResponse(ctx context.Context, content string, ephemeral bool, image tenor.ResponseObject, caption string) discord.InteractionResponse {
	imagePath, size, err := a.generateAndStoreGif(ctx, image.ID, image.Images["mediumgif"].URL, caption)
	if err != nil {
		return discord.NewError(false, fmt.Errorf("generate gif: %s", err))
	}

	resp := discord.NewResponse(discord.ChannelMessageWithSource, content)

	if ephemeral {
		resp = resp.Ephemeral()
	}

	return resp.AddAttachment("meme.gif", imagePath, size).AddEmbed(discord.Embed{
		Title: "Powered By Tenor",
		URL:   image.URL,
		Image: discord.NewImage("attachment://meme.gif"),
	})
}
