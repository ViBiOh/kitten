package kitten

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/kitten/pkg/tenor"
	"github.com/ViBiOh/kitten/pkg/unsplash"
	"github.com/ViBiOh/kitten/pkg/version"
)

const (
	captionParam = "caption"
	searchParam  = "search"
	idParam      = "id"
)

var (
	cachePrefix  = version.Redis("discord")
	cancelAction = fmt.Sprintf("action=%s", url.QueryEscape(cancelValue))
)

func (s Service) DiscordHandler(ctx context.Context, webhook discord.InteractionRequest) (discord.InteractionResponse, bool, func(context.Context) discord.InteractionResponse) {
	replace, kind, id, search, caption, next, err := s.parseQuery(ctx, webhook)
	if err != nil {
		return discord.NewError(replace, err), false, nil
	}

	if len(id) != 0 {
		return s.handleDiscordSend(ctx, kind, id, search, caption, webhook.Member.User.ID)
	}

	if len(search) != 0 {
		return discord.AsyncResponse(replace, true), false, func(ctx context.Context) discord.InteractionResponse {
			return s.handleDiscordSearch(ctx, kind, search, caption, replace, next)
		}
	}

	return discord.NewEphemeral(replace, "Ok, not now."), true, nil
}

func (s Service) parseQuery(ctx context.Context, webhook discord.InteractionRequest) (replace bool, kind memeKind, id string, search string, caption string, next string, err error) {
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

		var values url.Values
		values, err = discord.RestoreCustomID(ctx, s.redisClient, cachePrefix, webhook.Data.CustomID, []string{cancelAction})
		if err != nil {
			return
		}

		switch values.Get("action") {
		case sendValue:
			kind = parseKind(values.Get("kind"))
			id = values.Get(idParam)
			caption = values.Get(captionParam)

		case nextValue:
			kind = parseKind(values.Get("kind"))
			search = values.Get(searchParam)
			caption = values.Get(captionParam)
			next = values.Get("next")

		case cancelValue:
			return
		}
	}

	return
}

func (s Service) handleDiscordSend(_ context.Context, kind memeKind, id, search, caption, userID string) (discord.InteractionResponse, bool, func(context.Context) discord.InteractionResponse) {
	interactionResponse := discord.NewReplace("Sending it...")
	delete := true
	if len(search) == 0 {
		interactionResponse = discord.AsyncResponse(false, false)
		delete = false
	}

	switch kind {
	case gifKind:
		return interactionResponse, delete, func(ctx context.Context) discord.InteractionResponse {
			image, err := s.tenorService.Get(ctx, id)
			if err != nil {
				return discord.NewError(true, err)
			}

			go s.tenorService.SendAnalytics(context.WithoutCancel(ctx), image, search)

			return s.getDiscordGifResponse(ctx, fmt.Sprintf("<@!%s> shares a meme", userID), false, image, caption)
		}

	default:
		return interactionResponse, delete, func(ctx context.Context) discord.InteractionResponse {
			image, err := s.unsplashService.Get(ctx, id)
			if err != nil {
				return discord.NewError(true, err)
			}

			go s.unsplashService.SendDownload(context.WithoutCancel(ctx), image)

			return s.getDiscordUnsplashResponse(ctx, fmt.Sprintf("<@!%s> shares a meme", userID), false, image, caption)
		}
	}
}

func (s Service) handleDiscordSearch(ctx context.Context, kind memeKind, search, caption string, replace bool, next string) discord.InteractionResponse {
	var response discord.InteractionResponse
	var id string

	switch kind {
	case gifKind:
		image, nextValue, err := s.tenorService.Search(ctx, search, next)
		if err != nil {
			return discord.NewError(replace, err)
		}
		response = s.getDiscordGifResponse(ctx, "", true, image, caption)
		id = image.ID
		next = nextValue

	default:
		image, err := s.unsplashService.Search(ctx, search)
		switch err {
		case nil:
			response = s.getDiscordUnsplashResponse(ctx, "", true, image, caption)
			id = image.ID
		default:
			return discord.NewError(replace, err)
		}
	}

	if replace {
		response.Type = discord.UpdateMessageCallback
	}

	sendValues := url.Values{}
	sendValues.Add("action", sendValue)
	sendValues.Add("kind", string(kind))
	sendValues.Add(idParam, id)
	sendValues.Add(searchParam, search)
	sendValues.Add(captionParam, caption)

	sendKey, err := discord.SaveCustomID(ctx, s.redisClient, cachePrefix, sendValues)
	if err != nil {
		return discord.NewError(replace, err)
	}

	nextValues := url.Values{}
	nextValues.Add("action", nextValue)
	nextValues.Add("kind", string(kind))
	nextValues.Add("next", next)
	nextValues.Add(searchParam, search)
	nextValues.Add(captionParam, caption)

	nextKey, err := discord.SaveCustomID(ctx, s.redisClient, cachePrefix, nextValues)
	if err != nil {
		return discord.NewError(replace, err)
	}

	response.Data.Components = []discord.Component{
		{
			Type: discord.ActionRowType,
			Components: []discord.Component{
				discord.NewButton(discord.PrimaryButton, "Send", sendKey),
				discord.NewButton(discord.SecondaryButton, "Another?", nextKey),
				discord.NewButton(discord.DangerButton, "Cancel", cancelAction),
			},
		},
	}

	return response
}

func (s Service) getDiscordUnsplashResponse(ctx context.Context, content string, ephemeral bool, image unsplash.Image, caption string) discord.InteractionResponse {
	imagePath, size, err := s.generateAndStoreImage(ctx, image.ID, image.Raw, caption)
	if err != nil {
		return discord.NewError(false, fmt.Errorf("generate image: %w", err))
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

func (s Service) getDiscordGifResponse(ctx context.Context, content string, ephemeral bool, image tenor.ResponseObject, caption string) discord.InteractionResponse {
	imagePath, size, err := s.generateAndStoreGif(ctx, image.ID, image.GetImageURL(), caption)
	if err != nil {
		return discord.NewError(false, fmt.Errorf("generate gif: %w", err))
	}

	resp := discord.NewResponse(discord.ChannelMessageWithSource, content)

	if ephemeral {
		resp = resp.Ephemeral()
	}

	return resp.AddAttachment("meme.gif", imagePath, size).AddEmbed(discord.Embed{
		Title: "Powered By tenor",
		URL:   image.URL,
		Image: discord.NewImage("attachment://meme.gif"),
	})
}
