# syntax=docker/dockerfile:1

# Étape de compilation : exécutée sur l'architecture de l'hôte de build,
# avec compilation croisée vers la cible (linux/amd64 par défaut).
ARG GO_VERSION=1.27
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev

WORKDIR /src

# Les dépendances sont téléchargées dans une couche distincte pour profiter du cache.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/legifrance-mcp ./cmd/legifrance-mcp

# Image finale minimale : pas de shell, utilisateur non root, certificats CA inclus.
FROM gcr.io/distroless/static-debian13:nonroot

ARG VERSION=dev
LABEL org.opencontainers.image.title="legifrance-mcp" \
      org.opencontainers.image.description="Serveur MCP pour l'API Légifrance (DILA / PISTE)" \
      org.opencontainers.image.version="${VERSION}"

COPY --from=build /out/legifrance-mcp /usr/local/bin/legifrance-mcp

ENV MCP_TRANSPORT=http \
    MCP_HTTP_ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/legifrance-mcp"]
