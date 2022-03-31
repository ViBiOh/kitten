package kitten

import (
	"bytes"
	"errors"
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

	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/sha"
	"github.com/ViBiOh/kitten/pkg/meme"
	"github.com/ViBiOh/kitten/pkg/unsplash"
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

// Handler for Hello request. Should be use with net/http
func Handler(memeApp meme.App, tmpFolder string) http.Handler {
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
			httperror.BadRequest(w, errors.New("caption param is required"))
			return
		}

		if serveCached(w, tmpFolder, id, from, caption) {
			return
		}

		var image image.Image
		var details unsplash.Image
		var err error

		search := strings.TrimSpace(query.Get("search"))

		if len(from) != 0 {
			image, err = memeApp.GetFromURL(r.Context(), from, caption)
		} else if len(id) == 0 && len(search) == 0 {
			httperror.BadRequest(w, errors.New("search param is required"))
		} else {
			image, details, err = memeApp.GetFromUnsplash(r.Context(), id, search, caption)
		}

		if err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		if !details.IsZero() {
			// id = details.ID

			w.Header().Set("X-Image-ID", details.ID)
			w.Header().Set("X-Image-Author", details.Author)
			w.Header().Set("X-Image-Author-URL", details.AuthorURL)
		}

		w.Header().Add("Cache-Control", cacheControlDuration)
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		if err = png.Encode(w, image); err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		go storeInCache(tmpFolder, id, from, caption, image)
	})
}

func serveCached(w http.ResponseWriter, tmpFolder, id, from, caption string) bool {
	file, err := os.OpenFile(filepath.Join(tmpFolder, getCacheKey(id, from, caption)+".png"), os.O_RDONLY, 0o600)
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

	if _, err = io.CopyBuffer(w, file, buffer.Bytes()); err != nil {
		logger.Error("unable to write image from local cache: %s", err)
		return false
	}

	return true
}

func storeInCache(tmpFolder, id, from, caption string, image image.Image) {
	file, err := os.OpenFile(filepath.Join(tmpFolder, getCacheKey(id, from, caption)+".png"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		logger.Error("unable to open image to local cache: %s", err)
		return
	}

	if err := png.Encode(file, image); err != nil {
		logger.Error("unable to write image to local cache: %s", err)
	}
}

func getCacheKey(id, from, caption string) string {
	return "kitten:" + sha.New(fmt.Sprintf("%s:%s:%s", id, from, caption))
}
