package kitten

import (
	"context"
	"embed"
	"fmt"
	"image"
	"image/gif"
	"log/slog"
	"net/http"
	"sync"

	"github.com/ViBiOh/httputils/v4/pkg/cntxt"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/telemetry"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

//go:embed fonts
var content embed.FS

const (
	fontSizeCoeff float64 = 0.07
	widthPadding  float64 = 0.8
	maxBodySize   int64   = 2 << 20
)

var fontFacesSizes = map[float64]*sync.Pool{}

func loadFsFont(fontName string, points float64) (font.Face, error) {
	fontBytes, err := content.ReadFile(fontName)
	if err != nil {
		return nil, err
	}

	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	face := truetype.NewFace(f, &truetype.Options{
		Size: points,
	})
	return face, nil
}

func getFontFace(size float64) (font.Face, func()) {
	pool, ok := fontFacesSizes[size]
	if !ok {
		pool = &sync.Pool{
			New: func() any {
				impactFace, err := loadFsFont("fonts/impact.ttf", size)
				if err != nil {
					slog.Error("load font face", "error", err)
				}

				return impactFace
			},
		}

		fontFacesSizes[size] = pool
	}

	fontFace := pool.Get().(font.Face)
	return fontFace, func() { pool.Put(fontFace) }
}

// GetFromUnsplash generates a meme from the given id with caption text
func (s Service) GetFromUnsplash(ctx context.Context, w http.ResponseWriter, id, caption string) {
	var err error

	ctx, end := telemetry.StartSpan(ctx, s.tracer, "GetFromUnsplash")
	defer end(&err)

	unsplashImage, err := s.unsplashService.Get(ctx, id)
	if err != nil {
		httperror.InternalServerError(ctx, w, fmt.Errorf("get image: %s", err))
		return
	}

	go s.unsplashService.SendDownload(cntxt.WithoutDeadline(ctx), unsplashImage)

	s.serveImage(ctx, w, unsplashImage, caption)
}

// GetGif generates a meme from the given id with caption text
func (s Service) GetGif(ctx context.Context, id, search, caption string) (*gif.GIF, error) {
	var err error

	ctx, end := telemetry.StartSpan(ctx, s.tracer, "GetGif")
	defer end(&err)

	gifContent, err := s.tenorService.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get from tenor: %w", err)
	}

	go s.tenorService.SendAnalytics(cntxt.WithoutDeadline(ctx), gifContent, search)

	return s.generateGif(ctx, gifContent.GetImageURL(), caption)
}

// GetGifFromURL generates a meme gif from the given id with caption text
func (s Service) GetGifFromURL(ctx context.Context, imageURL, caption string) (img *gif.GIF, err error) {
	ctx, end := telemetry.StartSpan(ctx, s.tracer, "GetGifFromURL")
	defer end(&err)

	return s.generateGif(ctx, imageURL, caption)
}

// GetFromURL a meme caption to the given image name from url
func (s Service) GetFromURL(ctx context.Context, imageURL, caption string) (img image.Image, err error) {
	ctx, end := telemetry.StartSpan(ctx, s.tracer, "GetFromURL")
	defer end(&err)

	return s.generateImage(ctx, imageURL, caption)
}

// CaptionImage add caption on an image
func (s Service) CaptionImage(ctx context.Context, source image.Image, text string) (img image.Image, err error) {
	_, end := telemetry.StartSpan(ctx, s.tracer, "captionImage")
	defer end(&err)

	return s.caption(gg.NewContextForImage(source), text)
}
