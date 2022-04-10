package kitten

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/jpeg"
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
func (a App) DiscordHandler(ctx context.Context, webhook discord.InteractionRequest) discord.InteractionResponse {
	replace, id, search, caption, err := a.parseQuery(webhook)
	if err != nil {
		return discord.NewEphemeral(replace, err.Error())
	}

	if a.isOverride(search) {
		return a.overrideResponse(ctx, webhook.Member.User.ID, search, caption)
	}

	if len(id) != 0 {
		image, err := a.unsplashApp.GetImage(ctx, id)
		if err != nil {
			return discord.NewEphemeral(replace, err.Error())
		}

		return a.memeResponse(webhook.Member.User.ID, caption, image)
	}

	if len(search) != 0 {
		return a.handleSearch(ctx, webhook.Token, search, caption, replace)
	}

	return discord.NewEphemeral(replace, "Ok, not now.")
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
	response := discord.InteractionResponse{Type: discord.ChannelMessageWithSourceCallback}
	response.Data.AllowedMentions = discord.AllowedMention{
		Parse: []string{},
	}
	response.Data.Embeds = []discord.Embed{
		{
			Title: "Unsplash image",
			URL:   image.URL,
			Image: discord.Image{
				URL: fmt.Sprintf("%s/api/?id=%s&caption=%s", a.website, url.QueryEscape(image.ID), url.QueryEscape(caption)),
			},
			Author: discord.Author{
				Name: image.Author,
				URL:  image.AuthorURL,
			},
		},
	}

	return response
}

func (a App) overrideResponse(ctx context.Context, user, id, caption string) discord.InteractionResponse {
	image, err := a.generateImage(ctx, a.getOverride(id), caption)
	if err != nil {
		return discord.NewError(false, fmt.Errorf("unable to generate image: %s", err))
	}

	buffer := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buffer)

	if err = jpeg.Encode(base64.NewEncoder(base64.StdEncoding, buffer), image, &jpeg.Options{Quality: 80}); err != nil {
		return discord.NewError(false, fmt.Errorf("unable to encode image: %s", err))
	}

	response := discord.InteractionResponse{Type: discord.ChannelMessageWithSourceCallback}
	response.Data.AllowedMentions = discord.AllowedMention{
		Parse: []string{},
	}
	response.Data.Embeds = []discord.Embed{
		{
			Title: id,
			Image: discord.Image{
				URL: fmt.Sprintf("data:image/jpeg;base64,%s", buffer.Bytes()),
			},
		},
	}
	response.Data.Content = fmt.Sprintf("<@!%s> shares a meme", user)

	payload, _ := json.Marshal(response)
	fmt.Printf("%s", payload)

	return response
}
