package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	_ "net/http/pprof"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/alcotest"
	"github.com/ViBiOh/httputils/v4/pkg/cors"
	"github.com/ViBiOh/httputils/v4/pkg/health"
	"github.com/ViBiOh/httputils/v4/pkg/httputils"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/owasp"
	"github.com/ViBiOh/httputils/v4/pkg/prometheus"
	"github.com/ViBiOh/httputils/v4/pkg/recoverer"
	"github.com/ViBiOh/httputils/v4/pkg/redis"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/httputils/v4/pkg/request"
	"github.com/ViBiOh/httputils/v4/pkg/server"
	"github.com/ViBiOh/httputils/v4/pkg/tracer"
	"github.com/ViBiOh/kitten/pkg/kitten"
	"github.com/ViBiOh/kitten/pkg/meme"
	"github.com/ViBiOh/kitten/pkg/slack"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

//go:embed templates static
var content embed.FS

const (
	apiPrefix   = "/api"
	slackPrefix = "/slack"
)

func main() {
	fs := flag.NewFlagSet("kitten", flag.ExitOnError)

	appServerConfig := server.Flags(fs, "")
	promServerConfig := server.Flags(fs, "prometheus", flags.NewOverride("Port", uint(9090)), flags.NewOverride("IdleTimeout", "10s"), flags.NewOverride("ShutdownTimeout", "5s"))
	healthConfig := health.Flags(fs, "")

	alcotestConfig := alcotest.Flags(fs, "")
	loggerConfig := logger.Flags(fs, "logger")
	tracerConfig := tracer.Flags(fs, "tracer")
	prometheusConfig := prometheus.Flags(fs, "prometheus", flags.NewOverride("Gzip", false))
	owaspConfig := owasp.Flags(fs, "", flags.NewOverride("Csp", "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com"))
	corsConfig := cors.Flags(fs, "cors")

	redisConfig := redis.Flags(fs, "redis")

	unsplashConfig := unsplash.Flags(fs, "unsplash")
	slackConfig := slack.Flags(fs, "slack")
	rendererConfig := renderer.Flags(fs, "", flags.NewOverride("Title", "KittenBot"), flags.NewOverride("PublicURL", "https://kitten.vibioh.fr"))

	tmpFolder := flags.String(fs, "api", "kitten", "TmpFolder", "Folder used for temporary files storage", "/tmp", nil)

	logger.Fatal(fs.Parse(os.Args[1:]))

	alcotest.DoAndExit(alcotestConfig)
	logger.Global(logger.New(loggerConfig))
	defer logger.Close()

	tracerApp, err := tracer.New(tracerConfig)
	logger.Fatal(err)
	defer tracerApp.Close()
	request.AddTracerToDefaultClient(tracerApp.GetProvider())

	go func() {
		fmt.Println(http.ListenAndServe("localhost:9999", http.DefaultServeMux))
	}()

	appServer := server.New(appServerConfig)
	promServer := server.New(promServerConfig)
	prometheusApp := prometheus.New(prometheusConfig)
	healthApp := health.New(healthConfig)

	rendererApp, err := renderer.New(rendererConfig, content, template.FuncMap{}, tracerApp)
	logger.Fatal(err)

	kittenHandler := rendererApp.Handler(func(w http.ResponseWriter, r *http.Request) (renderer.Page, error) {
		return renderer.NewPage("public", http.StatusOK, nil), nil
	})

	redisApp := redis.New(redisConfig, prometheusApp.Registerer(), tracerApp)
	memeApp := meme.New(unsplash.New(unsplashConfig, redisApp), rendererApp.PublicURL(""))
	apiHandler := http.StripPrefix(apiPrefix, kitten.Handler(memeApp, strings.TrimSpace(*tmpFolder)))
	slackHandler := http.StripPrefix(slackPrefix, slack.New(slackConfig, memeApp.SlackCommand, memeApp.SlackInteract).Handler())

	appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, apiPrefix) {
			apiHandler.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, slackPrefix) {
			slackHandler.ServeHTTP(w, r)
			return
		}

		kittenHandler.ServeHTTP(w, r)
	})

	go promServer.Start("prometheus", healthApp.End(), prometheusApp.Handler())
	go appServer.Start("http", healthApp.End(), httputils.Handler(appHandler, healthApp, recoverer.Middleware, prometheusApp.Middleware, tracerApp.Middleware, owasp.New(owaspConfig).Middleware, cors.New(corsConfig).Middleware))

	healthApp.WaitForTermination(appServer.Done())
	server.GracefulWait(appServer.Done(), promServer.Done())
}
