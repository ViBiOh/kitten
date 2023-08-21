package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	_ "net/http/pprof"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/alcotest"
	"github.com/ViBiOh/httputils/v4/pkg/cors"
	"github.com/ViBiOh/httputils/v4/pkg/health"
	"github.com/ViBiOh/httputils/v4/pkg/httputils"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/owasp"
	"github.com/ViBiOh/httputils/v4/pkg/recoverer"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/httputils/v4/pkg/server"
	"github.com/ViBiOh/httputils/v4/pkg/telemetry"
	"github.com/ViBiOh/kitten/pkg/kitten"
	"github.com/ViBiOh/kitten/pkg/tenor"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

//go:embed templates static
var content embed.FS

const (
	apiPrefix     = "/api"
	gifPrefix     = "/gif"
	slackPrefix   = "/slack"
	discordPrefix = "/discord"
)

func main() {
	fs := flag.NewFlagSet("kitten", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	appServerConfig := server.Flags(fs, "")
	healthConfig := health.Flags(fs, "")

	alcotestConfig := alcotest.Flags(fs, "")
	loggerConfig := logger.Flags(fs, "logger")
	telemetryConfig := telemetry.Flags(fs, "telemetry")
	owaspConfig := owasp.Flags(fs, "", flags.NewOverride("Csp", "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com"))
	corsConfig := cors.Flags(fs, "cors")

	redisConfig := redis.Flags(fs, "redis")

	kittenConfig := kitten.Flags(fs, "")
	unsplashConfig := unsplash.Flags(fs, "unsplash")
	tenorConfig := tenor.Flags(fs, "tenor")
	slackConfig := slack.Flags(fs, "slack")
	discordConfig := discord.Flags(fs, "discord")
	rendererConfig := renderer.Flags(fs, "", flags.NewOverride("Title", "KittenBot"), flags.NewOverride("PublicURL", "https://kitten.vibioh.fr"))

	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	alcotest.DoAndExit(alcotestConfig)

	logger.Init(loggerConfig)

	ctx := context.Background()

	telemetryApp, err := telemetry.New(ctx, telemetryConfig)
	if err != nil {
		slog.Error("create telemetry", "err", err)
		os.Exit(1)
	}

	defer telemetryApp.Close(ctx)
	request.AddOpenTelemetryToDefaultClient(telemetryApp.MeterProvider(), telemetryApp.TracerProvider())

	go func() {
		fmt.Println(http.ListenAndServe("localhost:9999", http.DefaultServeMux))
	}()

	appServer := server.New(appServerConfig)
	healthApp := health.New(healthConfig)

	rendererApp, err := renderer.New(rendererConfig, content, template.FuncMap{}, telemetryApp.MeterProvider(), telemetryApp.TracerProvider())
	if err != nil {
		slog.Error("create renderer", "err", err)
		os.Exit(1)
	}

	kittenHandler := rendererApp.Handler(func(w http.ResponseWriter, r *http.Request) (renderer.Page, error) {
		return renderer.NewPage("public", http.StatusOK, nil), nil
	})

	redisApp, err := redis.New(redisConfig, telemetryApp.MeterProvider(), telemetryApp.TracerProvider())
	if err != nil {
		slog.Error("create redis", "err", err)
		os.Exit(1)
	}

	defer redisApp.Close()

	kittenApp := kitten.New(kittenConfig, unsplash.New(unsplashConfig, redisApp, telemetryApp.TracerProvider()), tenor.New(tenorConfig, redisApp, telemetryApp.TracerProvider()), redisApp, telemetryApp.MeterProvider(), telemetryApp.TracerProvider(), rendererApp.PublicURL(""))

	discordApp, err := discord.New(discordConfig, rendererApp.PublicURL(""), kittenApp.DiscordHandler, telemetryApp.TracerProvider())
	if err != nil {
		slog.Error("create discord", "err", err)
		os.Exit(1)
	}

	apiHandler := http.StripPrefix(apiPrefix, kittenApp.Handler())
	gifHandler := http.StripPrefix(gifPrefix, kittenApp.GifHandler())
	slackHandler := http.StripPrefix(slackPrefix, slack.New(slackConfig, kittenApp.SlackCommand, kittenApp.SlackInteract, telemetryApp.TracerProvider()).Handler())
	discordHandler := http.StripPrefix(discordPrefix, discordApp.Handler())

	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, apiPrefix) {
			apiHandler.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, gifPrefix) {
			gifHandler.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, slackPrefix) {
			slackHandler.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, discordPrefix) {
			discordHandler.ServeHTTP(w, r)
			return
		}

		kittenHandler.ServeHTTP(w, r)
	})

	endCtx := healthApp.End(ctx)

	go appServer.Start(endCtx, "http", httputils.Handler(appHandler, healthApp, recoverer.Middleware, telemetryApp.Middleware("http"), owasp.New(owaspConfig).Middleware, cors.New(corsConfig).Middleware))

	healthApp.WaitForTermination(appServer.Done())
	server.GracefulWait(appServer.Done())
}
