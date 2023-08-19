package kitten

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ViBiOh/httputils/v4/pkg/hash"
)

func (a App) serveCached(ctx context.Context, w http.ResponseWriter, id, caption string, gif bool) bool {
	var filename string
	if gif {
		filename = a.getGifCacheFilename(id, caption)
	} else {
		filename = a.getCacheFilename(id, caption)
	}

	file, err := os.OpenFile(filename, os.O_RDONLY, 0o600)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Error("open file from local cache", "err", err)
		}

		return false
	}

	buffer := bufferPool.Get().(*bytes.Buffer)
	defer bufferPool.Put(buffer)

	w.Header().Add("Cache-Control", cacheControlDuration)
	if gif {
		w.Header().Set("Content-Type", "image/gif")
	} else {
		w.Header().Set("Content-Type", "image/jpeg")
	}
	w.WriteHeader(http.StatusOK)

	if _, err = io.CopyBuffer(w, file, buffer.Bytes()); err != nil {
		slog.Error("write file from local cache", "err", err)
		return false
	}

	a.increaseCached(ctx)

	return true
}

func (a App) storeInCache(id, caption string, image image.Image) {
	if file, err := os.OpenFile(a.getCacheFilename(id, caption), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600); err != nil {
		slog.Error("open image to local cache", "err", err)
	} else if err := jpeg.Encode(file, image, &jpeg.Options{Quality: 80}); err != nil {
		slog.Error("write image to local cache", "err", err)
	}
}

func (a App) getCacheFilename(id, caption string) string {
	return filepath.Join(a.tmpFolder, hash.String(fmt.Sprintf("%s:%s", id, caption))+".jpeg")
}

func (a App) generateAndStoreImage(ctx context.Context, id, from, caption string) (string, int64, error) {
	imagePath := a.getCacheFilename(id, caption)

	info, err := os.Stat(imagePath)
	if err != nil && !os.IsNotExist(err) {
		return "", 0, err
	}

	if info == nil {
		imageOutput, err := a.generateImage(ctx, from, caption)
		if err != nil {
			return "", 0, fmt.Errorf("generate imageOutput: %w", err)
		}

		a.storeInCache(id, caption, imageOutput)

		info, err = os.Stat(imagePath)
		if err != nil {
			return "", 0, fmt.Errorf("get imageOutput info: %w", err)
		}
	}

	return imagePath, info.Size(), nil
}
