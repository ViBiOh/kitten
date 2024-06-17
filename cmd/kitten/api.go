package main

import (
	"context"
	"embed"
	"flag"
	"html/template"
	"os"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/alcotest"
	"github.com/ViBiOh/httputils/v4/pkg/cors"
	"github.com/ViBiOh/httputils/v4/pkg/health"
	"github.com/ViBiOh/httputils/v4/pkg/httputils"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/owasp"
	"github.com/ViBiOh/httputils/v4/pkg/pprof"
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

func main() {
	fs := flag.NewFlagSet("kitten", flag.ExitOnError)
	fs.Usage = flags.Usage(fs)

	appServerConfig := server.Flags(fs, "")
	healthConfig := health.Flags(fs, "")

	alcotestConfig := alcotest.Flags(fs, "")
	loggerConfig := logger.Flags(fs, "logger")
	telemetryConfig := telemetry.Flags(fs, "telemetry")
	pprofConfig := pprof.Flags(fs, "pprof")
	owaspConfig := owasp.Flags(fs, "", flags.NewOverride("Csp", "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com"))
	corsConfig := cors.Flags(fs, "cors")

	redisConfig := redis.Flags(fs, "redis")

	kittenConfig := kitten.Flags(fs, "")
	unsplashConfig := unsplash.Flags(fs, "unsplash")
	tenorConfig := tenor.Flags(fs, "tenor")
	slackConfig := slack.Flags(fs, "slack")
	discordConfig := discord.Flags(fs, "discord")
	rendererConfig := renderer.Flags(fs, "", flags.NewOverride("Title", "KittenBot"), flags.NewOverride("PublicURL", "https://kitten.vibioh.fr"))

	_ = fs.Parse(os.Args[1:])

	alcotest.DoAndExit(alcotestConfig)

	ctx := context.Background()

	logger.Init(ctx, loggerConfig)

	healthService := health.New(ctx, healthConfig)

	telemetryService, err := telemetry.New(ctx, telemetryConfig)
	logger.FatalfOnErr(ctx, err, "create telemetry")

	defer telemetryService.Close(ctx)

	logger.AddOpenTelemetryToDefaultLogger(telemetryService)
	request.AddOpenTelemetryToDefaultClient(telemetryService.MeterProvider(), telemetryService.TracerProvider())

	service, version, envName := telemetryService.GetServiceVersionAndEnv()
	pprofApp := pprof.New(pprofConfig, service, version, envName)

	go pprofApp.Start(healthService.DoneCtx())

	appServer := server.New(appServerConfig)

	rendererService, err := renderer.New(ctx, rendererConfig, content, template.FuncMap{}, telemetryService.MeterProvider(), telemetryService.TracerProvider())
	logger.FatalfOnErr(ctx, err, "create renderer")

	redisClient, err := redis.New(ctx, redisConfig, telemetryService.MeterProvider(), telemetryService.TracerProvider())
	logger.FatalfOnErr(ctx, err, "create redis")

	defer redisClient.Close(ctx)

	endCtx := healthService.EndCtx()

	kittenService := kitten.New(
		kittenConfig,
		unsplash.New(endCtx, unsplashConfig, redisClient, telemetryService.TracerProvider()),
		tenor.New(endCtx, tenorConfig, redisClient, telemetryService.TracerProvider()),
		redisClient,
		telemetryService.MeterProvider(),
		telemetryService.TracerProvider(),
		rendererService.PublicURL(""),
	)

	slackService := slack.New(slackConfig, kittenService.SlackCommand, kittenService.SlackInteract, telemetryService.TracerProvider())

	discordService, err := discord.New(discordConfig, rendererService.PublicURL(""), kittenService.DiscordHandler, telemetryService.TracerProvider())
	logger.FatalfOnErr(ctx, err, "create discord")

	port := newPort(rendererService, kittenService, slackService, discordService)

	go appServer.Start(endCtx, httputils.Handler(port, healthService, telemetryService.Middleware("http"), owasp.New(owaspConfig).Middleware, cors.New(corsConfig).Middleware))

	healthService.WaitForTermination(appServer.Done())

	server.GracefulWait(appServer.Done())
}
