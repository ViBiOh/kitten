package slack

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/request"
)

const (
	slackOauthURL = "https://slack.com/api/oauth.v2.access"
)

type slackOauthReponse struct {
	Team struct {
		ID string `json:"id"`
	} `json:"team"`
}

func (a App) handleOauth(w http.ResponseWriter, r *http.Request) {
	params := url.Values{}
	params.Set("code", r.URL.Query().Get("code"))
	params.Set("client_id", a.clientID)
	params.Set("client_secret", a.clientSecret)

	resp, err := request.Post(slackOauthURL).Form(r.Context(), params)
	if err != nil {
		httperror.InternalServerError(w, fmt.Errorf("unable to confirm oauth request: %s", err))
		return
	}

	var oauthResponse slackOauthReponse
	if err := httpjson.Read(resp, &oauthResponse); err != nil {
		httperror.InternalServerError(w, fmt.Errorf("unable to parse oauth response: %s", err))
		return
	}

	http.Redirect(w, r, fmt.Sprintf("https://app.slack.com/client/%s/", oauthResponse.Team.ID), http.StatusFound)
}
