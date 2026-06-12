# syntax=docker/dockerfile:1

FROM golang:1.26 AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN set -eux; \
	version="$(cat VERSION)"; \
	ldflags="-X opentunnel/internal/buildinfo.Version=${version}"; \
	mkdir -p /out/opentunnel-artifacts; \
	CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags "${ldflags}" -o /out/opentunnel ./cmd/opentunnel; \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${ldflags}" -o "/out/opentunnel-artifacts/opentunnel-${version}-linux-amd64" ./cmd/opentunnel; \
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "${ldflags}" -o "/out/opentunnel-artifacts/opentunnel-${version}-linux-arm64" ./cmd/opentunnel; \
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "${ldflags}" -o "/out/opentunnel-artifacts/opentunnel-${version}-darwin-amd64" ./cmd/opentunnel; \
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "${ldflags}" -o "/out/opentunnel-artifacts/opentunnel-${version}-darwin-arm64" ./cmd/opentunnel

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/opentunnel /opentunnel
COPY --from=builder /out/opentunnel-artifacts /opentunnel-artifacts

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/opentunnel"]
CMD ["relay", "--listen", ":8080"]
