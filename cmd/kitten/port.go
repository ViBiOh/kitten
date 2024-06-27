package main

import (
	"net/http"

	"github.com/ViBiOh/httputils/v4/pkg/renderer"
)

func newPort(config configuration, services services) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/search", services.kitten.SearchHandler())
	mux.Handle("/gif/{content...}", services.kitten.GifHandler())
	mux.Handle("/api/{content...}", services.kitten.Handler())

	mux.Handle("/slack/", services.slack.NewServeMux())
	mux.Handle("/discord/", services.discord.NewServeMux())

	mux.Handle(config.renderer.PathPrefix+"/", services.renderer.NewServeMux(func(w http.ResponseWriter, r *http.Request) (renderer.Page, error) {
		return renderer.NewPage("public", http.StatusOK, nil), nil
	}),
	)

	return mux
}
