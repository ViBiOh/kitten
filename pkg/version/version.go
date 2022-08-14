package version

import (
	"fmt"

	"github.com/ViBiOh/httputils/v4/pkg/sha"
)

var cacheVersion = sha.New("vibioh/kitten/1")[:8]

func Redis(content string) string {
	return fmt.Sprintf("kitten:%s:%s", cacheVersion, content)
}
