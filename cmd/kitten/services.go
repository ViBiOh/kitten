package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
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
	renderer *renderer.Service

	discord discord.Service
	slack   slack.Service
	kitten  kitten.Service
}

func newServices(ctx context.Context, config configuration, clients clients) (services, error) {
	rendererService, err := renderer.New(ctx, config.renderer, content, template.FuncMap{}, clients.telemetry.MeterProvider(), clients.telemetry.TracerProvider())
	if err != nil {
		return services{}, fmt.Errorf("renderer: %w", err)
	}

	unsplashService := unsplash.New(ctx, config.unsplash, clients.redis, clients.telemetry.TracerProvider())
	tenorService := tenor.New(ctx, config.tenor, clients.redis, clients.telemetry.TracerProvider())

	kittenService := kitten.New(
		config.kitten,
		unsplashService,
		tenorService,
		clients.redis,
		clients.telemetry.MeterProvider(),
		clients.telemetry.TracerProvider(),
		rendererService.PublicURL(""),
	)

	discordService, err := discord.New(config.discord, rendererService.PublicURL(""), kittenService.DiscordHandler, clients.telemetry.TracerProvider())
	if err != nil {
		return services{}, fmt.Errorf("discord: %w", err)
	}

	slackService := slack.New(config.slack, kittenService.SlackCommand, kittenService.SlackInteract, clients.telemetry.TracerProvider())

	return services{
		server:   server.New(config.server),
		renderer: rendererService,

		kitten:  kittenService,
		discord: discordService,
		slack:   slackService,
	}, nil
}
