package main

import (
	"net/http"

	"github.com/ViBiOh/httputils/v4/pkg/httputils"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
)

func newPort(clients clients, services services) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/search", services.kitten.SearchHandler())
	mux.Handle("/gif/{content...}", services.kitten.GifHandler())
	mux.Handle("/api/{content...}", services.kitten.Handler())

	mux.Handle("/slack/", http.StripPrefix("/slack", services.slack.NewServeMux()))
	mux.Handle("/discord/", http.StripPrefix("/discord", services.discord.NewServeMux()))

	services.renderer.RegisterMux(mux, func(w http.ResponseWriter, r *http.Request) (renderer.Page, error) {
		return renderer.NewPage("public", http.StatusOK, nil), nil
	})

	return httputils.Handler(mux, clients.health,
		clients.telemetry.Middleware("http"),
		services.owasp.Middleware,
		services.cors.Middleware,
	)
}
