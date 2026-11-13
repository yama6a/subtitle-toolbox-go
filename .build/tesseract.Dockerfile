# The image of .build/Dockerfile plus Tesseract with the fast models of every language, released with a -tesseract tag suffix.
# Build on the builder's native arch and cross-compile to the target, so the arm64 image needs no QEMU.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:eb61c3873c97416fff0116553fff65d67a3549f72e11d9758849a689cdd8b523 AS build
ARG TARGETOS TARGETARCH
ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,id=go-mod go mod download
COPY . .

# Per-arch cache id: the two builds run concurrently and would thrash a shared mount.
RUN --mount=type=cache,target=/root/.cache/go-build,id=go-build-$TARGETARCH \
    --mount=type=cache,target=/go/pkg/mod,id=go-mod \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /stbox ./cmd/stbox

# Alpine packages the standard models of 67 languages only. The fast models of all languages take 355 MB,
# the standard models 1,014 MB. The commit pins the content.
FROM --platform=$BUILDPLATFORM alpine:3@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf AS tessdata
ARG TESSDATA_COMMIT=87416418657359cb625c412a48b6e1d6d41c29bd
RUN apk add --no-cache git
WORKDIR /tessdata
RUN git init -q . \
    && git remote add origin https://github.com/tesseract-ocr/tessdata_fast.git \
    && git sparse-checkout set --no-cone '/*.traineddata' '/LICENSE' \
    && git fetch -q --depth 1 --filter=blob:none origin "$TESSDATA_COMMIT" \
    && git checkout -q FETCH_HEAD \
    && rm -rf .git

FROM alpine:3@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
RUN apk add --no-cache tesseract-ocr
COPY --from=tessdata /tessdata/*.traineddata /usr/share/tessdata/
COPY --from=tessdata /tessdata/LICENSE /usr/share/doc/tessdata_fast/LICENSE
COPY --from=build /stbox /stbox
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/doc/stbox/
WORKDIR /work
USER 65532:65532
ENTRYPOINT ["/stbox"]
