#    Copyright 2021 AERIS-Consulting e.U.
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

# The builder runs natively on the build platform and cross-compiles for the target platform.
ARG GO_VERSION=1.27.4
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS go-builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
# Defaults to the content of the file VERSION when not provided.
ARG VERSION=""

WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    VERSION="${VERSION:-$(cat VERSION)}" && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-w -s -X main.version=${VERSION}" -o /out/http-punching-ball .

# Final image: static binary on distroless, running as non-root.
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /http-punching-ball
COPY --from=go-builder /out/http-punching-ball /http-punching-ball/http-punching-ball
COPY http-server.crt http-server.key /http-punching-ball/

HEALTHCHECK --start-period=2s --interval=5s --timeout=2s --retries=5 CMD ["/http-punching-ball/http-punching-ball", "health"]
ENTRYPOINT ["/http-punching-ball/http-punching-ball"]

EXPOSE 8080
EXPOSE 8443

USER nonroot:nonroot
