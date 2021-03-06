package kitten

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	prom "github.com/ViBiOh/httputils/v4/pkg/prometheus"
	"github.com/ViBiOh/kitten/pkg/giphy"
	"github.com/ViBiOh/kitten/pkg/unsplash"
	"github.com/fogleman/gg"
	"github.com/prometheus/client_golang/prometheus"
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

// App of package
type App struct {
	tracer       trace.Tracer
	cachedMetric prometheus.Counter
	servedMetric prometheus.Counter
	giphyApp     giphy.App
	tmpFolder    string
	website      string
	unsplashApp  unsplash.App
}

// Config of package
type Config struct {
	idsOverrides *string
	tmpFolder    *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		tmpFolder:    flags.String(fs, prefix, "kitten", "TmpFolder", "Temp folder for storing cache image", "/tmp", overrides),
		idsOverrides: flags.String(fs, prefix, "kitten", "IdsOverrides", "Ids overrides in the form key1|http1~key2|http2", "", overrides),
	}
}

// New creates new App from Config
func New(config Config, unsplashApp unsplash.App, giphyApp giphy.App, prometheusRegisterer prometheus.Registerer, tracer trace.Tracer, website string) App {
	return App{
		unsplashApp:  unsplashApp,
		giphyApp:     giphyApp,
		tracer:       tracer,
		website:      website,
		cachedMetric: prom.Counter(prometheusRegisterer, "kitten", "image", "cached"),
		servedMetric: prom.Counter(prometheusRegisterer, "kitten", "image", "served"),
		tmpFolder:    strings.TrimSpace(*config.tmpFolder),
	}
}

// Handler for image request. Should be use with net/http
func (a App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		query, err := getQuery(r)
		if err != nil {
			httperror.BadRequest(w, err)
			return
		}

		id, caption, err := parseRequest(query)
		if err != nil {
			httperror.BadRequest(w, err)
			return
		}

		if a.serveCached(w, id, caption, false) {
			return
		}

		image, err := a.GetFromUnsplash(r.Context(), id, caption)
		if err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		w.Header().Add("Cache-Control", cacheControlDuration)
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		if err = jpeg.Encode(w, image, &jpeg.Options{Quality: 80}); err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		a.increaseServed()

		go a.storeInCache(id, caption, image)
	})
}

func getQuery(r *http.Request) (url.Values, error) {
	urlPath := strings.TrimPrefix(r.URL.Path, "/")
	if len(urlPath) == 0 {
		return r.URL.Query(), nil
	}

	content, err := base64.URLEncoding.DecodeString(urlPath)
	if err != nil {
		return nil, fmt.Errorf("unable to decode content: %s", err)
	}

	query, err := url.ParseQuery(string(content))
	if err != nil {
		return nil, fmt.Errorf("unable to parse content: %s", err)
	}

	return query, nil
}

func parseRequest(query url.Values) (string, string, error) {
	id := strings.TrimSpace(query.Get("id"))
	if len(id) == 0 {
		return "", "", fmt.Errorf("id param is required")
	}

	caption := strings.TrimSpace(query.Get("caption"))
	if len(caption) == 0 {
		return "", "", fmt.Errorf("caption param is required")
	}

	return id, caption, nil
}

func (a App) caption(imageCtx *gg.Context, text string) (image.Image, error) {
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
