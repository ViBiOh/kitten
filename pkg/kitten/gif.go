package kitten

import (
	"context"
	"errors"
	"fmt"
	"image/draw"
	"image/gif"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ViBiOh/httputils/v4/pkg/concurrent"
	"github.com/ViBiOh/httputils/v4/pkg/hash"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/httputils/v4/pkg/telemetry"
	"github.com/ViBiOh/kitten/pkg/tenor"
	"github.com/fogleman/gg"
)

func (a App) GifHandler() http.Handler {
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

		id, search, caption, err := parseRequest(query)
		if err != nil {
			httperror.BadRequest(w, err)
			return
		}

		if a.serveCached(r.Context(), w, id, caption, true) {
			return
		}

		image, err := a.GetGif(r.Context(), id, search, caption)
		if err != nil {
			if errors.Is(err, tenor.ErrNotFound) {
				httperror.NotFound(w)
			} else {
				httperror.InternalServerError(w, err)
			}

			return
		}

		w.Header().Add("Cache-Control", cacheControlDuration)
		w.Header().Set("Content-Type", "image/gif")
		w.WriteHeader(http.StatusOK)

		if err = gif.EncodeAll(w, image); err != nil {
			httperror.InternalServerError(w, err)
			return
		}

		a.increaseServed(r.Context())

		go a.storeGifInCache(id, caption, image)
	})
}

func (a App) getGifCacheFilename(id, caption string) string {
	return filepath.Join(a.tmpFolder, hash.String(fmt.Sprintf("%s:%s", id, caption))+".gif")
}

func (a App) storeGifInCache(id, caption string, image *gif.GIF) {
	if file, err := os.OpenFile(a.getGifCacheFilename(id, caption), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600); err != nil {
		slog.Error("open gif to local cache", "err", err)
	} else if err := gif.EncodeAll(file, image); err != nil {
		slog.Error("write gif to local cache", "err", err)
	}
}

func (a App) generateGif(ctx context.Context, from, caption string) (*gif.GIF, error) {
	image, err := getGif(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("get gif: %w", err)
	}

	image, err = a.CaptionGif(ctx, image, caption)
	if err != nil {
		return nil, fmt.Errorf("caption gif: %w", err)
	}

	return image, nil
}

func (a App) generateAndStoreGif(ctx context.Context, id, from, caption string) (string, int64, error) {
	imagePath := a.getGifCacheFilename(id, caption)

	info, err := os.Stat(imagePath)
	if err != nil && !os.IsNotExist(err) {
		return "", 0, err
	}

	if info == nil {
		image, err := a.generateGif(ctx, from, caption)
		if err != nil {
			return "", 0, fmt.Errorf("generate image: %w", err)
		}

		a.storeGifInCache(id, caption, image)

		info, err = os.Stat(imagePath)
		if err != nil {
			return "", 0, fmt.Errorf("get image info: %w", err)
		}
	}

	return imagePath, info.Size(), nil
}

func getGif(ctx context.Context, imageURL string) (*gif.GIF, error) {
	resp, err := request.Get(imageURL).Send(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch URL `%s`: %w", imageURL, err)
	}

	output, err := gif.DecodeAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decode gif: %w", err)
	}

	return output, nil
}

func (a App) CaptionGif(ctx context.Context, source *gif.GIF, text string) (*gif.GIF, error) {
	var err error

	_, end := telemetry.StartSpan(ctx, a.tracer, "captionGif")
	defer end(&err)

	wg := concurrent.NewFailFast(8)

	textImage, err := a.caption(gg.NewContext(source.Config.Width, source.Config.Height), text)
	if err != nil {
		return source, fmt.Errorf("generate text layer: %w", err)
	}
	textImageBounds := textImage.Bounds()

	for _, frame := range source.Image {
		maskedFrame := frame
		wg.Go(func() error {
			draw.DrawMask(maskedFrame, textImageBounds, textImage, textImageBounds.Min, textImage, textImageBounds.Min, draw.Over)
			return err
		})
	}

	if err = wg.Wait(); err != nil {
		return source, err
	}

	return source, nil
}
