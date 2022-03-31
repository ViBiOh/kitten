package discord

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/request"
)

func (a App) handleOauth(w http.ResponseWriter, r *http.Request) {
	params := url.Values{}
	params.Set("code", r.URL.Query().Get("code"))
	params.Set("client_id", a.clientID)
	params.Set("client_secret", a.clientSecret)
	params.Set("grant_type", "authorization_code")
	params.Set("redirect_uri", a.website)

	resp, err := discordRequest.Path("/oauth2/token").Method(http.MethodPost).Form(r.Context(), params)
	if err != nil {
		httperror.InternalServerError(w, fmt.Errorf("unable to confirm oauth request: %s", err))
		return
	}

	if err := request.DiscardBody(resp.Body); err != nil {
		httperror.InternalServerError(w, fmt.Errorf("unable to discard body: %s", err))
		return
	}

	http.Redirect(w, r, a.website, http.StatusFound)
}
