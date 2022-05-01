package kitten

import (
	"context"
	"fmt"
	"image"
	"image/gif"
	"strings"
	"sync"

	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/fogleman/gg"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/image/font"
)

const (
	fontSize    float64 = 64
	gifFontSize float64 = 32
	maxBodySize int64   = 2 << 20
)

var (
	fontFacePool = sync.Pool{
		New: func() any {
			impactFace, err := gg.LoadFontFace("impact.ttf", fontSize)
			if err != nil {
				logger.Error("unable to load font face: %s", err)
			}

			return impactFace
		},
	}

	gifFontFacePool = sync.Pool{
		New: func() any {
			impactFace, err := gg.LoadFontFace("impact.ttf", gifFontSize)
			if err != nil {
				logger.Error("unable to load font face: %s", err)
			}

			return impactFace
		},
	}
)

// GetFromUnsplash generates a meme from the given id with caption text
func (a App) GetFromUnsplash(ctx context.Context, id, caption string) (image.Image, error) {
	if a.tracer != nil {
		var span trace.Span
		ctx, span = a.tracer.Start(ctx, "GetFromUnsplash")
		defer span.End()
	}

	unsplashImage, err := a.unsplashApp.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to get image from unsplash: %s", err)
	}

	go a.unsplashApp.SendDownload(context.Background(), unsplashImage)

	return a.generateImage(ctx, unsplashImage.Raw, caption)
}

// GetFromGiphy generates a meme from the given id with caption text
func (a App) GetFromGiphy(ctx context.Context, id, caption string) (*gif.GIF, error) {
	if a.tracer != nil {
		var span trace.Span
		ctx, span = a.tracer.Start(ctx, "GetFromGiphy")
		defer span.End()
	}

	giphyImage, err := a.giphyApp.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to get image from unsplash: %s", err)
	}

	go a.giphyApp.SendAnalytics(context.Background(), giphyImage)

	return a.generateGif(ctx, giphyImage.Images["downsized"].URL, caption)
}

// GetFromURL a meme caption to the given image name from unsplash
func (a App) GetFromURL(ctx context.Context, imageURL, caption string) (image.Image, error) {
	if a.tracer != nil {
		_, span := a.tracer.Start(ctx, "GetFromURL")
		defer span.End()
	}

	return a.generateImage(ctx, imageURL, caption)
}

func (a App) captionImage(ctx context.Context, source image.Image, text string, gif bool) (image.Image, error) {
	if a.tracer != nil {
		_, span := a.tracer.Start(ctx, "captionImage")
		defer span.End()
	}

	imageCtx := gg.NewContextForImage(source)

	var captionFontSize float64
	var fontFace font.Face

	if gif {
		fontFace = gifFontFacePool.Get().(font.Face)
		defer gifFontFacePool.Put(fontFace)
		captionFontSize = gifFontSize
	} else {
		fontFace = fontFacePool.Get().(font.Face)
		defer fontFacePool.Put(fontFace)
		captionFontSize = fontSize
	}

	imageCtx.SetFontFace(fontFace)

	imageCtx.SetRGB(1, 1, 1)
	lines := imageCtx.WordWrap(strings.ToUpper(text), float64(imageCtx.Width())*0.75)
	xAnchor := float64(imageCtx.Width() / 2)
	yAnchor := captionFontSize / 2

	n := float64(2)

	for _, lineString := range lines {
		yAnchor += captionFontSize

		imageCtx.SetRGB(0, 0, 0)
		for dy := -n; dy <= n; dy++ {
			for dx := -n; dx <= n; dx++ {
				imageCtx.DrawStringAnchored(lineString, xAnchor+dx, yAnchor+dy, 0.5, 0.5)
			}
		}

		imageCtx.SetRGB(1, 1, 1)
		imageCtx.DrawStringAnchored(lineString, xAnchor, yAnchor, 0.5, 0.5)
	}

	return imageCtx.Image(), nil
}
