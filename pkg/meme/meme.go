package meme

import (
	"context"
	"fmt"
	"image"
	"io"
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/kitten/pkg/unsplash"
	"github.com/fogleman/gg"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/image/font"
)

const (
	fontSize    float64 = 64
	maxBodySize int64   = 2 << 20
)

// App of package
type App struct {
	unsplashApp unsplash.App
	tracer      trace.Tracer
	fontFace    font.Face
	website     string
}

// New creates new App from Config
func New(unsplashApp unsplash.App, tracer trace.Tracer, website string) (App, error) {
	impactFace, err := gg.LoadFontFace("impact.ttf", fontSize)
	if err != nil {
		return App{}, fmt.Errorf("unable to load font face: %s", err)
	}

	return App{
		unsplashApp: unsplashApp,
		tracer:      tracer,
		fontFace:    impactFace,
		website:     website,
	}, nil
}

// GetFromUnsplash a meme caption to the given image name from unsplash
func (a App) GetFromUnsplash(ctx context.Context, id, name, caption string) (output image.Image, unsplashImage unsplash.Image, err error) {
	if a.tracer != nil {
		_, span := a.tracer.Start(ctx, "GetFromUnsplash")
		defer span.End()
	}

	if len(id) != 0 {
		unsplashImage, err = a.unsplashApp.GetImage(ctx, id)
	} else {
		unsplashImage, err = a.unsplashApp.GetRandomImage(ctx, name)
	}

	if err != nil {
		return nil, unsplashImage, fmt.Errorf("unable to get image from unsplash: %s", err)
	}

	output, err = getImage(ctx, unsplashImage.Raw)
	if err != nil {
		return nil, unsplashImage, fmt.Errorf("unable to get image: %s", err)
	}

	go a.unsplashApp.SendDownload(context.Background(), unsplashImage)

	output, err = a.captionImage(ctx, output, caption)
	if err != nil {
		return nil, unsplashImage, fmt.Errorf("unable to caption image: %s", err)
	}

	return
}

// GetFromURL a meme caption to the given image name from unsplash
func (a App) GetFromURL(ctx context.Context, imageURL, caption string) (image.Image, error) {
	if a.tracer != nil {
		_, span := a.tracer.Start(ctx, "GetFromURL")
		defer span.End()
	}

	image, err := getImage(ctx, imageURL)
	if err != nil {
		return nil, fmt.Errorf("unable to get image from url: %s", err)
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
	imageCtx.SetFontFace(a.fontFace)

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
