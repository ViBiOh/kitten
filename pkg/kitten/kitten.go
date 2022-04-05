package kitten

import (
	"bytes"
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
	"github.com/ViBiOh/kitten/pkg/unsplash"
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
	unsplashApp  unsplash.App
	tracer       trace.Tracer
	cachedMetric prometheus.Counter
	servedMetric prometheus.Counter
	idsOverrides map[string]string
	website      string
	tmpFolder    string
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
func New(config Config, unsplashApp unsplash.App, prometheusRegisterer prometheus.Registerer, tracer trace.Tracer, website string) App {
	return App{
		unsplashApp:  unsplashApp,
		tracer:       tracer,
		website:      website,
		cachedMetric: prom.Counter(prometheusRegisterer, "kitten", "image", "cached"),
		servedMetric: prom.Counter(prometheusRegisterer, "kitten", "image", "served"),
		tmpFolder:    strings.TrimSpace(*config.tmpFolder),
		idsOverrides: parseIdsOverrides(strings.TrimSpace(*config.idsOverrides)),
	}
}

// Handler for Hello request. Should be use with net/http
func (a App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		id, caption, err := parseRequest(r.URL.Query())
		if err != nil {
			httperror.BadRequest(w, err)
			return
		}

		if a.serveCached(w, id, caption) {
			return
		}

		var image image.Image

		if override := a.getOverride(id); len(override) != 0 {
			image, err = a.GetFromURL(r.Context(), override, caption)
		} else {
			image, err = a.GetFromUnsplash(r.Context(), id, caption)
		}

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
