package kitten

import (
	"context"
	"embed"
	"fmt"
	"image"
	"image/gif"
	"strings"
	"sync"

	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/tracer"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

//go:embed fonts
var content embed.FS

const (
	fontSizeCoeff float64 = 0.07
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
					logger.Error("unable to load font face: %s", err)
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
func (a App) GetFromUnsplash(ctx context.Context, id, caption string) (image.Image, error) {
	ctx, end := tracer.StartSpan(ctx, a.tracer, "GetFromUnsplash")
	defer end()

	unsplashImage, err := a.unsplashApp.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to get image from unsplash: %s", err)
	}

	go a.unsplashApp.SendDownload(context.Background(), unsplashImage)

	return a.generateImage(ctx, unsplashImage.Raw, caption)
}

// GetFromGiphy generates a meme from the given id with caption text
func (a App) GetFromGiphy(ctx context.Context, id, caption string) (*gif.GIF, error) {
	ctx, end := tracer.StartSpan(ctx, a.tracer, "GetFromGiphy")
	defer end()

	giphyImage, err := a.giphyApp.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to get image from giphy: %s", err)
	}

	go a.giphyApp.SendAnalytics(context.Background(), giphyImage)

	return a.generateGif(ctx, giphyImage.Images["downsized"].URL, caption)
}

// GetGifFromURL generates a meme gif from the given id with caption text
func (a App) GetGifFromURL(ctx context.Context, imageURL, caption string) (*gif.GIF, error) {
	ctx, end := tracer.StartSpan(ctx, a.tracer, "GetGifFromURL")
	defer end()

	return a.generateGif(ctx, imageURL, caption)
}

// GetFromURL a meme caption to the given image name from url
func (a App) GetFromURL(ctx context.Context, imageURL, caption string) (image.Image, error) {
	ctx, end := tracer.StartSpan(ctx, a.tracer, "GetFromURL")
	defer end()

	return a.generateImage(ctx, imageURL, caption)
}

// CaptionImage add caption on an image
func (a App) CaptionImage(ctx context.Context, source image.Image, text string) (image.Image, error) {
	_, end := tracer.StartSpan(ctx, a.tracer, "captionImage")
	defer end()

	imageCtx := gg.NewContextForImage(source)

	fontSize := float64(source.Bounds().Dx()) * fontSizeCoeff
	fontFace, resolve := getFontFace(fontSize)
	defer resolve()

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
