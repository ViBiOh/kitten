# kitten

[![Build](https://github.com/ViBiOh/kitten/workflows/Build/badge.svg)](https://github.com/ViBiOh/kitten/actions)

## Getting started

Golang binary is built with static link. You can download it directly from the [GitHub Release page](https://github.com/ViBiOh/kitten/releases) or build it by yourself by cloning this repo and running `make`.

You can configure app by passing CLI args or environment variables (cf. [Usage](#usage) section). CLI override environment variables.

You'll find a Kubernetes exemple in the [`infra/`](infra) folder, using my [`app chart`](https://github.com/ViBiOh/charts/tree/main/app)

## CI

Following variables are required for CI:

|            Name            |           Purpose           |
| :------------------------: | :-------------------------: |
|      **DOCKER_USER**       | for publishing Docker image |
|      **DOCKER_PASS**       | for publishing Docker image |
| **SCRIPTS_NO_INTERACTIVE** |  for running scripts in CI  |

## Usage

The application can be configured by passing CLI args described below or their equivalent as environment variable. CLI values take precedence over environments variables.

Be careful when using the CLI values, if someone list the processes on the system, they will appear in plain-text. Pass secrets by environment variables: it's less easily visible.

```bash
Usage of kitten:
  --address               string        [server] Listen address ${KITTEN_ADDRESS}
  --cert                  string        [server] Certificate file ${KITTEN_CERT}
  --corsCredentials                     [cors] Access-Control-Allow-Credentials ${KITTEN_CORS_CREDENTIALS} (default false)
  --corsExpose            string        [cors] Access-Control-Expose-Headers ${KITTEN_CORS_EXPOSE}
  --corsHeaders           string        [cors] Access-Control-Allow-Headers ${KITTEN_CORS_HEADERS} (default "Content-Type")
  --corsMethods           string        [cors] Access-Control-Allow-Methods ${KITTEN_CORS_METHODS} (default "GET")
  --corsOrigin            string        [cors] Access-Control-Allow-Origin ${KITTEN_CORS_ORIGIN} (default "*")
  --csp                   string        [owasp] Content-Security-Policy ${KITTEN_CSP} (default "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com")
  --discordApplicationID  string        [discord] Application ID ${KITTEN_DISCORD_APPLICATION_ID}
  --discordBotToken       string        [discord] Bot Token ${KITTEN_DISCORD_BOT_TOKEN}
  --discordClientID       string        [discord] Client ID ${KITTEN_DISCORD_CLIENT_ID}
  --discordClientSecret   string        [discord] Client Secret ${KITTEN_DISCORD_CLIENT_SECRET}
  --discordPublicKey      string        [discord] Public Key ${KITTEN_DISCORD_PUBLIC_KEY}
  --frameOptions          string        [owasp] X-Frame-Options ${KITTEN_FRAME_OPTIONS} (default "deny")
  --graceDuration         duration      [http] Grace duration when signal received ${KITTEN_GRACE_DURATION} (default 30s)
  --hsts                                [owasp] Indicate Strict Transport Security ${KITTEN_HSTS} (default true)
  --idleTimeout           duration      [server] Idle Timeout ${KITTEN_IDLE_TIMEOUT} (default 2m0s)
  --key                   string        [server] Key file ${KITTEN_KEY}
  --loggerJson                          [logger] Log format as JSON ${KITTEN_LOGGER_JSON} (default false)
  --loggerLevel           string        [logger] Logger level ${KITTEN_LOGGER_LEVEL} (default "INFO")
  --loggerLevelKey        string        [logger] Key for level in JSON ${KITTEN_LOGGER_LEVEL_KEY} (default "level")
  --loggerMessageKey      string        [logger] Key for message in JSON ${KITTEN_LOGGER_MESSAGE_KEY} (default "msg")
  --loggerTimeKey         string        [logger] Key for timestamp in JSON ${KITTEN_LOGGER_TIME_KEY} (default "time")
  --minify                              Minify HTML ${KITTEN_MINIFY} (default true)
  --name                  string        [server] Name ${KITTEN_NAME} (default "http")
  --okStatus              int           [http] Healthy HTTP Status code ${KITTEN_OK_STATUS} (default 204)
  --pathPrefix            string        Root Path Prefix ${KITTEN_PATH_PREFIX}
  --port                  uint          [server] Listen port (0 to disable) ${KITTEN_PORT} (default 1080)
  --pprofAgent            string        [pprof] URL of the Datadog Trace Agent (e.g. http://datadog.observability:8126) ${KITTEN_PPROF_AGENT}
  --pprofPort             int           [pprof] Port of the HTTP server (0 to disable) ${KITTEN_PPROF_PORT} (default 0)
  --publicURL             string        Public URL ${KITTEN_PUBLIC_URL} (default "https://kitten.vibioh.fr")
  --readTimeout           duration      [server] Read Timeout ${KITTEN_READ_TIMEOUT} (default 5s)
  --redisAddress          string slice  [redis] Redis Address host:port (blank to disable) ${KITTEN_REDIS_ADDRESS}, as a string slice, environment variable separated by "," (default [127.0.0.1:6379])
  --redisDatabase         int           [redis] Redis Database ${KITTEN_REDIS_DATABASE} (default 0)
  --redisMinIdleConn      int           [redis] Redis Minimum Idle Connections ${KITTEN_REDIS_MIN_IDLE_CONN} (default 0)
  --redisPassword         string        [redis] Redis Password, if any ${KITTEN_REDIS_PASSWORD}
  --redisPoolSize         int           [redis] Redis Pool Size (default GOMAXPROCS*10) ${KITTEN_REDIS_POOL_SIZE} (default 0)
  --redisUsername         string        [redis] Redis Username, if any ${KITTEN_REDIS_USERNAME}
  --shutdownTimeout       duration      [server] Shutdown Timeout ${KITTEN_SHUTDOWN_TIMEOUT} (default 10s)
  --slackClientID         string        [slack] ClientID ${KITTEN_SLACK_CLIENT_ID}
  --slackClientSecret     string        [slack] ClientSecret ${KITTEN_SLACK_CLIENT_SECRET}
  --slackSigningSecret    string        [slack] Signing secret ${KITTEN_SLACK_SIGNING_SECRET}
  --telemetryRate         string        [telemetry] OpenTelemetry sample rate, 'always', 'never' or a float value ${KITTEN_TELEMETRY_RATE} (default "always")
  --telemetryURL          string        [telemetry] OpenTelemetry gRPC endpoint (e.g. otel-exporter:4317) ${KITTEN_TELEMETRY_URL}
  --telemetryUint64                     [telemetry] Change OpenTelemetry Trace ID format to an unsigned int 64 ${KITTEN_TELEMETRY_UINT64} (default true)
  --tenorApiKey           string        [tenor] API Key ${KITTEN_TENOR_API_KEY}
  --tenorClientKey        string        [tenor] Client Key ${KITTEN_TENOR_CLIENT_KEY}
  --title                 string        Application title ${KITTEN_TITLE} (default "KittenBot")
  --tmpFolder             string        [kitten] Temp folder for storing cache image ${KITTEN_TMP_FOLDER} (default "/tmp")
  --unsplashAccessKey     string        [unsplash] Unsplash Access Key ${KITTEN_UNSPLASH_ACCESS_KEY}
  --unsplashName          string        [unsplash] Unsplash App name ${KITTEN_UNSPLASH_NAME} (default "SayIt")
  --url                   string        [alcotest] URL to check ${KITTEN_URL}
  --userAgent             string        [alcotest] User-Agent for check ${KITTEN_USER_AGENT} (default "Alcotest")
  --writeTimeout          duration      [server] Write Timeout ${KITTEN_WRITE_TIMEOUT} (default 10s)
```
