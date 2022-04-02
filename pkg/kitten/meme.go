package kitten

import (
	"context"
	"fmt"
	"image"
	"io"
	"strings"
	"sync"

	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

const (
	fontSize    float64 = 64
	maxBodySize int64   = 2 << 20
)

var fontFacePool = sync.Pool{
	New: func() any {
		impactFace, err := gg.LoadFontFace("impact.ttf", fontSize)
		if err != nil {
			logger.Error("unable to load font face: %s", err)
		}

		return impactFace
	},
}

// GetFromUnsplash generates a meme from the given id with caption text
func (a App) GetFromUnsplash(ctx context.Context, id, caption string) (image.Image, error) {
	if a.tracer != nil {
		_, span := a.tracer.Start(ctx, "GetFromUnsplash")
		defer span.End()
	}

	unsplashImage, err := a.unsplashApp.GetImage(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to get image from unsplash: %s", err)
	}

	go a.unsplashApp.SendDownload(context.Background(), unsplashImage)

	return a.generateImage(ctx, unsplashImage.Raw, caption)
}

// GetFromURL a meme caption to the given image name from unsplash
func (a App) GetFromURL(ctx context.Context, imageURL, caption string) (image.Image, error) {
	if a.tracer != nil {
		_, span := a.tracer.Start(ctx, "GetFromURL")
		defer span.End()
	}

	return a.generateImage(ctx, imageURL, caption)
}

func (a App) generateImage(ctx context.Context, from, caption string) (image.Image, error) {
	image, err := getImage(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("unable to get image: %s", err)
	}

	image, err = a.captionImage(ctx, image, caption)
	if err != nil {
		return nil, fmt.Errorf("unable to caption image: %s", err)
	}

	return image, nil
}

func getImage(ctx context.Context, imageURL string) (image.Image, error) {
	resp, err := request.Get(imageURL).Send(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch URL `%s`: %s", imageURL, err)
	}

	output, _, err := image.Decode(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("unable to decode image, perhaps it exceeded the %d bytes length: %s", maxBodySize, err)
	}

	return output, nil
}

func (a App) captionImage(ctx context.Context, source image.Image, text string) (image.Image, error) {
	if a.tracer != nil {
		_, span := a.tracer.Start(ctx, "captionImage")
		defer span.End()
	}

	imageCtx := gg.NewContextForImage(source)

	fontFace := fontFacePool.Get().(font.Face)
	defer fontFacePool.Put(fontFace)

	imageCtx.SetFontFace(fontFace)

	imageCtx.SetRGB(1, 1, 1)
	lines := imageCtx.WordWrap(strings.ToUpper(text), float64(imageCtx.Width())*0.75)
	xAnchor := float64(imageCtx.Width() / 2)
	yAnchor := fontSize / 2

	n := float64(2)

	for _, lineString := range lines {
		yAnchor += fontSize

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
