# kitten

[![Build](https://github.com/ViBiOh/kitten/workflows/Build/badge.svg)](https://github.com/ViBiOh/kitten/actions)
[![codecov](https://codecov.io/gh/ViBiOh/kitten/branch/main/graph/badge.svg)](https://codecov.io/gh/ViBiOh/kitten)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=ViBiOh_kitten&metric=alert_status)](https://sonarcloud.io/dashboard?id=ViBiOh_kitten)

## Getting started

Golang binary is built with static link. You can download it directly from the [GitHub Release page](https://github.com/ViBiOh/kitten/releases) or build it by yourself by cloning this repo and running `make`.

A Docker image is available for `amd64`, `arm` and `arm64` platforms on Docker Hub: [vibioh/kitten](https://hub.docker.com/r/vibioh/kitten/tags).

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
  -address string
        [server] Listen address {KITTEN_ADDRESS}
  -cert string
        [server] Certificate file {KITTEN_CERT}
  -corsCredentials
        [cors] Access-Control-Allow-Credentials {KITTEN_CORS_CREDENTIALS}
  -corsExpose string
        [cors] Access-Control-Expose-Headers {KITTEN_CORS_EXPOSE}
  -corsHeaders string
        [cors] Access-Control-Allow-Headers {KITTEN_CORS_HEADERS} (default "Content-Type")
  -corsMethods string
        [cors] Access-Control-Allow-Methods {KITTEN_CORS_METHODS} (default "GET")
  -corsOrigin string
        [cors] Access-Control-Allow-Origin {KITTEN_CORS_ORIGIN} (default "*")
  -csp string
        [owasp] Content-Security-Policy {KITTEN_CSP} (default "default-src 'self'; base-uri 'self'; script-src 'self' 'httputils-nonce'; style-src 'self' 'httputils-nonce'; img-src 'self' platform.slack-edge.com")
  -discordApplicationID string
        [discord] Application ID {KITTEN_DISCORD_APPLICATION_ID}
  -discordClientID string
        [discord] Client ID {KITTEN_DISCORD_CLIENT_ID}
  -discordClientSecret string
        [discord] Client Secret {KITTEN_DISCORD_CLIENT_SECRET}
  -discordPublicKey string
        [discord] Public Key {KITTEN_DISCORD_PUBLIC_KEY}
  -frameOptions string
        [owasp] X-Frame-Options {KITTEN_FRAME_OPTIONS} (default "deny")
  -giphyApiKey string
        [giphy] API Key {KITTEN_GIPHY_API_KEY}
  -graceDuration duration
        [http] Grace duration when SIGTERM received {KITTEN_GRACE_DURATION} (default 30s)
  -hsts
        [owasp] Indicate Strict Transport Security {KITTEN_HSTS} (default true)
  -idleTimeout duration
        [server] Idle Timeout {KITTEN_IDLE_TIMEOUT} (default 2m0s)
  -idsOverrides string
        [kitten] Ids overrides in the form key1|http1~key2|http2 {KITTEN_IDS_OVERRIDES}
  -key string
        [server] Key file {KITTEN_KEY}
  -loggerJson
        [logger] Log format as JSON {KITTEN_LOGGER_JSON}
  -loggerLevel string
        [logger] Logger level {KITTEN_LOGGER_LEVEL} (default "INFO")
  -loggerLevelKey string
        [logger] Key for level in JSON {KITTEN_LOGGER_LEVEL_KEY} (default "level")
  -loggerMessageKey string
        [logger] Key for message in JSON {KITTEN_LOGGER_MESSAGE_KEY} (default "message")
  -loggerTimeKey string
        [logger] Key for timestamp in JSON {KITTEN_LOGGER_TIME_KEY} (default "time")
  -minify
        Minify HTML {KITTEN_MINIFY} (default true)
  -okStatus int
        [http] Healthy HTTP Status code {KITTEN_OK_STATUS} (default 204)
  -pathPrefix string
        Root Path Prefix {KITTEN_PATH_PREFIX}
  -port uint
        [server] Listen port (0 to disable) {KITTEN_PORT} (default 1080)
  -prometheusAddress string
        [prometheus] Listen address {KITTEN_PROMETHEUS_ADDRESS}
  -prometheusCert string
        [prometheus] Certificate file {KITTEN_PROMETHEUS_CERT}
  -prometheusGzip
        [prometheus] Enable gzip compression of metrics output {KITTEN_PROMETHEUS_GZIP}
  -prometheusIdleTimeout duration
        [prometheus] Idle Timeout {KITTEN_PROMETHEUS_IDLE_TIMEOUT} (default 10s)
  -prometheusIgnore string
        [prometheus] Ignored path prefixes for metrics, comma separated {KITTEN_PROMETHEUS_IGNORE}
  -prometheusKey string
        [prometheus] Key file {KITTEN_PROMETHEUS_KEY}
  -prometheusPort uint
        [prometheus] Listen port (0 to disable) {KITTEN_PROMETHEUS_PORT} (default 9090)
  -prometheusReadTimeout duration
        [prometheus] Read Timeout {KITTEN_PROMETHEUS_READ_TIMEOUT} (default 5s)
  -prometheusShutdownTimeout duration
        [prometheus] Shutdown Timeout {KITTEN_PROMETHEUS_SHUTDOWN_TIMEOUT} (default 5s)
  -prometheusWriteTimeout duration
        [prometheus] Write Timeout {KITTEN_PROMETHEUS_WRITE_TIMEOUT} (default 10s)
  -publicURL string
        Public URL {KITTEN_PUBLIC_URL} (default "https://kitten.vibioh.fr")
  -readTimeout duration
        [server] Read Timeout {KITTEN_READ_TIMEOUT} (default 5s)
  -redisAddress string
        [redis] Redis Address (blank to disable) {KITTEN_REDIS_ADDRESS} (default "localhost:6379")
  -redisAlias string
        [redis] Connection alias, for metric {KITTEN_REDIS_ALIAS}
  -redisDatabase int
        [redis] Redis Database {KITTEN_REDIS_DATABASE}
  -redisPassword string
        [redis] Redis Password, if any {KITTEN_REDIS_PASSWORD}
  -redisUsername string
        [redis] Redis Username, if any {KITTEN_REDIS_USERNAME}
  -shutdownTimeout duration
        [server] Shutdown Timeout {KITTEN_SHUTDOWN_TIMEOUT} (default 10s)
  -slackClientID string
        [slack] ClientID {KITTEN_SLACK_CLIENT_ID}
  -slackClientSecret string
        [slack] ClientSecret {KITTEN_SLACK_CLIENT_SECRET}
  -slackSigningSecret string
        [slack] Signing secret {KITTEN_SLACK_SIGNING_SECRET}
  -title string
        Application title {KITTEN_TITLE} (default "KittenBot")
  -tmpFolder string
        [kitten] Temp folder for storing cache image {KITTEN_TMP_FOLDER} (default "/tmp")
  -tracerRate string
        [tracer] Jaeger sample rate, 'always', 'never' or a float value {KITTEN_TRACER_RATE} (default "always")
  -tracerURL string
        [tracer] Jaeger endpoint URL (e.g. http://jaeger:14268/api/traces) {KITTEN_TRACER_URL}
  -unsplashAccessKey string
        [unsplash] Unsplash Access Key {KITTEN_UNSPLASH_ACCESS_KEY}
  -unsplashName string
        [unsplash] Unsplash App name {KITTEN_UNSPLASH_NAME} (default "SayIt")
  -url string
        [alcotest] URL to check {KITTEN_URL}
  -userAgent string
        [alcotest] User-Agent for check {KITTEN_USER_AGENT} (default "Alcotest")
  -writeTimeout duration
        [server] Write Timeout {KITTEN_WRITE_TIMEOUT} (default 10s)
```
