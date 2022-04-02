package kitten

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	prom "github.com/ViBiOh/httputils/v4/pkg/prometheus"
	"github.com/ViBiOh/httputils/v4/pkg/sha"
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

	cacheDuration        = time.Hour * 24
	cacheControlDuration = fmt.Sprintf("public, max-age=%.0f", cacheDuration.Seconds())
)

// App of package
type App struct {
	unsplashApp  unsplash.App
	tracer       trace.Tracer
	cachedMetric prometheus.Counter
	servedMetric prometheus.Counter
	website      string
	tmpFolder    string
}

// Config of package
type Config struct {
	tmpFolder *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		tmpFolder: flags.String(fs, prefix, "kitten", "TmpFolder", "/tmp", "", overrides),
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
	}
}

// Handler for Hello request. Should be use with net/http
func (a App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()

		id := strings.TrimSpace(query.Get("id"))
		from := strings.TrimSpace(query.Get("from"))
		caption := strings.TrimSpace(query.Get("caption"))

		if len(caption) == 0 {
			httperror.BadRequest(w, fmt.Errorf("caption param is required for url `%s`", r.URL.String()))
			return
		}

		if a.serveCached(w, id, from, caption) {
			return
		}

		var image image.Image
		var err error

		search := strings.TrimSpace(query.Get("search"))

		if len(from) != 0 {
			image, err = a.GetFromURL(r.Context(), from, caption)
		} else if len(id) == 0 && len(search) == 0 {
			httperror.BadRequest(w, fmt.Errorf("search param is required for url `%s`", r.URL.String()))
		} else {
			image, _, err = a.GetFromUnsplash(r.Context(), id, search, caption)
		}

		if err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		w.Header().Add("Cache-Control", cacheControlDuration)
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		if err = png.Encode(w, image); err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		a.increaseServed()

		go a.storeInCache(id, from, caption, image)
	})
}

func (a App) serveCached(w http.ResponseWriter, id, from, caption string) bool {
	file, err := os.OpenFile(filepath.Join(a.tmpFolder, getRequestHash(id, from, caption)+".png"), os.O_RDONLY, 0o600)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Error("unable to open image from local cache: %s", err)
		}

		return false
	}

	buffer := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buffer)

	w.Header().Add("Cache-Control", cacheControlDuration)
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)

	if _, err = io.CopyBuffer(w, file, buffer.Bytes()); err != nil {
		logger.Error("unable to write image from local cache: %s", err)
		return false
	}

	a.increaseCached()

	return true
}

func (a App) storeInCache(id, from, caption string, image image.Image) {
	file, err := os.OpenFile(filepath.Join(a.tmpFolder, getRequestHash(id, from, caption)+".png"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		logger.Error("unable to open image to local cache: %s", err)
		return
	}

	if err := png.Encode(file, image); err != nil {
		logger.Error("unable to write image to local cache: %s", err)
	}
}

func getRequestHash(id, from, caption string) string {
	return sha.New(fmt.Sprintf("%s:%s:%s", id, from, caption))
}
