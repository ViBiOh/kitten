package tenor

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/cache"
	"github.com/ViBiOh/httputils/v4/pkg/cntxt"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/kitten/pkg/version"
	"go.opentelemetry.io/otel/trace"
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

type Service struct {
	cache     *cache.Cache[string, ResponseObject]
	apiKey    string
	clientKey string
	req       request.Request
}

type Config struct {
	apiKey    string
	clientKey string
}

func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) *Config {
	var config Config

	flags.New("ApiKey", "API Key").Prefix(prefix).DocPrefix("tenor").StringVar(fs, &config.apiKey, "", overrides)
	flags.New("ClientKey", "Client Key").Prefix(prefix).DocPrefix("tenor").StringVar(fs, &config.clientKey, "", overrides)

	return &config
}

func New(ctx context.Context, config *Config, redisClient redis.Client, tracerProvider trace.TracerProvider) Service {
	service := Service{
		req:       request.Get(root).WithClient(request.CreateClient(time.Second*30, request.NoRedirection)),
		apiKey:    url.QueryEscape(config.apiKey),
		clientKey: url.QueryEscape(config.clientKey),
	}

	service.cache = cache.New(redisClient, cacheID, func(ctx context.Context, id string) (ResponseObject, error) {
		resp, err := service.req.Path("/posts?key=%s&client_key=%s&ids=%s", service.apiKey, service.clientKey, url.QueryEscape(id)).Send(ctx, nil)
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
	}, tracerProvider).
		WithTTL(cacheDuration).
		WithExtendOnHit(ctx, cacheDuration/4, 50).
		WithClientSideCaching(ctx, "kitten_tenor", 50)

	return service
}

func (s Service) Search(ctx context.Context, query string, pos string) (ResponseObject, string, error) {
	resp, err := s.req.Path(fmt.Sprintf("/search?key=%s&client_key=%s&q=%s&limit=1&pos=%s&media_filter=mediumgif,tinygif", s.apiKey, s.clientKey, url.QueryEscape(query), url.QueryEscape(pos))).Send(ctx, nil)
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
			if err = s.cache.Store(ctx, gif.ID, gif); err != nil {
				slog.ErrorContext(ctx, "save gif in cache", "error", err)
			}
		}(cntxt.WithoutDeadline(ctx))
	}

	return gif, search.Next, nil
}

func (s Service) Get(ctx context.Context, id string) (ResponseObject, error) {
	return s.cache.Get(ctx, id)
}

func (s Service) SendAnalytics(ctx context.Context, content ResponseObject, query string) {
	resp, err := s.req.Path("/registershare?key=%s&client_key=%s&id=%s&q=%s", s.apiKey, s.clientKey, url.QueryEscape(content.ID), url.QueryEscape(query)).Send(ctx, nil)
	if err != nil {
		slog.ErrorContext(ctx, "send share events to tenor", "error", err)
		return
	}

	if err = request.DiscardBody(resp.Body); err != nil {
		slog.ErrorContext(ctx, "discard analytics from tenor", "error", err)
	}
}

func cacheID(id string) string {
	return version.Redis("tenor:" + id)
}
