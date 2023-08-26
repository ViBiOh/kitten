package kitten

import (
	"context"
	"embed"
	"fmt"
	"image"
	"image/gif"
	"log/slog"
	"sync"

	"github.com/ViBiOh/httputils/v4/pkg/cntxt"
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
					slog.Error("load font face", "err", err)
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
func (a Service) GetFromUnsplash(ctx context.Context, id, caption string) (image.Image, error) {
	var err error

	ctx, end := telemetry.StartSpan(ctx, a.tracer, "GetFromUnsplash")
	defer end(&err)

	unsplashImage, err := a.unsplashService.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get image from unsplash: %w", err)
	}

	go a.unsplashService.SendDownload(cntxt.WithoutDeadline(ctx), unsplashImage)

	return a.generateImage(ctx, unsplashImage.Raw, caption)
}

// GetGif generates a meme from the given id with caption text
func (a Service) GetGif(ctx context.Context, id, search, caption string) (*gif.GIF, error) {
	var err error

	ctx, end := telemetry.StartSpan(ctx, a.tracer, "GetGif")
	defer end(&err)

	gifContent, err := a.tenorService.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get from tenor: %w", err)
	}

	go a.tenorService.SendAnalytics(cntxt.WithoutDeadline(ctx), gifContent, search)

	return a.generateGif(ctx, gifContent.GetImageURL(), caption)
}

// GetGifFromURL generates a meme gif from the given id with caption text
func (a Service) GetGifFromURL(ctx context.Context, imageURL, caption string) (img *gif.GIF, err error) {
	ctx, end := telemetry.StartSpan(ctx, a.tracer, "GetGifFromURL")
	defer end(&err)

	return a.generateGif(ctx, imageURL, caption)
}

// GetFromURL a meme caption to the given image name from url
func (a Service) GetFromURL(ctx context.Context, imageURL, caption string) (img image.Image, err error) {
	ctx, end := telemetry.StartSpan(ctx, a.tracer, "GetFromURL")
	defer end(&err)

	return a.generateImage(ctx, imageURL, caption)
}

// CaptionImage add caption on an image
func (a Service) CaptionImage(ctx context.Context, source image.Image, text string) (img image.Image, err error) {
	_, end := telemetry.StartSpan(ctx, a.tracer, "captionImage")
	defer end(&err)

	return a.caption(gg.NewContextForImage(source), text)
}
