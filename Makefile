# Raccourcis de développement. Voir README.md.

BINARY  := bin/legifrance-mcp
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
IMAGE   ?= legifrance-mcp:$(VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test cover lint vuln run docker snapshot clean

build: ## Compile le binaire local
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/legifrance-mcp

test: ## Lance les tests avec le détecteur de concurrence
	go test -race ./...

cover: ## Produit un rapport de couverture
	go test -covermode=atomic -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

lint: ## Analyse statique (golangci-lint v2 requis)
	golangci-lint run

vuln: ## Recherche de vulnérabilités connues dans les dépendances
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

run: build ## Démarre le serveur avec la configuration de .env (dont MCP_TRANSPORT)
	set -a; [ -f .env ] && . ./.env; set +a; ./$(BINARY)

docker: ## Construit l'image Docker pour l'architecture de la machine
	docker buildx build --build-arg VERSION=$(VERSION) -t $(IMAGE) --load .

snapshot: ## Construit les binaires de toutes les plateformes dans dist/ (GoReleaser requis)
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist coverage.out
