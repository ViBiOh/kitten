package meme

import (
	"context"
	"fmt"
	"image"
	"strings"

	"github.com/ViBiOh/kitten/pkg/unsplash"
	"github.com/fogleman/gg"
)

const fontSize = 64

// App of package
type App struct {
	unsplashApp unsplash.App
}

// New creates new App from Config
func New(unsplashApp unsplash.App) App {
	return App{
		unsplashApp: unsplashApp,
	}
}

// Get  a meme caption to the given image name
func (a App) Get(ctx context.Context, name, caption string) (image.Image, string, string, error) {
	image, credits, id, err := a.unsplashApp.GetRandomImage(ctx, name)
	if err != nil {
		return nil, credits, id, fmt.Errorf("unable to get image from unsplash: %s", err)
	}

	image, err = captionImage(image, caption)
	if err != nil {
		return nil, credits, id, fmt.Errorf("unable to caption image: %s", err)
	}

	return image, credits, id, nil
}

func captionImage(source image.Image, text string) (image.Image, error) {
	imageCtx := gg.NewContextForImage(source)
	if err := imageCtx.LoadFontFace("impact.ttf", fontSize); err != nil {
		return nil, fmt.Errorf("unable to load font: %s", err)
	}

	imageCtx.SetRGB(1, 1, 1)
	lines := imageCtx.WordWrap(strings.ToUpper(text), float64(imageCtx.Width())*0.75)
	xAnchor := float64(imageCtx.Width() / 2)
	yAnchor := float64(fontSize) / 2

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
