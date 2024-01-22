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

func (s Service) serveCached(ctx context.Context, w http.ResponseWriter, id, caption string, gif bool) bool {
	var filename string
	if gif {
		filename = s.getGifCacheFilename(id, caption)
	} else {
		filename = s.getCacheFilename(id, caption)
	}

	file, err := os.OpenFile(filename, os.O_RDONLY, 0o600)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.LogAttrs(ctx, slog.LevelError, "open file from local cache", slog.Any("error", err))
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
		slog.LogAttrs(ctx, slog.LevelError, "write file from local cache", slog.Any("error", err))
		return false
	}

	s.increaseCached(ctx)

	return true
}

func (s Service) storeInCache(ctx context.Context, id, caption string, image image.Image) {
	if file, err := os.OpenFile(s.getCacheFilename(id, caption), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600); err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "open image to local cache", slog.Any("error", err))
	} else if err := jpeg.Encode(file, image, &jpeg.Options{Quality: 80}); err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "write image to local cache", slog.Any("error", err))
	}
}

func (s Service) getCacheFilename(id, caption string) string {
	return filepath.Join(s.tmpFolder, hash.String(fmt.Sprintf("%s:%s", id, caption))+".jpeg")
}

func (s Service) generateAndStoreImage(ctx context.Context, id, from, caption string) (string, int64, error) {
	imagePath := s.getCacheFilename(id, caption)

	info, err := os.Stat(imagePath)
	if err != nil && !os.IsNotExist(err) {
		return "", 0, err
	}

	if info == nil {
		imageOutput, err := s.generateImage(ctx, from, caption)
		if err != nil {
			return "", 0, fmt.Errorf("generate imageOutput: %w", err)
		}

		s.storeInCache(ctx, id, caption, imageOutput)

		info, err = os.Stat(imagePath)
		if err != nil {
			return "", 0, fmt.Errorf("get imageOutput info: %w", err)
		}
	}

	return imagePath, info.Size(), nil
}
