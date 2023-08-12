package version

import (
	"fmt"

	"github.com/ViBiOh/httputils/v4/pkg/hash"
)

var (
	CacheVersion = hash.String("vibioh/kitten/1")[:8]
	CachePrefix  = "kitten:" + CacheVersion
)

func Redis(content string) string {
	return fmt.Sprintf("%s:%s", CachePrefix, content)
}
