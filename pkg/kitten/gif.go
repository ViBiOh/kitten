package kitten

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ViBiOh/httputils/v4/pkg/concurrent"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/httputils/v4/pkg/sha"
	"go.opentelemetry.io/otel/trace"
)

// GifHandler for gif request. Should be use with net/http
func (a App) GifHandler() http.Handler {
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

		if a.serveCached(w, id, caption, true) {
			return
		}

		image, err := a.GetFromGiphy(r.Context(), id, caption)
		if err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		w.Header().Add("Cache-Control", cacheControlDuration)
		w.Header().Set("Content-Type", "image/gif")
		w.WriteHeader(http.StatusOK)
		if err = gif.EncodeAll(w, image); err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		a.increaseServed()

		go a.storeGifInCache(id, caption, image)
	})
}

func (a App) getGifCacheFilename(id, caption string) string {
	return filepath.Join(a.tmpFolder, sha.New(fmt.Sprintf("%s:%s", id, caption))+".gif")
}

func (a App) storeGifInCache(id, caption string, image *gif.GIF) {
	if file, err := os.OpenFile(a.getGifCacheFilename(id, caption), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600); err != nil {
		logger.Error("unable to open gif to local cache: %s", err)
	} else if err := gif.EncodeAll(file, image); err != nil {
		logger.Error("unable to write gif to local cache: %s", err)
	}
}

func (a App) generateGif(ctx context.Context, from, caption string) (*gif.GIF, error) {
	image, err := getGif(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("unable to get gif: %s", err)
	}

	image, err = a.captionGif(ctx, image, caption)
	if err != nil {
		return nil, fmt.Errorf("unable to caption gif: %s", err)
	}

	return image, nil
}

func getGif(ctx context.Context, imageURL string) (*gif.GIF, error) {
	resp, err := request.Get(imageURL).Send(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch URL `%s`: %s", imageURL, err)
	}

	output, err := gif.DecodeAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to decode gif: %s", err)
	}

	return output, nil
}

func (a App) captionGif(ctx context.Context, source *gif.GIF, text string) (*gif.GIF, error) {
	if a.tracer != nil {
		var span trace.Span
		ctx, span = a.tracer.Start(ctx, "captionGif")
		defer span.End()
	}

	wg := concurrent.NewFailFast(8)

	for i, frame := range source.Image {
		func(i int, frame *image.Paletted) {
			wg.Go(func() error {
				img, err := a.captionImage(ctx, frame, text, true)
				bounds := frame.Bounds()
				draw.Draw(frame, bounds, img, bounds.Min, draw.Src)
				return err
			})
		}(i, frame)
	}

	if err := wg.Wait(); err != nil {
		return source, err
	}

	return source, nil
}
