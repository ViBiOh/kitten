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
	"mime/multipart"
	"net/http"
	"os"
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
type CommandHandler func(context.Context, SlashPayload) Response

// InteractHandler for handling when user interact with a button
type InteractHandler func(context.Context, InteractivePayload) Response

// Config of package
type Config struct {
	clientID      *string
	clientSecret  *string
	signingSecret *string
	accessToken   *string
}

// App of package
type App struct {
	onCommand  CommandHandler
	onInteract InteractHandler
	slackReq   request.Request

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
		accessToken:   flags.String(fs, prefix, "slack", "AccessToken", "Bot Access Token (begin with xoxb-)", "", overrides),
	}
}

// New creates new App from Config
func New(config Config, command CommandHandler, interact InteractHandler) App {
	return App{
		clientID:      *config.clientID,
		clientSecret:  *config.clientSecret,
		slackReq:      request.Post("https://slack.com/api/").Header("Authorization", fmt.Sprintf("Bearer %s", strings.TrimSpace(*config.accessToken))),
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

		case http.MethodPost:
			if r.URL.Path == "/interactive" {
				a.handleInteract(w, r)
			} else {
				payload := SlashPayload{
					ChannelID:   r.FormValue("channel_id"),
					Command:     strings.TrimPrefix(r.FormValue("command"), "/"),
					ResponseURL: r.FormValue("response_url"),
					Text:        r.FormValue("text"),
					Token:       r.FormValue("token"),
					UserID:      r.FormValue("user_id"),
				}

				httpjson.Write(w, http.StatusOK, a.onCommand(r.Context(), payload))
			}

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
	var payload InteractivePayload
	if err := json.Unmarshal([]byte(r.FormValue("payload")), &payload); err != nil {
		httpjson.Write(w, http.StatusOK, NewEphemeralMessage(fmt.Sprintf("cannot unmarshall payload: %v", err)))
		return
	}

	w.WriteHeader(http.StatusOK)

	go func() {
		ctx := context.Background()
		slackResponse := a.onInteract(ctx, payload)

		resp, err := request.Post(payload.ResponseURL).JSON(ctx, slackResponse)
		if err != nil {
			logger.Error("unable to send interact on response_url: %s", err)
		} else if discardErr := request.DiscardBody(resp.Body); discardErr != nil {
			logger.Error("unable to discard interact body on response_url: %s", err)
		}

		if slackResponse.File != nil {
			resp, err := a.slackReq.Path("files.upload").Multipart(ctx, writeMultipart(*slackResponse.File))
			if err != nil {
				logger.Error("unable to upload file: %s", err)
			} else if discardErr := request.DiscardBody(resp.Body); discardErr != nil {
				logger.Error("unable to discard file upload body: %s", err)
			}
		}
	}()
}

func writeMultipart(file File) func(*multipart.Writer) error {
	return func(mw *multipart.Writer) error {
		if err := mw.WriteField("initial_comment", file.InitialComment); err != nil {
			return err
		}

		if err := mw.WriteField("channels", strings.Join(file.Channels, ",")); err != nil {
			return err
		}

		logger.Info("Uploading `%s` with file `%s` to channel `%s`", file.InitialComment, file.Filepath, file.Channels)

		if err := addAttachment(mw, file); err != nil {
			return err
		}

		return nil
	}
}

func addAttachment(mw *multipart.Writer, file File) error {
	partWriter, err := mw.CreateFormField("file")
	if err != nil {
		return fmt.Errorf("unable to create file part: %s", err)
	}

	var fileReader io.ReadCloser
	fileReader, err = os.Open(file.Filepath)
	if err != nil {
		return fmt.Errorf("unable to open file part: %s", err)
	}

	defer func() {
		if closeErr := fileReader.Close(); closeErr != nil {
			logger.Error("unable to close file part: %s", closeErr)
		}
	}()

	if _, err = io.Copy(partWriter, fileReader); err != nil {
		return fmt.Errorf("unable to copy file part: %s", err)
	}

	return nil
}
