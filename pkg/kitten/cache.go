package kitten

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/sha"
)

func (a App) serveCached(w http.ResponseWriter, id, caption string) bool {
	file, err := os.OpenFile(filepath.Join(a.tmpFolder, getRequestHash(id, caption)+".png"), os.O_RDONLY, 0o600)
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
	file, err := os.OpenFile(filepath.Join(a.tmpFolder, getRequestHash(id, caption)+".jpeg"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		logger.Error("unable to open image to local cache: %s", err)
		return
	}

	if err := png.Encode(file, image); err != nil {
		logger.Error("unable to write image to local cache: %s", err)
	}
}

func getRequestHash(id, caption string) string {
	return sha.New(fmt.Sprintf("%s:%s", id, caption))
}