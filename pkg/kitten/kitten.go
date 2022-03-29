package kitten

import (
	"errors"
	"image/png"
	"net/http"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/kitten/pkg/meme"
)

// Handler for Hello request. Should be use with net/http
func Handler(memeApp meme.App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()

		search := strings.TrimSpace(query.Get("search"))
		if len(search) == 0 {
			httperror.BadRequest(w, errors.New("search param is required"))
			return
		}

		caption := strings.TrimSpace(query.Get("caption"))
		if len(caption) == 0 {
			httperror.BadRequest(w, errors.New("caption param is required"))
			return
		}

		image, _, _, err := memeApp.GetFromUnsplash(r.Context(), search, caption)
		if err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		if err = png.Encode(w, image); err != nil {
			httperror.InternalServerError(w, err)
			return
		}
	})
}
