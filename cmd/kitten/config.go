package main

import (
	"flag"
	"os"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/alcotest"
	"github.com/ViBiOh/httputils/v4/pkg/cors"
	"github.com/ViBiOh/httputils/v4/pkg/health"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/owasp"
	"github.com/ViBiOh/httputils/v4/pkg/pprof"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/httputils/v4/pkg/server"
	"github.com/ViBiOh/httputils/v4/pkg/telemetry"
	"github.com/ViBiOh/kitten/pkg/kitten"
	"github.com/ViBiOh/kitten/pkg/tenor"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

type configuration struct {
	logger    *logger.Config
	alcotest  *alcotest.Config
	telemetry *telemetry.Config
	pprof     *pprof.Config
	health    *health.Config

	server   *server.Config
	owasp    *owasp.Config
	cors     *cors.Config
	renderer *renderer.Config

	redis *redis.Config

	kitten   *kitten.Config
	unsplash *unsplash.Config
	tenor    *tenor.Config
	slack    *slack.Config
	discord  *discord.Config
}

func newConfig() configuration {
	fs := flag.NewFlagSet("kitten", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	config := configuration{
		logger:    logger.Flags(fs, "logger"),
		alcotest:  alcotest.Flags(fs, ""),
		telemetry: telemetry.Flags(fs, "telemetry"),
		pprof:     pprof.Flags(fs, "pprof"),
		health:    health.Flags(fs, ""),

		server:   server.Flags(fs, ""),
		owasp:    owasp.Flags(fs, "", flags.NewOverride("Csp", "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com")),
		cors:     cors.Flags(fs, "cors"),
		renderer: renderer.Flags(fs, "", flags.NewOverride("Title", "KittenBot"), flags.NewOverride("PublicURL", "https://kitten.vibioh.fr")),

		redis: redis.Flags(fs, "redis"),

		kitten:   kitten.Flags(fs, ""),
		unsplash: unsplash.Flags(fs, "unsplash"),
		tenor:    tenor.Flags(fs, "tenor"),
		slack:    slack.Flags(fs, "slack"),
		discord:  discord.Flags(fs, "discord"),
	}

	_ = fs.Parse(os.Args[1:])

	return config
}
