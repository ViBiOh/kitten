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
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/kitten/pkg/version"
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
	Links map[string]string `json:"links"`
	Name  string            `json:"name"`
}

type unsplashResponse struct {
	User  unsplashUser      `json:"user"`
	URLs  map[string]string `json:"urls"`
	Links map[string]string `json:"links"`
	ID    string            `json:"id"`
}

const root = "https://api.unsplash.com"

var (
	// ErrRateLimitExceeded occurs when rate limit is exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	cacheDuration = time.Hour * 24 * 7
)

// App of package
type App struct {
	redisApp    redis.App
	appName     string
	req         request.Request
	downloadReq request.Request
}

// Config of package
type Config struct {
	appName   *string
	accessKey *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		appName:   flags.String(fs, prefix, "unsplash", "Name", "Unsplash App name", "SayIt", overrides),
		accessKey: flags.String(fs, prefix, "unsplash", "AccessKey", "Unsplash Access Key", "", overrides),
	}
}

// New creates new App from Config
func New(config Config, redisApp redis.App) App {
	return App{
		req:         request.Get(root).Header("Authorization", fmt.Sprintf("Client-ID %s", strings.TrimSpace(*config.accessKey))),
		downloadReq: request.New().Header("Authorization", fmt.Sprintf("Client-ID %s", strings.TrimSpace(*config.accessKey))),
		redisApp:    redisApp,
		appName:     strings.TrimSpace(*config.appName),
	}
}

// SendDownload event
func (a App) SendDownload(ctx context.Context, content Image) {
	if resp, err := a.downloadReq.Get(content.DownloadURL).Send(ctx, nil); err != nil {
		logger.Error("send download request to unsplash: %s", err)
	} else if err = request.DiscardBody(resp.Body); err != nil {
		logger.Error("discard download body: %s", err)
	}
}

// Get from unsplash for given id
func (a App) Get(ctx context.Context, id string) (Image, error) {
	return cache.Retrieve(ctx, a.redisApp, func(ctx context.Context) (Image, error) {
		resp, err := a.req.Path(fmt.Sprintf("/photos/%s", url.PathEscape(id))).Send(ctx, nil)
		if err != nil {
			if strings.Contains(err.Error(), "Rate Limit Exceeded") {
				return Image{}, ErrRateLimitExceeded
			}

			return Image{}, httperror.FromResponse(resp, fmt.Errorf("get image `%s`: %w", id, err))
		}

		return a.getImageFromResponse(ctx, resp)
	}, cacheDuration, cacheID(id))
}

// Search from unsplash for given keyword
func (a App) Search(ctx context.Context, query string) (Image, error) {
	resp, err := a.req.Path(fmt.Sprintf("/photos/random?query=%s&orientation=landscape", url.QueryEscape(query))).Send(ctx, nil)
	if err != nil {
		if strings.Contains(err.Error(), "Rate Limit Exceeded") {
			return Image{}, ErrRateLimitExceeded
		}

		return Image{}, httperror.FromResponse(resp, fmt.Errorf("get random image for `%s`: %w", query, err))
	}

	image, err := a.getImageFromResponse(ctx, resp)
	if err != nil {
		go func() {
			payload, err := json.Marshal(image)
			if err != nil {
				logger.Error("marshal image for cache: %s", err)
			}

			if err = a.redisApp.Store(context.Background(), cacheID(image.ID), payload, cacheDuration); err != nil {
				logger.Error("save image in cache: %s", err)
			}
		}()
	}

	return image, err
}

func (a App) getImageFromResponse(ctx context.Context, resp *http.Response) (output Image, err error) {
	var imageContent unsplashResponse
	if err = httpjson.Read(resp, &imageContent); err != nil {
		err = fmt.Errorf("parse random response: %w", err)
		return
	}

	output.ID = imageContent.ID
	output.Raw = fmt.Sprintf("%s?fm=jpeg&w=800&fit=clip", imageContent.URLs["raw"])
	output.URL = fmt.Sprintf("%s?utm_source=%s&utm_medium=referral", imageContent.Links["html"], url.QueryEscape(a.appName))
	output.DownloadURL = imageContent.Links["download_location"]
	output.Author = imageContent.User.Name
	output.AuthorURL = fmt.Sprintf("%s?utm_source=%s&utm_medium=referral", imageContent.User.Links["html"], url.QueryEscape(a.appName))

	return
}

func cacheID(id string) string {
	return version.Redis("unsplash:" + id)
}
