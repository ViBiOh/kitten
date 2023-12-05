package kitten

import (
	"context"
	"fmt"
	"image"
	"io"

	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/go-oss/image/imageutil"
)

func (s Service) generateImage(ctx context.Context, from, caption string) (image.Image, error) {
	imageOutput, err := getImage(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("get imageOutput: %w", err)
	}

	imageOutput, err = s.CaptionImage(ctx, imageOutput, caption)
	if err != nil {
		return nil, fmt.Errorf("caption imageOutput: %w", err)
	}

	return imageOutput, nil
}

func getImage(ctx context.Context, imageURL string) (image.Image, error) {
	resp, err := request.Get(imageURL).Send(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch URL `%s`: %w", imageURL, err)
	}

	reader, err := imageutil.RemoveExif(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("remove exif from image, perhaps it exceeded the %d bytes length: %w", maxBodySize, err)
	}

	output, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	return output, nil
}
