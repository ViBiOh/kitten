package kitten

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
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
func Handler(memeApp meme.App, redisApp redis.App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()

		caption := strings.TrimSpace(query.Get("caption"))
		if len(caption) == 0 {
			httperror.BadRequest(w, errors.New("caption param is required"))
			return
		}

		id := strings.TrimSpace(query.Get("id"))
		from := strings.TrimSpace(query.Get("from"))

		if content, err := redisApp.Load(r.Context(), getCacheKey(id, from, caption)); err == nil && len(content) > 0 {
			if payload, err := base64.RawStdEncoding.DecodeString(content); err != nil {
				logger.Error("unable to decode image from cache: %s", err)
			} else {
				w.Header().Add("Cache-Control", cacheControlDuration)
				w.Header().Set("Content-Type", "image/png")
				if _, err = w.Write(payload); err != nil {
					logger.Error("unable to write image from cache: %s", err)
				}
				return
			}
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
			id = details.ID

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

		go func() {
			buffer := bufferPool.Get().(*bytes.Buffer)
			defer bufferPool.Put(buffer)

			if err := png.Encode(buffer, image); err != nil {
				logger.Error("unable to encode image for cache: %s", err)
			} else if err = redisApp.Store(context.Background(), getCacheKey(id, from, caption), base64.RawStdEncoding.EncodeToString(buffer.Bytes()), cacheDuration); err != nil {
				logger.Error("unable to write image to cache: %s", err)
			}
		}()
	})
}

func getCacheKey(id, from, caption string) string {
	return "kitten:" + sha.New(fmt.Sprintf("%s:%s:%s", id, from, caption))
}
