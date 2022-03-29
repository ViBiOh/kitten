package unsplash

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/request"
)

// Image describe an image use by app
type Image struct {
	ID        string
	URL       string
	Author    string
	AuthorURL string
}

// IsZero checks if instance has value
func (i Image) IsZero() bool {
	return len(i.ID) == 0
}

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
func (a App) GetImage(ctx context.Context, id string) (Image, error) {
	resp, err := a.unplashReq.Path(fmt.Sprintf("/photos/%s", url.PathEscape(id))).Send(ctx, nil)
	if err != nil {
		return Image{}, fmt.Errorf("unable to get image `%s`: %s", id, err)
	}

	return getImageFromResponse(ctx, resp)
}

// GetRandomImage from unsplash for given keyword
func (a App) GetRandomImage(ctx context.Context, query string) (Image, error) {
	resp, err := a.unplashReq.Path(fmt.Sprintf("/photos/random?query=%s", url.QueryEscape(query))).Send(ctx, nil)
	if err != nil {
		return Image{}, fmt.Errorf("unable to get random image: %s", err)
	}

	return getImageFromResponse(ctx, resp)
}

func getImageFromResponse(ctx context.Context, resp *http.Response) (output Image, err error) {
	var imageContent unsplashResponse
	if err = httpjson.Read(resp, &imageContent); err != nil {
		err = fmt.Errorf("unable to parse random response: %s", err)
		return
	}

	output.ID = imageContent.ID
	output.URL = fmt.Sprintf("%s?fm=png&w=800&fit=max", imageContent.URLs["raw"])
	output.Author = imageContent.User.Name
	output.AuthorURL = imageContent.User.Links["html"]

	return
}
