package tenor

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/cache"
	"github.com/ViBiOh/httputils/v4/pkg/cntxt"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/httputils/v4/pkg/tracer"
	"github.com/ViBiOh/kitten/pkg/version"
)

const (
	root        = "https://tenor.googleapis.com/v2/"
	maxFileSize = 4 << 20
)

var (
	ErrNotFound   = errors.New("no gif found")
	cacheDuration = time.Hour * 24 * 7
)

type image struct {
	URL  string `json:"url"`
	Size uint64 `json:"size"`
}

// ResponseObject described from tenor API
type ResponseObject struct {
	Images map[string]image `json:"media_formats"`
	URL    string           `json:"url"`
	ID     string           `json:"id"`
}

func (ro ResponseObject) GetImageURL() string {
	if medium, ok := ro.Images["mediumgif"]; ok && medium.Size < maxFileSize {
		return medium.URL
	}

	return ro.Images["tinygif"].URL
}

type response struct {
	Next    string           `json:"next"`
	Results []ResponseObject `json:"results"`
}

// App of package
type App struct {
	cacheApp  *cache.App[string, ResponseObject]
	apiKey    string
	clientKey string
	req       request.Request
}

// Config of package
type Config struct {
	apiKey    *string
	clientKey *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		apiKey:    flags.New("ApiKey", "API Key").Prefix(prefix).DocPrefix("tenor").String(fs, "", overrides),
		clientKey: flags.New("ClientKey", "Client Key").Prefix(prefix).DocPrefix("tenor").String(fs, "", overrides),
	}
}

// New creates new App from Config
func New(config Config, redisApp redis.Client, tracerApp tracer.App) App {
	app := App{
		req:       request.Get(root).WithClient(request.CreateClient(time.Second*30, request.NoRedirection)),
		apiKey:    url.QueryEscape(strings.TrimSpace(*config.apiKey)),
		clientKey: url.QueryEscape(strings.TrimSpace(*config.clientKey)),
	}

	app.cacheApp = cache.New(redisApp, cacheID, func(ctx context.Context, id string) (ResponseObject, error) {
		resp, err := app.req.Path("/posts?key=%s&client_key=%s&ids=%s", app.apiKey, app.clientKey, url.QueryEscape(id)).Send(ctx, nil)
		if err != nil {
			return ResponseObject{}, httperror.FromResponse(resp, fmt.Errorf("get gif: %w", err))
		}

		var result response
		if err := httpjson.Read(resp, &result); err != nil {
			return ResponseObject{}, fmt.Errorf("parse gif response: %w", err)
		}

		if len(result.Results) == 0 {
			return ResponseObject{}, ErrNotFound
		}

		return result.Results[0], nil
	}, cacheDuration, 6, tracerApp.GetTracer("tenor_cache"))

	return app
}

// Search from a gif from Tenor
func (a App) Search(ctx context.Context, query string, pos string) (ResponseObject, string, error) {
	resp, err := a.req.Path(fmt.Sprintf("/search?key=%s&client_key=%s&q=%s&limit=1&pos=%s&media_filter=mediumgif,tinygif", a.apiKey, a.clientKey, url.QueryEscape(query), url.QueryEscape(pos))).Send(ctx, nil)
	if err != nil {
		return ResponseObject{}, "", httperror.FromResponse(resp, fmt.Errorf("search gif: %w", err))
	}

	var search response
	if err := httpjson.Read(resp, &search); err != nil {
		return ResponseObject{}, "", fmt.Errorf("parse gif response: %w", err)
	}

	if len(search.Results) == 0 || len(search.Next) == 0 {
		return ResponseObject{}, "", ErrNotFound
	}

	gif := search.Results[0]

	if err != nil {
		go func(ctx context.Context) {
			if err = a.cacheApp.Store(ctx, gif.ID, gif); err != nil {
				logger.Error("save gif in cache: %s", err)
			}
		}(cntxt.WithoutDeadline(ctx))
	}

	return gif, search.Next, nil
}

// Get gif by id
func (a App) Get(ctx context.Context, id string) (ResponseObject, error) {
	return a.cacheApp.Get(ctx, id)
}

// SendAnalytics send anonymous analytics event
func (a App) SendAnalytics(ctx context.Context, content ResponseObject, query string) {
	resp, err := a.req.Path("/registershare?key=%s&client_key=%s&id=%s&q=%s", a.apiKey, a.clientKey, url.QueryEscape(content.ID), url.QueryEscape(query)).Send(ctx, nil)
	if err != nil {
		logger.Error("send share events to tenor: %s", err)
		return
	}

	if err = request.DiscardBody(resp.Body); err != nil {
		logger.Error("discard analytics from tenor: %s", err)
	}
}

func cacheID(id string) string {
	return version.Redis("tenor:" + id)
}
