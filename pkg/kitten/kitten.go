package kitten

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/kitten/pkg/tenor"
	"github.com/ViBiOh/kitten/pkg/unsplash"
	"github.com/fogleman/gg"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	bufferPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 32*1024))
		},
	}

	cacheDuration        = time.Hour * 24 * 7
	cacheControlDuration = fmt.Sprintf("public, max-age=%.0f", cacheDuration.Seconds())
)

type Service struct {
	redisClient     redis.Client
	tracer          trace.Tracer
	cachedMetric    metric.Int64Counter
	servedMetric    metric.Int64Counter
	tmpFolder       string
	website         string
	unsplashService unsplash.Service
	tenorService    tenor.Service
}

type Config struct {
	TmpFolder string
}

func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) *Config {
	var config Config

	flags.New("TmpFolder", "Temp folder for storing cache image").Prefix(prefix).DocPrefix("kitten").StringVar(fs, &config.TmpFolder, "/tmp", overrides)

	return &config
}

func New(config *Config, unsplashService unsplash.Service, tenorService tenor.Service, redisClient redis.Client, meterProvider metric.MeterProvider, tracerProvider trace.TracerProvider, website string) Service {
	service := Service{
		unsplashService: unsplashService,
		tenorService:    tenorService,
		redisClient:     redisClient,
		website:         website,
		tmpFolder:       config.TmpFolder,
	}

	if meterProvider != nil {
		meter := meterProvider.Meter("github.com/ViBiOh/kitten/pkg/kitten")

		var err error

		service.cachedMetric, err = meter.Int64Counter("kitten.image_cached")
		if err != nil {
			slog.Error("create cached counter", "err", err)
		}

		service.servedMetric, err = meter.Int64Counter("kitten.image_served")
		if err != nil {
			slog.Error("create served counter", "err", err)
		}
	}

	if tracerProvider != nil {
		service.tracer = tracerProvider.Tracer("kitten")
	}

	return service
}

func (a Service) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		query, err := getQuery(r)
		if err != nil {
			httperror.BadRequest(r.Context(), w, err)
			return
		}

		id, _, caption, err := parseRequest(query)
		if err != nil {
			httperror.BadRequest(r.Context(), w, err)
			return
		}

		if a.serveCached(r.Context(), w, id, caption, false) {
			return
		}

		imageOutput, err := a.GetFromUnsplash(r.Context(), id, caption)
		if err != nil {
			httperror.InternalServerError(r.Context(), w, err)
			return
		}

		w.Header().Add("Cache-Control", cacheControlDuration)
		w.Header().Set("Content-Type", "imageOutput/jpeg")
		w.WriteHeader(http.StatusOK)
		if err = jpeg.Encode(w, imageOutput, &jpeg.Options{Quality: 80}); err != nil {
			httperror.InternalServerError(r.Context(), w, err)
			return
		}

		a.increaseServed(r.Context())

		go a.storeInCache(r.Context(), id, caption, imageOutput)
	})
}

func getQuery(r *http.Request) (url.Values, error) {
	urlPath := strings.TrimPrefix(r.URL.Path, "/")
	if len(urlPath) == 0 {
		return r.URL.Query(), nil
	}

	content, err := base64.URLEncoding.DecodeString(urlPath)
	if err != nil {
		return nil, fmt.Errorf("decode content: %w", err)
	}

	query, err := url.ParseQuery(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse content: %w", err)
	}

	return query, nil
}

func parseRequest(query url.Values) (string, string, string, error) {
	id := strings.TrimSpace(query.Get("id"))
	if len(id) == 0 {
		return "", "", "", fmt.Errorf("id param is required")
	}

	search := strings.TrimSpace(query.Get("search"))

	caption := strings.TrimSpace(query.Get("caption"))
	if len(caption) == 0 {
		return "", "", "", fmt.Errorf("caption param is required")
	}

	return id, search, caption, nil
}

func (a Service) caption(imageCtx *gg.Context, text string) (image.Image, error) {
	fontSize := float64(imageCtx.Width()) * fontSizeCoeff
	fontFace, resolve := getFontFace(fontSize)
	defer resolve()

	imageCtx.SetFontFace(fontFace)

	lines := imageCtx.WordWrap(strings.ToUpper(text), float64(imageCtx.Width())*widthPadding)
	xAnchor := float64(imageCtx.Width() / 2)
	yAnchor := fontSize / 2

	n := float64(2)

	for _, lineString := range lines {
		yAnchor += fontSize

		imageCtx.SetRGBA(0, 0, 0, 1)
		for dy := -n; dy <= n; dy++ {
			for dx := -n; dx <= n; dx++ {
				imageCtx.DrawStringAnchored(lineString, xAnchor+dx, yAnchor+dy, 0.5, 0.5)
			}
		}

		imageCtx.SetRGBA(1, 1, 1, 1)
		imageCtx.DrawStringAnchored(lineString, xAnchor, yAnchor, 0.5, 0.5)
	}

	return imageCtx.Image(), nil
}
