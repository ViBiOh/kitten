package kitten

import (
	"strings"

	"github.com/ViBiOh/httputils/v4/pkg/logger"
)

func parseIdsOverrides(value string) map[string]string {
	if len(value) == 0 {
		return nil
	}

	output := make(map[string]string)

	for _, override := range strings.Split(value, "~") {
		parts := strings.SplitN(override, "|", 2)
		if len(parts) != 2 {
			logger.Error("unable to parse override `%s`", override)
			continue
		}

		output[parts[0]] = parts[1]
	}

	return output
}

func (a App) isOverride(id string) bool {
	_, ok := a.idsOverrides[id]
	return ok
}

func (a App) getOverride(id string) string {
	if url, ok := a.idsOverrides[id]; ok {
		return url
	}

	return ""
}
