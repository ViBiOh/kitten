package meme

import (
	"fmt"
	"image"
	"strings"

	"github.com/fogleman/gg"
)

const fontSize = 64

// CaptionImage add a meme caption to the given image
func CaptionImage(source image.Image, text string) (image.Image, error) {
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
