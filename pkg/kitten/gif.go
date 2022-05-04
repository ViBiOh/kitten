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
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/concurrent"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/httputils/v4/pkg/sha"
	"github.com/fogleman/gg"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/image/font"
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

func (a App) generateAndStoreGif(ctx context.Context, id, from, caption string) (string, int64, error) {
	imagePath := a.getGifCacheFilename(id, caption)

	info, err := os.Stat(imagePath)
	if err != nil && !os.IsNotExist(err) {
		return "", 0, err
	}

	if info == nil {
		image, err := a.generateGif(ctx, from, caption)
		if err != nil {
			return "", 0, fmt.Errorf("unable to generate image: %s", err)
		}

		a.storeGifInCache(id, caption, image)

		info, err = os.Stat(imagePath)
		if err != nil {
			return "", 0, fmt.Errorf("unable to get image info: %s", err)
		}
	}

	return imagePath, info.Size(), nil
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

	textImage, err := a.captionAlpha(ctx, source.Config.Width, source.Config.Height, text)
	if err != nil {
		return source, fmt.Errorf("unable to generate text layer: %s", err)
	}
	textImageBounds := textImage.Bounds()

	for _, frame := range source.Image {
		maskedFrame := frame
		wg.Go(func() error {
			draw.DrawMask(maskedFrame, textImageBounds, textImage, textImageBounds.Min, textImage, textImageBounds.Min, draw.Over)
			return err
		})
	}

	if err := wg.Wait(); err != nil {
		return source, err
	}

	return source, nil
}

func (a App) captionAlpha(ctx context.Context, width, height int, text string) (image.Image, error) {
	if a.tracer != nil {
		_, span := a.tracer.Start(ctx, "captionImage")
		defer span.End()
	}

	imageCtx := gg.NewContext(width, height)

	fontFace := gifFontFacePool.Get().(font.Face)
	defer gifFontFacePool.Put(fontFace)

	imageCtx.SetFontFace(fontFace)

	lines := imageCtx.WordWrap(strings.ToUpper(text), float64(imageCtx.Width())*0.75)
	xAnchor := float64(imageCtx.Width() / 2)
	yAnchor := gifFontSize / 2

	n := float64(2)

	for _, lineString := range lines {
		yAnchor += gifFontSize

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
