package unsplash

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/cache"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/kitten/pkg/version"
	"go.opentelemetry.io/otel/trace"
)

type Image struct {
	ID          string
	Raw         string
	URL         string
	DownloadURL string
	Author      string
	AuthorURL   string
}

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
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	cacheDuration = time.Hour * 24 * 7
)

type Service struct {
	cache       *cache.Cache[string, Image]
	appName     string
	req         request.Request
	downloadReq request.Request
}

type Config struct {
	appName   string
	accessKey string
}

func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) *Config {
	var config Config

	flags.New("Name", "Unsplash App name").Prefix(prefix).DocPrefix("unsplash").StringVar(fs, &config.appName, "SayIt", overrides)
	flags.New("AccessKey", "Unsplash Access Key").Prefix(prefix).DocPrefix("unsplash").StringVar(fs, &config.accessKey, "", overrides)

	return &config
}

func New(ctx context.Context, config *Config, redisClient redis.Client, tracerProvider trace.TracerProvider) Service {
	service := Service{
		req:         request.Get(root).Header("Authorization", fmt.Sprintf("Client-ID %s", config.accessKey)).WithClient(request.CreateClient(time.Second*30, request.NoRedirection)),
		downloadReq: request.New().Header("Authorization", fmt.Sprintf("Client-ID %s", config.accessKey)),
		appName:     config.appName,
	}

	service.cache = cache.New(redisClient, cacheID, func(ctx context.Context, id string) (Image, error) {
		resp, err := service.req.Path("/photos/%s", url.PathEscape(id)).Send(ctx, nil)
		if err != nil {
			if strings.Contains(err.Error(), "Rate Limit Exceeded") {
				return Image{}, ErrRateLimitExceeded
			}

			return Image{}, httperror.FromResponse(resp, fmt.Errorf("get image `%s`: %w", id, err))
		}

		return service.getImageFromResponse(resp)
	}, tracerProvider).
		WithTTL(cacheDuration).
		WithExtendOnHit(ctx, cacheDuration/4, 50).
		WithClientSideCaching(ctx, "kitten_unsplash", 50)

	return service
}

func (s Service) SendDownload(ctx context.Context, content Image) {
	if resp, err := s.downloadReq.Get(content.DownloadURL).Send(ctx, nil); err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "send download request to unsplash", slog.Any("error", err))
	} else if err = request.DiscardBody(resp.Body); err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "discard download body", slog.Any("error", err))
	}
}

func (s Service) Get(ctx context.Context, id string) (Image, error) {
	return s.cache.Get(ctx, id)
}

func (s Service) Search(ctx context.Context, query string) (Image, error) {
	resp, err := s.req.Path("/photos/random?query=%s&orientation=landscape", url.QueryEscape(query)).Send(ctx, nil)
	if err != nil {
		if strings.Contains(err.Error(), "Rate Limit Exceeded") {
			return Image{}, ErrRateLimitExceeded
		}

		var httpError request.RequestError
		if errors.As(err, &httpError) && httpError.StatusCode == http.StatusNotFound {
			return Image{}, httperror.FromResponse(resp, fmt.Errorf("nothing was found for the query `%s`", query))
		}

		return Image{}, httperror.FromResponse(resp, fmt.Errorf("get random image for `%s`: %w", query, err))
	}

	image, err := s.getImageFromResponse(resp)
	if err != nil {
		go func(ctx context.Context) {
			if err = s.cache.Store(ctx, image.ID, image); err != nil {
				slog.LogAttrs(ctx, slog.LevelError, "save image in cache", slog.Any("error", err))
			}
		}(context.WithoutCancel(ctx))
	}

	return image, err
}

func (s Service) getImageFromResponse(resp *http.Response) (Image, error) {
	imageContent, err := httpjson.Read[unsplashResponse](resp)
	if err != nil {
		return Image{}, fmt.Errorf("parse random response: %w", err)
	}

	return Image{
		ID:          imageContent.ID,
		Raw:         fmt.Sprintf("%s?fm=jpeg&w=800&fit=clip", imageContent.URLs["raw"]),
		URL:         fmt.Sprintf("%s?utm_source=%s&utm_medium=referral", imageContent.Links["html"], url.QueryEscape(s.appName)),
		DownloadURL: imageContent.Links["download_location"],
		Author:      imageContent.User.Name,
		AuthorURL:   fmt.Sprintf("%s?utm_source=%s&utm_medium=referral", imageContent.User.Links["html"], url.QueryEscape(s.appName)),
	}, nil
}

func cacheID(id string) string {
	return version.Redis("unsplash:" + id)
}
