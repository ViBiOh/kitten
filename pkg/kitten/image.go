package kitten

import (
	"context"
	"fmt"
	"image"
	"io"

	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/go-oss/image/imageutil"
)

func (a App) generateImage(ctx context.Context, from, caption string) (image.Image, error) {
	image, err := getImage(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("get image: %s", err)
	}

	image, err = a.CaptionImage(ctx, image, caption)
	if err != nil {
		return nil, fmt.Errorf("caption image: %s", err)
	}

	return image, nil
}

func getImage(ctx context.Context, imageURL string) (image.Image, error) {
	resp, err := request.Get(imageURL).Send(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch URL `%s`: %s", imageURL, err)
	}

	reader, err := imageutil.RemoveExif(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("remove exif from image, perhaps it exceeded the %d bytes length: %s", maxBodySize, err)
	}

	output, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("decode image: %s", err)
	}

	return output, nil
}
