package kitten

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/sha"
)

func (a App) serveCached(w http.ResponseWriter, id, caption string) bool {
	file, err := os.OpenFile(a.getCacheFilename(id, caption), os.O_RDONLY, 0o600)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Error("unable to open image from local cache: %s", err)
		}

		return false
	}

	buffer := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buffer)

	w.Header().Add("Cache-Control", cacheControlDuration)
	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)

	if _, err = io.CopyBuffer(w, file, buffer.Bytes()); err != nil {
		logger.Error("unable to write image from local cache: %s", err)
		return false
	}

	a.increaseCached()

	return true
}

func (a App) storeInCache(id, caption string, image image.Image) {
	if file, err := os.OpenFile(a.getCacheFilename(id, caption), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600); err != nil {
		logger.Error("unable to open image to local cache: %s", err)
	} else if err := jpeg.Encode(file, image, &jpeg.Options{Quality: 80}); err != nil {
		logger.Error("unable to write image to local cache: %s", err)
	}
}

func (a App) getCacheFilename(id, caption string) string {
	return filepath.Join(a.tmpFolder, sha.New(fmt.Sprintf("%s:%s", id, caption))+".jpeg")
}

func (a App) generateAndStoreImage(ctx context.Context, id, from, caption string) (string, int64, error) {
	imagePath := a.getCacheFilename(id, caption)

	info, err := os.Stat(imagePath)
	if err != nil && !os.IsNotExist(err) {
		return "", 0, err
	}

	if info == nil {
		image, err := a.generateImage(ctx, from, caption)
		if err != nil {
			return "", 0, fmt.Errorf("unable to generate image: %s", err)
		}

		a.storeInCache(id, caption, image)

		info, err = os.Stat(imagePath)
		if err != nil {
			return "", 0, fmt.Errorf("unable to get image info: %s", err)
		}
	}

	return imagePath, info.Size(), nil
}
