package tenor

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
	"github.com/ViBiOh/kitten/pkg/version"
)

const root = "https://tenor.googleapis.com/v2/"

var (
	// ErrNotFound occurs when no git is found
	ErrNotFound = errors.New("no gif found")

	cacheDuration = time.Hour * 24 * 7
)

type image struct {
	Dimensions []int64 `json:"dims"`
	URL        string  `json:"url"`
}

// ResponseObject described from tenor API
type ResponseObject struct {
	Images map[string]image `json:"media_formats"`
	URL    string           `json:"url"`
	ID     string           `json:"id"`
}

type response struct {
	Results []ResponseObject `json:"results"`
	Next    string           `json:"next"`
}

// App of package
type App struct {
	redisApp  redis.App
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
		apiKey:    flags.String(fs, prefix, "tenor", "ApiKey", "API Key", "", overrides),
		clientKey: flags.String(fs, prefix, "tenor", "ClientKey", "Client Key", "", overrides),
	}
}

// New creates new App from Config
func New(config Config, redisApp redis.App) App {
	return App{
		req:       request.New().URL(root),
		apiKey:    url.QueryEscape(strings.TrimSpace(*config.apiKey)),
		clientKey: url.QueryEscape(strings.TrimSpace(*config.clientKey)),
		redisApp:  redisApp,
	}
}

// Search from a gif from Tenor
func (a App) Search(ctx context.Context, query string, pos string) (ResponseObject, string, error) {
	resp, err := a.req.Path(fmt.Sprintf("/search?key=%s&client_key=%s&q=%s&limit=1&pos=%s", a.apiKey, a.clientKey, url.QueryEscape(query), pos)).Send(ctx, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return ResponseObject{}, "", ErrNotFound
		}

		return ResponseObject{}, "", fmt.Errorf("search gif: %s", err)
	}

	var search response
	if err := httpjson.Read(resp, &search); err != nil {
		return ResponseObject{}, "", fmt.Errorf("parse gif response: %s", err)
	}

	if len(search.Results) == 0 || len(search.Next) == 0 {
		return ResponseObject{}, "", ErrNotFound
	}

	gif := search.Results[0]

	if err != nil {
		go func() {
			payload, err := json.Marshal(gif)
			if err != nil {
				logger.Error("marshal gif for cache: %s", err)
			}

			if err = a.redisApp.Store(context.Background(), cacheID(gif.ID), payload, cacheDuration); err != nil {
				logger.Error("save gif in cache: %s", err)
			}
		}()
	}

	return gif, search.Next, nil
}

// Get gif by id
func (a App) Get(ctx context.Context, id string) (ResponseObject, error) {
	return cache.Retrieve(ctx, a.redisApp, cacheID(id), func(ctx context.Context) (ResponseObject, error) {
		resp, err := a.req.Path(fmt.Sprintf("/posts?key=%s&client_key=%s&ids=%s", a.apiKey, a.clientKey, url.QueryEscape(id))).Send(ctx, nil)
		if err != nil {
			return ResponseObject{}, fmt.Errorf("get gif `%s`: %s", id, err)
		}

		var result response
		if err := httpjson.Read(resp, &result); err != nil {
			return ResponseObject{}, fmt.Errorf("parse gif response: %s", err)
		}

		if len(result.Results) == 0 {
			return ResponseObject{}, ErrNotFound
		}

		return result.Results[0], nil
	}, cacheDuration)
}

// SendAnalytics send anonymous analytics event
func (a App) SendAnalytics(ctx context.Context, content ResponseObject, query string) {
	resp, err := a.req.Path(fmt.Sprintf("/registershare?key=%s&client_key=%s&id=%s&q=%s", a.apiKey, a.clientKey, url.QueryEscape(content.ID), query)).Send(ctx, nil)
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
