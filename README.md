# HTTP Punching Ball

Lightweight HTTP server for testing of HTTP clients and as a neutral target for load tests. This service supports
both HTTP and HTTPS.

The source code is distributed under the [Apache License Version 2.0](./LICENSE).

## Endpoints

| Endpoint         | Description                                                                                          |
|------------------|------------------------------------------------------------------------------------------------------|
| `/` (any method) | Returns `202 Accepted` with the received payload as base64 in a JSON body: `{"binary":"..."}`.      |
| `GET /_stats`    | Statistics of the requests received on `/` since the start or the last reset (see below).           |
| `DELETE /_stats` | Resets the statistics.                                                                               |
| any other URI    | Returns a JSON description of the received request (URI, headers, cookies, parameters, form, body). |

The statistics contain `requestsCount`, `receivedBytes`, `earliestEpochMs`, `latestEpochMs`, `durationMs`,
`countPerSeconds` and `clientsCount`, the count of distinct connections that sent at least one request to `/`.
They are lock-free and nothing is logged per request (unless `--debug`), so that the server remains a cheap target.

## Build and run locally

Requires the Go version declared in `go.mod` or later.

```
go vet ./...
go test -race ./...
go build -ldflags "-X main.version=$(cat VERSION)" -o http-punching-ball .
./http-punching-ball --plain-port 8080
```

## Flags

```
      --debug         enables the debug mode, logging every request
  -h, --help          help for http-punching-ball
      --http          enables the plain HTTP server (default true)
      --https         enables the HTTPS server
      --plain-port    port for plain HTTP (default 8080)
      --ssl-cert      certificate file for the server (default "http-server.crt")
      --ssl-key       key file for the server certificate (default "http-server.key")
      --ssl-port      port for HTTPS (default 8443)
  -v, --version       version for http-punching-ball
```

The command `http-punching-ball health [--plain-port <port>]` exits with 0 when the local plain HTTP server answers
on `/_stats`. It is the health check of the Docker image.

## Build the Docker image

The image is based on distroless static and runs as non-root. The Go binary is cross-compiled natively on the build
platform, so no emulation is needed, even for foreign architectures.

Image for the local platform:

```
docker buildx build --load -t aerisconsulting/http-punching-ball:local .
```

Multi-architecture image (pushed to a registry, since `--load` only supports a single platform with the classic
image store):

```
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t aerisconsulting/http-punching-ball:$(cat VERSION) --push .
```

The version is read from the file `VERSION`, unless provided with `--build-arg VERSION=...`.

The CI (`.github/workflows/build.yml`) runs the tests, cross-compiles the binaries for Linux, macOS and Windows
(amd64 and arm64) and Linux ARMv7, and on each push to `main` publishes the multi-architecture image with the tags
`latest` and the version.

A tag `v<version>` matching the file `VERSION` publishes the binaries as assets of a GitHub release, with their
`SHA256SUMS`. Running the workflow manually (`workflow_dispatch`) on `main` does the same for the current version,
creating the tag when it does not exist.

## Download a binary

The binaries of each release are on the [releases page](https://github.com/aeris-consulting/http-punching-ball/releases),
named `http-punching-ball-<os>-<arch>-<version>` (`.exe` on Windows):

```
VERSION=1.1.0
curl -fLO "https://github.com/aeris-consulting/http-punching-ball/releases/download/v${VERSION}/http-punching-ball-darwin-arm64-${VERSION}"
chmod +x "http-punching-ball-darwin-arm64-${VERSION}"
```

## Use from Docker

```
docker run -d --rm -p 8080:8080 aerisconsulting/http-punching-ball
docker run -d --rm -p 8443:8443 aerisconsulting/http-punching-ball --https --http=false
docker run --rm aerisconsulting/http-punching-ball --help
```

The certificate `http-server.crt` and its key `http-server.key` are self-signed and only meant for tests.
