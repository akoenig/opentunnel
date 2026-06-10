# syntax=docker/dockerfile:1

FROM golang:1.23 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/opentunnel ./cmd/opentunnel

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/opentunnel /opentunnel

USER nonroot:nonroot
EXPOSE 8080

ENV OPENTUNNEL_LISTEN=:8080
ENV OPENTUNNEL_PUBLIC_URL=http://localhost:8080
ENV OPENTUNNEL_ARTIFACT_PATH=/opentunnel
ENV OPENTUNNEL_VERSION=dev

ENTRYPOINT ["/opentunnel"]
CMD ["relay", "--listen", ":8080", "--public-url", "http://localhost:8080", "--artifact-path", "/opentunnel", "--version", "dev"]
