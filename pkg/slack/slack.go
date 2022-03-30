package slack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ViBiOh/flags"
	"github.com/ViBiOh/httputils/v4/pkg/httperror"
	"github.com/ViBiOh/httputils/v4/pkg/httpjson"
	"github.com/ViBiOh/httputils/v4/pkg/logger"
	"github.com/ViBiOh/httputils/v4/pkg/request"
)

// CommandHandler for handling when user send a slash command
type CommandHandler func(ctx context.Context, w http.ResponseWriter, pathName, text string)

// InteractHandler for handling when user interact with a button
type InteractHandler func(ctx context.Context, user string, actions []InteractiveAction) Response

// Config of package
type Config struct {
	clientID      *string
	clientSecret  *string
	signingSecret *string
}

// App of package
type App struct {
	onCommand  CommandHandler
	onInteract InteractHandler

	clientID      string
	clientSecret  string
	signingSecret []byte
}

// Flags adds flags for configuring package
func Flags(fs *flag.FlagSet, prefix string, overrides ...flags.Override) Config {
	return Config{
		clientID:      flags.String(fs, prefix, "slack", "ClientID", "ClientID", "", overrides),
		clientSecret:  flags.String(fs, prefix, "slack", "ClientSecret", "ClientSecret", "", overrides),
		signingSecret: flags.String(fs, prefix, "slack", "SigningSecret", "Signing secret", "", overrides),
	}
}

// New creates new App from Config
func New(config Config, command CommandHandler, interact InteractHandler) App {
	return App{
		clientID:      *config.clientID,
		clientSecret:  *config.clientSecret,
		signingSecret: []byte(*config.signingSecret),

		onCommand:  command,
		onInteract: interact,
	}
}

// Handler for net/http
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

		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusOK)
			return

		case http.MethodPost:
			if r.URL.Path == "/interactive" {
				a.handleInteract(w, r)
			} else {
				a.onCommand(r.Context(), w, strings.TrimPrefix(r.URL.Path, "/"), r.FormValue("text"))
			}

			return

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func (a App) checkSignature(r *http.Request) bool {
	tsValue, err := strconv.ParseInt(r.Header.Get("X-Slack-Request-Timestamp"), 10, 64)
	if err != nil {
		logger.Error("unable to parse timestamp: %s", err)
		return false
	}

	if time.Unix(tsValue, 0).Before(time.Now().Add(time.Minute * -5)) {
		logger.Warn("timestamp is from 5 minutes ago")
		return false
	}

	body, err := request.ReadBodyRequest(r)
	if err != nil {
		logger.Warn("unable to read request body: %s", err)
		return false
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	slackSignature := r.Header.Get("X-Slack-Signature")
	signatureValue := []byte(fmt.Sprintf("v0:%d:%s", tsValue, body))

	sig := hmac.New(sha256.New, a.signingSecret)
	sig.Write(signatureValue)
	ownSignature := fmt.Sprintf("v0=%s", hex.EncodeToString(sig.Sum(nil)))

	if hmac.Equal([]byte(slackSignature), []byte(ownSignature)) {
		return true
	}

	logger.Error("signature mismatch from slack's one: `%s`", slackSignature)
	return false
}

func (a App) handleInteract(w http.ResponseWriter, r *http.Request) {
	rawPayload := r.FormValue("payload")
	var payload Interactive

	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		a.returnEphemeral(w, fmt.Sprintf("cannot unmarshall payload: %v", err))
		return
	}

	a.send(payload.ResponseURL, a.onInteract(r.Context(), payload.User.ID, payload.Actions))
}

func (a App) returnEphemeral(w http.ResponseWriter, message string) {
	httpjson.Write(w, http.StatusOK, NewEphemeralMessage(message))
}

func (a App) send(url string, message Response) {
	if payload, err := json.Marshal(message); err == nil {
		fmt.Printf("%s\n", payload)
	}

	_, err := request.Post(url).JSON(context.Background(), message)
	if err != nil {
		logger.Error("unable to send response: %s", err)
	}
}
