package discord

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/query"
	"github.com/ViBiOh/httputils/v4/pkg/request"
)

// OnMessage handle message event
type OnMessage func(context.Context, InteractionRequest) (InteractionResponse, func() InteractionResponse)

var discordRequest = request.New().URL("https://discord.com/api/v8")

// App of package
type App struct {
	applicationID string
	clientID      string
	clientSecret  string
	website       string
	publicKey     []byte
	handler       OnMessage
}

// Config of package
type Config struct {
	applicationID *string
	publicKey     *string
	clientID      *string
	clientSecret  *string
	website       *string
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		applicationID: flags.String(fs, prefix, "discord", "ApplicationID", "Application ID", "", overrides),
		publicKey:     flags.String(fs, prefix, "discord", "PublicKey", "Public Key", "", overrides),
		clientID:      flags.String(fs, prefix, "discord", "ClientID", "Client ID", "", overrides),
		clientSecret:  flags.String(fs, prefix, "discord", "ClientSecret", "Client Secret", "", overrides),
	}
}

// New creates new App from Config
func New(config Config, website string, handler OnMessage) (App, error) {
	publicKeyStr := *config.publicKey
	if len(publicKeyStr) == 0 {
		return App{}, nil
	}

	publicKey, err := hex.DecodeString(publicKeyStr)
	if err != nil {
		return App{}, fmt.Errorf("unable to decode public key string: %s", err)
	}

	return App{
		applicationID: *config.applicationID,
		publicKey:     publicKey,
		clientID:      *config.clientID,
		clientSecret:  *config.clientSecret,
		website:       website,
		handler:       handler,
	}, nil
}

// Handler for request. Should be use with net/http
func (a App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth" {
			a.handleOauth(w, r)
			return
		}

		if !a.checkSignature(r) {
			httperror.Unauthorized(w, errors.New("invalid signature"))
			return
		}

		if query.IsRoot(r) && r.Method == http.MethodPost {
			a.handleWebhook(w, r)
			return
		}

		httperror.NotFound(w)
	})
}

func (a App) checkSignature(r *http.Request) bool {
	sig, err := hex.DecodeString(r.Header.Get("X-Signature-Ed25519"))
	if err != nil {
		logger.Warn("unable to decode signature string: %s", err)
		return false
	}

	if len(sig) != ed25519.SignatureSize || sig[63]&224 != 0 {
		logger.Warn("length of signature is invalid: %d", len(sig))
		return false
	}

	body, err := request.ReadBodyRequest(r)
	if err != nil {
		logger.Warn("unable to read request body: %s", err)
		return false
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var msg bytes.Buffer
	msg.WriteString(r.Header.Get("X-Signature-Timestamp"))
	msg.Write(body)

	return ed25519.Verify(ed25519.PublicKey(a.publicKey), msg.Bytes(), sig)
}

func (a App) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var message InteractionRequest
	if err := httpjson.Parse(r, &message); err != nil {
		httperror.BadRequest(w, err)
		return
	}

	if message.Type == pingInteraction {
		httpjson.Write(w, http.StatusOK, InteractionResponse{Type: pongCallback})
		return
	}

	response, asyncFn := a.handler(r.Context(), message)

	httpjson.Write(w, http.StatusOK, response)
	if asyncFn != nil {
		go func() {
			deferredResponse := asyncFn()

			resp, err := discordRequest.Method(http.MethodPatch).Path(fmt.Sprintf("/webhooks/%s/%s/messages/@original", a.applicationID, message.Token)).JSON(context.Background(), deferredResponse.Data)
			if err != nil {
				logger.Error("unable to send async response: %s", err)
				return
			}

			if err = request.DiscardBody(resp.Body); err != nil {
				logger.Error("unable to discard async body: %s", err)
			}
		}()
	}
}
