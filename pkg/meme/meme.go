package meme

import (
	"context"
	"fmt"
	"image"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/kitten/pkg/unsplash"
	"github.com/fogleman/gg"
)

const fontSize float64 = 64

// App of package
type App struct {
	unsplashApp unsplash.App
	website     string
}

// New creates new App from Config
func New(unsplashApp unsplash.App, website string) App {
	return App{
		unsplashApp: unsplashApp,
		website:     website,
	}
}

// GetFromUnsplash a meme caption to the given image name from unsplash
func (a App) GetFromUnsplash(ctx context.Context, id, name, caption string) (output image.Image, unsplashImage unsplash.Image, err error) {
	if len(id) != 0 {
		unsplashImage, err = a.unsplashApp.GetImage(ctx, id)
	} else {
		unsplashImage, err = a.unsplashApp.GetRandomImage(ctx, name)
	}

	if err != nil {
		return nil, unsplashImage, fmt.Errorf("unable to get image from unsplash: %s", err)
	}

	output, err = getImage(ctx, unsplashImage.URL)
	if err != nil {
		return nil, unsplashImage, fmt.Errorf("unable to get image: %s", err)
	}

	output, err = captionImage(output, caption, fontSize)
	if err != nil {
		return nil, unsplashImage, fmt.Errorf("unable to caption image: %s", err)
	}

	return
}

// GetFromURL a meme caption to the given image name from unsplash
func (a App) GetFromURL(ctx context.Context, imageURL, caption string) (image.Image, error) {
	image, err := getImage(ctx, imageURL)
	if err != nil {
		return nil, fmt.Errorf("unable to get image: %s", err)
	}

	image, err = captionImage(image, caption, fontSize)
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

	output, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to decode image: %s", err)
	}

	return output, nil
}

func captionImage(source image.Image, text string, fontSize float64) (image.Image, error) {
	imageCtx := gg.NewContextForImage(source)
	if err := imageCtx.LoadFontFace("impact.ttf", fontSize); err != nil {
		return nil, fmt.Errorf("unable to load font: %s", err)
	}

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
