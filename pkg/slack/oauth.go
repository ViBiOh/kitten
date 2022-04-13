package slack

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/model"
	"github.com/ViBiOh/httputils/v4/pkg/request"
)

const (
	slackOauthURL = "https://slack.com/api/oauth.v2.access"
)

type slackOauthReponse struct {
	Enterprise struct {
		ID string `json:"id"`
	} `json:"enterprise"`
	Team struct {
		ID string `json:"id"`
	} `json:"team"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
}

func (a App) handleOauth(w http.ResponseWriter, r *http.Request) {
	params := url.Values{}
	params.Set("code", r.URL.Query().Get("code"))
	params.Set("client_id", a.clientID)
	params.Set("client_secret", a.clientSecret)

	ctx := r.Context()

	resp, err := request.Post(slackOauthURL).Form(ctx, params)
	if err != nil {
		httperror.InternalServerError(w, fmt.Errorf("unable to confirm oauth request: %s", err))
		return
	}

	var oauthResponse slackOauthReponse
	if err := httpjson.Read(resp, &oauthResponse); err != nil {
		httperror.InternalServerError(w, fmt.Errorf("unable to parse oauth response: %s", err))
		return
	}

	if !model.IsNil(a.db) && a.db.Enabled() {
		if err := a.db.DoAtomic(ctx, func(ctx context.Context) error {
			if _, err := a.GetToken(ctx, oauthResponse.Enterprise.ID, oauthResponse.Team.ID); err == nil {
				if err = a.Update(ctx, oauthResponse); err != nil {
					return fmt.Errorf("unable to update: %s", err)
				}
			} else if err = a.Create(ctx, oauthResponse); err != nil {
				return fmt.Errorf("unable to create: %s", err)
			}

			return nil
		}); err != nil {
			httperror.InternalServerError(w, fmt.Errorf("unable to save installation: %s", err))
			return
		}
	} else {
		logger.Info("%+v", oauthResponse)
	}

	http.Redirect(w, r, fmt.Sprintf("https://app.slack.com/client/%s/", oauthResponse.Team.ID), http.StatusFound)
}
