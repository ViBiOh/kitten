package unsplash

import (
	"context"
	"flag"
	"fmt"
	"image"
	"net/url"
	"strings"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/request"
)

type randomResponse struct {
	URLs map[string]string `json:"urls"`
}

const unsplashRoot = "https://api.unsplash.com"

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

// GetRandomImage from unsplash for given keyword
func (a App) GetRandomImage(ctx context.Context, query string) (image.Image, error) {
	resp, err := a.unplashReq.Path(fmt.Sprintf("/photos/random?query=%s", url.QueryEscape(query))).Send(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get random image: %s", err)
	}

	var random randomResponse
	if err = httpjson.Read(resp, &random); err != nil {
		return nil, fmt.Errorf("unable to parse random response: %s", err)
	}

	resp, err = request.Get(fmt.Sprintf("%s?fm=png&w=800&fit=max", random.URLs["raw"])).Send(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to download image: %s", err)
	}

	image, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to decode image: %s", err)
	}

	return image, nil
}
