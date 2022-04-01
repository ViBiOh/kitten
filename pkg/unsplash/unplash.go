package unsplash

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/cache"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/request"
)

// Image describe an image use by app
type Image struct {
	ID          string
	Raw         string
	URL         string
	DownloadURL string
	Author      string
	AuthorURL   string
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
	ID    string            `json:"id"`
	URLs  map[string]string `json:"urls"`
	Links map[string]string `json:"links"`
	User  unsplashUser      `json:"user"`
}

const (
	unsplashRoot = "https://api.unsplash.com"
)

var (
	// ErrRateLimitExceeded occurs when rate limit is exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	cacheDuration = time.Hour * 24
)

// App of package
type App struct {
	unplashReq  request.Request
	downloadReq request.Request
	redisApp    redis.App
	appName     string
}

// Config of package
type Config struct {
	appName           *string
	unsplashAccessKey *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		appName:           flags.String(fs, prefix, "unsplash", "Name", "Unsplash App name", "SayIt", overrides),
		unsplashAccessKey: flags.String(fs, prefix, "unsplash", "AccessKey", "Unsplash Access Key", "", overrides),
	}
}

// New creates new App from Config
func New(config Config, redisApp redis.App) App {
	return App{
		unplashReq:  request.Get(unsplashRoot).Header("Authorization", fmt.Sprintf("Client-ID %s", strings.TrimSpace(*config.unsplashAccessKey))),
		downloadReq: request.New().Header("Authorization", fmt.Sprintf("Client-ID %s", strings.TrimSpace(*config.unsplashAccessKey))),
		redisApp:    redisApp,
		appName:     strings.TrimSpace(*config.appName),
	}
}

// SendDownload send download event
func (a App) SendDownload(ctx context.Context, content Image) {
	if resp, err := a.downloadReq.Get(content.DownloadURL).Send(ctx, nil); err != nil {
		logger.Error("unable to send download request to unsplash: %s", err)
	} else if err = request.DiscardBody(resp.Body); err != nil {
		logger.Error("unable to discard download body: %s", err)
	}
}

// GetImage from unsplash for given keyword
func (a App) GetImage(ctx context.Context, id string) (Image, error) {
	return cache.Retrieve(ctx, a.redisApp, cacheID(id), func(ctx context.Context) (Image, error) {
		resp, err := a.unplashReq.Path(fmt.Sprintf("/photos/%s", url.PathEscape(id))).Send(ctx, nil)
		if err != nil {
			if strings.Contains(err.Error(), "Rate Limit Exceeded") {
				return Image{}, ErrRateLimitExceeded
			}

			return Image{}, fmt.Errorf("unable to get image `%s`: %s", id, err)
		}

		return a.getImageFromResponse(ctx, resp)
	}, cacheDuration)
}

// GetRandomImage from unsplash for given keyword
func (a App) GetRandomImage(ctx context.Context, query string) (Image, error) {
	resp, err := a.unplashReq.Path(fmt.Sprintf("/photos/random?query=%s&orientation=landscape", url.QueryEscape(query))).Send(ctx, nil)
	if err != nil {
		if strings.Contains(err.Error(), "Rate Limit Exceeded") {
			return Image{}, ErrRateLimitExceeded
		}

		return Image{}, fmt.Errorf("unable to get random image for `%s`: %s", query, err)
	}

	image, err := a.getImageFromResponse(ctx, resp)
	if err != nil {
		go func() {
			payload, err := json.Marshal(image)
			if err != nil {
				logger.Error("unable to marshal image for cache: %s", err)
			}

			if err = a.redisApp.Store(context.Background(), cacheID(image.ID), payload, cacheDuration); err != nil {
				logger.Error("unable to save image in cache: %s", err)
			}
		}()
	}

	return image, err
}

func (a App) getImageFromResponse(ctx context.Context, resp *http.Response) (output Image, err error) {
	var imageContent unsplashResponse
	if err = httpjson.Read(resp, &imageContent); err != nil {
		err = fmt.Errorf("unable to parse random response: %s", err)
		return
	}

	output.ID = imageContent.ID
	output.Raw = fmt.Sprintf("%s?fm=png&w=800&fit=clip", imageContent.URLs["raw"])
	output.URL = fmt.Sprintf("%s?utm_source=%s&utm_medium=referral", imageContent.Links["html"], url.QueryEscape(a.appName))
	output.DownloadURL = imageContent.Links["download_location"]
	output.Author = imageContent.User.Name
	output.AuthorURL = fmt.Sprintf("%s?utm_source=%s&utm_medium=referral", imageContent.User.Links["html"], url.QueryEscape(a.appName))

	return
}

func cacheID(id string) string {
	return "kitten:unsplash:" + id
}
