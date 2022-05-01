package giphy

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

const root = "https://api.giphy.com/v1"

var cacheDuration = time.Hour * 24 * 7

// Gif described from giphy API
type Gif struct {
	Images    map[string]image    `json:"images"`
	Analytics map[string]analytic `json:"analytics"`
	ID        string              `json:"id"`
}

// IsZero checks that instance is hydrated
func (g Gif) IsZero() bool {
	return len(g.ID) == 0
}

type analytic struct {
	URL string `json:"url"`
}

type image struct {
	Height string `json:"height"`
	Width  string `json:"width"`
	URL    string `json:"url"`
}

type randomResponse struct {
	Data Gif `json:"data"`
	Meta struct {
		Message string `json:"msg"`
		Status  uint64 `json:"uint64"`
	} `json:"meta"`
}

type randomIDResponse struct {
	ID string `json:"random_id"`
}

// App of package
type App struct {
	redisApp redis.App
	apiKey   string
	req      request.Request
}

// Config of package
type Config struct {
	apiKey *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		apiKey: flags.String(fs, prefix, "giphy", "ApiKey", "API Key", "", overrides),
	}
}

// New creates new App from Config
func New(config Config, redisApp redis.App) App {
	return App{
		req:      request.New().URL(root),
		apiKey:   url.QueryEscape(strings.TrimSpace(*config.apiKey)),
		redisApp: redisApp,
	}
}

// Search search from a gif from Giphy
func (a App) Search(ctx context.Context, query string) (Gif, error) {
	resp, err := a.req.Path(fmt.Sprintf("/random?api_key=%s&tag=%s", a.apiKey, url.QueryEscape(query))).Send(ctx, nil)
	if err != nil {
		return Gif{}, fmt.Errorf("unable to fetch random gif: %s", err)
	}

	gif, err := getGifFromResponse(resp)
	if err != nil {
		go func() {
			payload, err := json.Marshal(gif)
			if err != nil {
				logger.Error("unable to marshal gif for cache: %s", err)
			}

			if err = a.redisApp.Store(context.Background(), cacheID(gif.ID), payload, cacheDuration); err != nil {
				logger.Error("unable to save gif in cache: %s", err)
			}
		}()
	}

	return gif, nil
}

// Get gif by id
func (a App) Get(ctx context.Context, id string) (Gif, error) {
	return cache.Retrieve(ctx, a.redisApp, cacheID(id), func(ctx context.Context) (Gif, error) {
		resp, err := a.req.Path(fmt.Sprintf("/gifs/%s", url.PathEscape(id))).Send(ctx, nil)
		if err != nil {
			return Gif{}, fmt.Errorf("unable to get gif `%s`: %s", id, err)
		}

		return getGifFromResponse(resp)
	}, cacheDuration)
}

// SendAnalytics send anonymous analytics event
func (a App) SendAnalytics(ctx context.Context, content Gif) {
	analyticURL, ok := content.Analytics["onload"]
	if !ok {
		logger.Error("no `onload` analytics URL for giphy")
		return
	}

	resp, err := a.req.Method(http.MethodGet).Path(fmt.Sprintf("/randomid?api_key=%s", a.apiKey)).Send(ctx, nil)
	if err != nil {
		logger.Error("unable to get random id from giphy: %s", err)
		return
	}

	var randomID randomIDResponse
	if err = httpjson.Read(resp, &randomID); err != nil {
		logger.Error("unable to parse random id from giphy: %s", err)
		return
	}

	resp, err = request.Get(fmt.Sprintf("%s&api_key=%s&random_id=%s&ts=%d", analyticURL, a.apiKey, url.QueryEscape(randomID.ID), time.Now().Unix())).Send(ctx, nil)
	if err != nil {
		logger.Error("unable to send analytics to giphy: %s", err)
	}

	if err = request.DiscardBody(resp.Body); err != nil {
		logger.Error("unable to discard analytics body from giphy: %s", err)
	}
}

func getGifFromResponse(resp *http.Response) (Gif, error) {
	var random randomResponse
	if err := httpjson.Read(resp, &random); err != nil {
		return Gif{}, fmt.Errorf("unable to parse gif response: %s", err)
	}

	if random.Data.IsZero() {
		return Gif{}, errors.New("no gif found")
	}

	return random.Data, nil
}

func cacheID(id string) string {
	return "kitten:giphy:" + id
}
