package discord

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
)

// Start discord configuration
func (a App) Start(commands map[string]Command) error {
	if len(a.applicationID) == 0 {
		return nil
	}

	ctx := context.Background()

	data := url.Values{}
	data.Add("grant_type", "client_credentials")
	data.Add("scope", "applications.commands.update")

	resp, err := discordRequest.Method(http.MethodPost).Path("/oauth2/token").BasicAuth(a.clientID, a.clientSecret).Form(ctx, data)
	if err != nil {
		return fmt.Errorf("unable to get token: %s", err)
	}

	content := make(map[string]any)
	if err := httpjson.Read(resp, &content); err != nil {
		return fmt.Errorf("unable to read oauth token: %s", err)
	}

	bearer := content["access_token"].(string)
	rootURL := fmt.Sprintf("/applications/%s", a.applicationID)

	for name, command := range commands {
		for _, url := range getRegisterURLs(command) {
			url := rootURL + url
			logger.WithField("command", name).Info("Configuring with URL `%s`", url)

			_, err := discordRequest.Method(http.MethodPost).Path(url).Header("Authorization", fmt.Sprintf("Bearer %s", bearer)).JSON(ctx, command)
			if err != nil {
				return fmt.Errorf("unable to configure `%s` command: %s", name, err)
			}
		}

		logger.Info("Command `%s` configured!", name)
	}

	return nil
}

func getRegisterURLs(command Command) []string {
	if len(command.Guilds) == 0 {
		return []string{"/commands"}
	}

	urls := make([]string, len(command.Guilds))

	for i, guild := range command.Guilds {
		urls[i] = fmt.Sprintf("/guilds/%s/commands", guild)
	}

	return urls
}
