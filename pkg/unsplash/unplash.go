package unsplash

import (
	"context"
	"flag"
	"fmt"
	"image"
	"net/http"
	"net/url"
	"strings"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/request"
)

type unsplashUser struct {
	Name  string            `json:"name"`
	Links map[string]string `json:"links"`
}

type unsplashResponse struct {
	ID   string            `json:"id"`
	URLs map[string]string `json:"urls"`
	User unsplashUser      `json:"user"`
}

const (
	unsplashRoot   = "https://api.unsplash.com"
	unsplashImages = "https://images.unsplash.com/"
)

// App of package
type App struct {
	unplashReq request.Request
}

// Config of package
type Config struct {
	unsplashAccessKey *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		unsplashAccessKey: flags.String(fs, prefix, "unsplash", "UnsplashAccessKey", "Unsplash Access Key", "", overrides),
	}
}

// New creates new App from Config
func New(config Config) App {
	return App{
		unplashReq: request.Get(unsplashRoot).Header("Authorization", fmt.Sprintf("Client-ID %s", strings.TrimSpace(*config.unsplashAccessKey))),
	}
}

// GetImage from unsplash for given keyword
func (a App) GetImage(ctx context.Context, id string) (image.Image, string, string, error) {
	resp, err := a.unplashReq.Path(fmt.Sprintf("/photos/%s", url.PathEscape(id))).Send(ctx, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to get image `%s`: %s", id, err)
	}

	return getImageFromResponse(ctx, resp)
}

// GetRandomImage from unsplash for given keyword
func (a App) GetRandomImage(ctx context.Context, query string) (image.Image, string, string, error) {
	resp, err := a.unplashReq.Path(fmt.Sprintf("/photos/random?query=%s", url.QueryEscape(query))).Send(ctx, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("unable to get random image: %s", err)
	}

	return getImageFromResponse(ctx, resp)
}

func getImageFromResponse(ctx context.Context, resp *http.Response) (output image.Image, id string, credits string, err error) {
	var imageContent unsplashResponse
	if err = httpjson.Read(resp, &imageContent); err != nil {
		err = fmt.Errorf("unable to parse random response: %s", err)
		return
	}

	id = imageContent.ID

	resp, err = request.Get(fmt.Sprintf("%s?fm=png&w=800&fit=max", imageContent.URLs["raw"])).Send(ctx, nil)
	if err != nil {
		err = fmt.Errorf("unable to download image: %s", err)
		return
	}

	output, _, err = image.Decode(resp.Body)
	if err != nil {
		err = fmt.Errorf("unable to decode image: %s", err)
		return
	}

	credits = fmt.Sprintf("%s|%s", imageContent.User.Name, imageContent.User.Links["html"])
	return
}
