package kitten

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"strings"
	"time"

	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/kitten/pkg/meme"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

var cacheDuration = fmt.Sprintf("public, max-age=%.0f", time.Duration(time.Hour*24).Seconds())

// Handler for Hello request. Should be use with net/http
func Handler(memeApp meme.App) http.Handler {
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

		var image image.Image
		var details unsplash.Image
		var err error

		id := strings.TrimSpace(query.Get("id"))
		from := strings.TrimSpace(query.Get("from"))
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
			w.Header().Set("X-Image-ID", details.ID)
			w.Header().Set("X-Image-Author", details.Author)
			w.Header().Set("X-Image-Author-URL", details.AuthorURL)
		}

		w.Header().Add("Cache-Control", cacheDuration)
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		if err = png.Encode(w, image); err != nil {
			httperror.InternalServerError(w, err)
			return
		}
	})
}
