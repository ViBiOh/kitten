package version

import (
	"fmt"

	"github.com/ViBiOh/httputils/v4/pkg/sha"
)

var (
	CacheVersion = sha.New("vibioh/kitten/1")[:8]
	CachePrefix  = "kitten:" + CacheVersion
)

func Redis(content string) string {
	return fmt.Sprintf("%s:%s", CachePrefix, content)
}
