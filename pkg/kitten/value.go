package kitten

import (
	"regexp"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	specialChars = regexp.MustCompile(`[^0-9a-zA-Z -](?m)`)

	transliterations = map[string]string{
		"Ð": "D",
		"Ł": "l",
		"Ø": "oe",
		"Þ": "Th",
		"ß": "ss",
		"æ": "ae",
		"ð": "d",
		"ł": "l",
		"ø": "oe",
		"þ": "th",
		"œ": "oe",
	}

	transformerPool = sync.Pool{
		New: func() any {
			return transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
		},
	}
)

func sanitizeValue(name string) (string, error) {
	withoutLigatures := strings.ToLower(name)
	for key, value := range transliterations {
		if strings.Contains(withoutLigatures, key) {
			withoutLigatures = strings.ReplaceAll(withoutLigatures, key, value)
		}
	}

	transformer := transformerPool.Get().(transform.Transformer)
	defer transformerPool.Put(transformer)

	withoutDiacritics, _, err := transform.String(transformer, withoutLigatures)
	if err != nil {
		return "", err
	}

	return specialChars.ReplaceAllString(withoutDiacritics, ""), nil
}
