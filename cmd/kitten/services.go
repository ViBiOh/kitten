package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/httputils/v4/pkg/cors"
	"github.com/ViBiOh/httputils/v4/pkg/owasp"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/httputils/v4/pkg/server"
	"github.com/ViBiOh/kitten/pkg/kitten"
	"github.com/ViBiOh/kitten/pkg/tenor"
	"github.com/ViBiOh/kitten/pkg/unsplash"
)

//go:embed templates static
var content embed.FS

type services struct {
	server   *server.Server
	owasp    owasp.Service
	cors     cors.Service
	renderer *renderer.Service

	discord discord.Service
	slack   slack.Service
	kitten  kitten.Service
}

func newServices(ctx context.Context, config configuration, clients clients) (services, error) {
	var output services
	var err error

	output.server = server.New(config.server)
	output.owasp = owasp.New(config.owasp)
	output.cors = cors.New(config.cors)

	output.renderer, err = renderer.New(ctx, config.renderer, content, template.FuncMap{}, clients.telemetry.MeterProvider(), clients.telemetry.TracerProvider())
	if err != nil {
		return output, fmt.Errorf("renderer: %w", err)
	}

	unsplashService := unsplash.New(ctx, config.unsplash, clients.redis, clients.telemetry.TracerProvider())
	tenorService := tenor.New(ctx, config.tenor, clients.redis, clients.telemetry.TracerProvider())

	output.kitten = kitten.New(
		config.kitten,
		unsplashService,
		tenorService,
		clients.redis,
		clients.telemetry.MeterProvider(),
		clients.telemetry.TracerProvider(),
		output.renderer.PublicURL(""),
	)

	output.discord, err = discord.New(config.discord, output.renderer.PublicURL(""), output.kitten.DiscordHandler, clients.telemetry.TracerProvider())
	if err != nil {
		return output, fmt.Errorf("discord: %w", err)
	}

	output.slack = slack.New(config.slack, output.kitten.SlackCommand, output.kitten.SlackInteract, clients.telemetry.TracerProvider())

	return output, nil
}
