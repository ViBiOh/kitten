package main

import (
	"net/http"

	"github.com/ViBiOh/ChatPotte/discord"
	"github.com/ViBiOh/ChatPotte/slack"
	"github.com/ViBiOh/httputils/v4/pkg/renderer"
	"github.com/ViBiOh/kitten/pkg/kitten"
)

func newPort(rendererService *renderer.Service, kittenService kitten.Service, slackService slack.Service, discordService discord.Service) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/search", kittenService.SearchHandler())
	mux.Handle("/gif/{content...}", kittenService.GifHandler())
	mux.Handle("/api/{content...}", kittenService.Handler())

	mux.Handle("/slack", http.StripPrefix("/slack", slackService.Handler()))
	mux.Handle("/discord", http.StripPrefix("/discord", discordService.Handler()))

	rendererService.Register(mux, func(w http.ResponseWriter, r *http.Request) (renderer.Page, error) {
		return renderer.NewPage("public", http.StatusOK, nil), nil
	})

	return mux
}
