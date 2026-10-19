# legifrance-mcp

Serveur [MCP](https://modelcontextprotocol.io) (Model Context Protocol) écrit en Go, qui donne à un assistant IA (Claude, etc.) un accès en lecture au droit français via l'**API Légifrance** de la DILA, publiée sur la plateforme [PISTE](https://piste.gouv.fr).

Il permet notamment de :

- rechercher dans les codes, lois et décrets, le Journal officiel, la jurisprudence (judiciaire, administrative, constitutionnelle, financière), les conventions collectives, les accords d'entreprise, les délibérations CNIL et les circulaires ;
- lire un article, un texte consolidé à une date donnée, une décision de justice, une convention collective ;
- parcourir la table des matières d'un code et l'historique des versions d'un article.

## Sommaire

- [Prérequis](#prérequis)
- [Installation](#installation)
- [Configuration](#configuration)
- [Utilisation avec un client MCP](#utilisation-avec-un-client-mcp)
- [Outils exposés](#outils-exposés)
- [Architecture](#architecture)
- [Développement](#développement)
- [CI/CD GitHub Actions](#cicd-github-actions)
- [Sécurité](#sécurité)
- [Limites connues](#limites-connues)

## Prérequis

- **Un compte PISTE** et une application abonnée à l'API Légifrance :
  1. créer un compte sur [piste.gouv.fr](https://piste.gouv.fr) ;
  2. accepter les CGU de l'API Légifrance (environnement _sandbox_ et/ou _production_) ;
  3. créer une application et y associer l'API Légifrance ;
  4. récupérer le **client ID** et le **client secret** OAuth de l'application.
- Go 1.27 ou plus pour compiler, ou Docker, ou un binaire précompilé.

## Installation

### Depuis les sources

```bash
make build            # produit bin/legifrance-mcp
```

ou directement :

```bash
go build -o bin/legifrance-mcp ./cmd/legifrance-mcp
```

### Image Docker

```bash
make docker           # image locale legifrance-mcp:<version> (architecture de la machine)
```

La CI publie une image multi-architecture (amd64 et arm64) sur `ghcr.io/<owner>/<repo>`.

### Binaires précompilés

Chaque [release GitHub](../../releases) fournit des binaires pour Windows (amd64), macOS (Apple Silicon), Linux amd64 et Linux arm64, avec le fichier `checksums.txt`.

## Configuration

Toute la configuration passe par des variables d'environnement (voir `.env.example`). Certaines peuvent être surchargées en ligne de commande. Les secrets ne sont volontairement pas acceptés en option, pour ne pas apparaître dans la liste des processus.

| Variable                    | Option                | Défaut                  | Description                                                                            |
| --------------------------- | --------------------- | ----------------------- | -------------------------------------------------------------------------------------- |
| `LEGIFRANCE_CLIENT_ID`      | —                     | **obligatoire**         | Client ID de l'application PISTE                                                       |
| `LEGIFRANCE_CLIENT_SECRET`  | —                     | **obligatoire**         | Client secret de l'application PISTE                                                   |
| `LEGIFRANCE_ENV`            | `-env`                | `production`            | `production` ou `sandbox` (détermine les URL par défaut)                               |
| `LEGIFRANCE_API_URL`        | —                     | selon l'environnement   | URL de base de l'API (HTTPS obligatoire)                                               |
| `LEGIFRANCE_TOKEN_URL`      | —                     | selon l'environnement   | URL du serveur OAuth (HTTPS obligatoire)                                               |
| `LEGIFRANCE_TIMEOUT`        | `-timeout`            | `30s`                   | Délai maximal d'un appel (`30s`, `1m` ou un nombre de secondes)                        |
| `MCP_TRANSPORT`             | `-transport`          | `stdio`                 | `stdio` (usage local) ou `http` (Streamable HTTP)                                      |
| `MCP_HTTP_ADDR`             | `-addr`               | `:8080`                 | Adresse d'écoute du transport HTTP                                                     |
| `MCP_AUTH_TOKEN`            | —                     | **obligatoire en HTTP** | Jeton Bearer exigé sur `/mcp` (32 caractères minimum, ex. `openssl rand -hex 32`)      |
| `MCP_ALLOW_UNAUTHENTICATED` | —                     | `false`                 | `true` autorise le mode HTTP sans jeton (derrière une passerelle qui authentifie déjà) |
| `MCP_MAX_CONCURRENCY`       | —                     | `8`                     | Nombre maximal de requêtes MCP traitées simultanément en HTTP                          |
| `MCP_MAX_CONTENT_LENGTH`    | `-max-content-length` | `20000`                 | Taille par défaut (en caractères) d'une réponse avant pagination                       |
| `LOG_LEVEL`                 | `-log-level`          | `info`                  | `debug`, `info`, `warn` ou `error` (logs JSON sur stderr)                              |

URL utilisées par défaut :

| Environnement | API                                                               | OAuth                                                 |
| ------------- | ----------------------------------------------------------------- | ----------------------------------------------------- |
| production    | `https://api.piste.gouv.fr/dila/legifrance/lf-engine-app`         | `https://oauth.piste.gouv.fr/api/oauth/token`         |
| sandbox       | `https://sandbox-api.piste.gouv.fr/dila/legifrance/lf-engine-app` | `https://sandbox-oauth.piste.gouv.fr/api/oauth/token` |

> Les données de la sandbox ne sont pas identiques à celles de production, et cet environnement peut être indisponible.

## Utilisation avec un client MCP

### Claude Code (stdio)

```bash
claude mcp add legifrance \
  -e LEGIFRANCE_CLIENT_ID=xxx \
  -e LEGIFRANCE_CLIENT_SECRET=yyy \
  -- /chemin/vers/bin/legifrance-mcp
```

### Claude Desktop ou autre client (stdio)

```json
{
  "mcpServers": {
    "legifrance": {
      "command": "/chemin/vers/bin/legifrance-mcp",
      "env": {
        "LEGIFRANCE_CLIENT_ID": "xxx",
        "LEGIFRANCE_CLIENT_SECRET": "yyy"
      }
    }
  }
}
```

### Serveur HTTP (Docker)

```bash
docker run -d --name legifrance-mcp -p 8080:8080 \
  -e LEGIFRANCE_CLIENT_ID=xxx \
  -e LEGIFRANCE_CLIENT_SECRET=yyy \
  -e MCP_AUTH_TOKEN="$(openssl rand -hex 32)" \
  ghcr.io/<owner>/<repo>:latest
```

- point d'entrée MCP : `POST http://<hôte>:8080/mcp` (en-tête `Authorization: Bearer <MCP_AUTH_TOKEN>`) ;
- sonde de santé : `GET /healthz` (sans authentification) ;
- déclaration côté Claude Code : `claude mcp add --transport http legifrance http://<hôte>:8080/mcp --header "Authorization: Bearer <jeton>"`.

Le serveur fonctionne en mode **sans état** : plusieurs instances peuvent tourner derrière un répartiteur de charge sans affinité de session.

## Outils exposés

Tous les outils sont en lecture seule. Ils renvoient du Markdown nettoyé (sans HTML) qui cite les identifiants Légifrance.

| Outil                      | Rôle                                                                                 | Endpoint(s)                                       |
| -------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------- |
| `search`                   | Recherche plein texte par fonds, avec filtres (facettes), tri et pagination          | `/search`                                         |
| `describe_search_options`  | Champs, filtres et tris disponibles pour un fonds                                    | référentiel embarqué                              |
| `suggest`                  | Autocomplétion de titres (« loi Macron », « code du travail »…)                      | `/suggest`                                        |
| `get_document`             | Lecture d'un document, en choisissant l'endpoint d'après le préfixe de l'identifiant | voir ci-dessous                                   |
| `get_article_by_number`    | Article en vigueur d'un code, d'après son numéro (ex. Code civil, art. 1240)         | `/consult/getArticleWithIdAndNum`                 |
| `get_article_history`      | Toutes les versions d'un article, avec leur période de validité                      | `/consult/getArticle`, `/consult/getArticleByCid` |
| `get_table_of_contents`    | Plan d'un code ou d'un texte consolidé à une date                                    | `/consult/legi/tableMatieres`                     |
| `get_code_section`         | Contenu d'une section de code à une date                                             | `/consult/code`                                   |
| `get_collective_agreement` | Convention collective et textes rattachés, d'après l'IDCC                            | `/consult/kaliContIdcc`                           |
| `get_text_by_nor`          | Texte du JO d'après son NOR                                                          | `/consult/getJoWithNor`                           |
| `list_codes`               | Liste des codes et de leurs identifiants                                             | `/list/code`                                      |
| `list_texts`               | Liste des lois, ordonnances, décrets et arrêtés, par nature ou par période           | `/list/loda`                                      |
| `get_latest_journals`      | Derniers numéros du Journal officiel                                                 | `/consult/lastNJo`                                |

Choix de l'endpoint par `get_document` :

| Préfixe                                        | Type de document                            | Endpoint                                  |
| ---------------------------------------------- | ------------------------------------------- | ----------------------------------------- |
| `LEGIARTI`, `JORFARTI`, `KALIARTI`…            | article                                     | `/consult/getArticle`                     |
| `LEGITEXT`                                     | texte consolidé (version à la date `date`)  | `/consult/lawDecree`                      |
| `JORFTEXT`                                     | texte publié au JO                          | `/consult/jorf`                           |
| `JURITEXT`, `CETATEXT`, `CONSTEXT`, `JUFITEXT` | décision de justice                         | `/consult/juri`                           |
| `KALITEXT` / `KALICONT`                        | texte ou conteneur de convention collective | `/consult/kaliText` / `/consult/kaliCont` |
| `ACCOTEXT`                                     | accord d'entreprise                         | `/consult/acco`                           |
| `CNILTEXT`                                     | délibération CNIL                           | `/consult/cnil`                           |
| identifiant numérique                          | circulaire                                  | `/consult/circulaire`                     |

**Pagination des contenus longs** : au-delà de `max_length` caractères (20 000 par défaut), la réponse est tronquée en fin de ligne et se termine par une indication du type `Call again with start_index=N`. Le modèle rappelle alors l'outil avec ce `start_index`.

**Choix de conception** : l'API compte 68 endpoints, mais exposer un outil par endpoint saturerait le contexte du modèle et compliquerait le choix de l'outil. Le serveur propose donc un nombre réduit d'outils orientés usage, qui couvrent les besoins courants.

## Architecture

```
cmd/legifrance-mcp/     point d'entrée : configuration, transports stdio/HTTP, arrêt propre
internal/config/        lecture et validation de la configuration (env + options)
internal/legifrance/    client HTTP typé : OAuth2 PISTE, nouvelles tentatives, DTO réduits
internal/tools/         outils MCP : schémas d'entrée, validation, rendu Markdown
internal/textutil/      conversion HTML → texte, pagination, troncature
doc/                    Swagger de l'API et référentiel des tris et filtres (DILA)
```

Points notables :

- **Authentification** : `golang.org/x/oauth2/clientcredentials` obtient le jeton PISTE, le met en cache et le renouvelle avant expiration.
- **Robustesse** :
  - nouvelles tentatives avec backoff exponentiel sur 429, 502, 503, 504 et sur les erreurs réseau, en respectant `Retry-After` ;
  - réponses limitées à 64 Mo ;
  - délai maximal par appel.
- **Décodage tolérant** :
  - les dates de l'API arrivent sous plusieurs formes (horodatage en millisecondes, ISO 8601, `AAAA-MM-JJ`), le type `legifrance.Date` les accepte toutes ;
  - la date `2999-01-01` est interprétée comme « sans date de fin ».
- **Schémas d'entrée** : ils sont déduits des structs Go (tags `jsonschema`), puis complétés par des énumérations, des bornes et des motifs, pour que le SDK MCP rejette les entrées invalides avant l'appel à l'API.
- **Testabilité** : les outils dépendent de l'interface `tools.API`, ce qui permet de les tester avec un faux client à travers une vraie session MCP en mémoire.

## Développement

```bash
make test     # tests unitaires avec -race
make cover    # couverture
make lint     # golangci-lint v2 (configuration : .golangci.yml)
make vuln     # govulncheck
make run      # lance le serveur avec la configuration de .env
```

Pour inspecter le serveur à la main, utiliser le [MCP Inspector](https://github.com/modelcontextprotocol/inspector) :

```bash
npx @modelcontextprotocol/inspector ./bin/legifrance-mcp
```

Conventions : code, messages d'erreur et descriptions d'outils en anglais ; commentaires et documentation en français.

## CI/CD GitHub Actions

Le workflow `.github/workflows/ci.yml` se déclenche sur les pull requests, sur les push vers `main` et sur les tags `v*`.

| Job           | Rôle                                                                                                                                            |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `lint`        | `go mod tidy -diff` et `golangci-lint`                                                                                                          |
| `test`        | tests avec `-race` (dont un test qui vérifie les énumérations par rapport à `doc/swagger.json`), couverture affichée dans le résumé du workflow |
| `govulncheck` | analyse des vulnérabilités connues des dépendances                                                                                              |
| `binaries`    | binaires produits par [GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`)                                                                 |
| `docker`      | image multi-architecture **linux/amd64 + linux/arm64**, publiée sur GitHub Container Registry                                                   |

Les jobs `binaries` et `docker` ne démarrent que si `lint`, `test` et `govulncheck` réussissent.

**Binaires** (archives `tar.gz`, `zip` pour Windows, et fichier `checksums.txt`) :

| Système | Architecture          |
| ------- | --------------------- |
| Windows | amd64                 |
| macOS   | arm64 (Apple Silicon) |
| Linux   | amd64                 |
| Linux   | arm64                 |

Sur une pull request ou un push sur `main`, ils sont construits en mode « snapshot » et disponibles en artefacts du workflow pendant 14 jours. Sur un tag `v*`, GoReleaser crée une **GitHub Release** avec les archives et le changelog.

**Image Docker** `ghcr.io/<owner>/<repo>` :

| Événement       | Tags publiés                              |
| --------------- | ----------------------------------------- |
| pull request    | aucun (image construite sans publication) |
| push sur `main` | `latest`, `sha-<commit>`                  |
| tag `v1.2.3`    | `1.2.3`, `1.2`, `sha-<commit>`            |

L'image est accompagnée d'attestations de provenance (SLSA) et d'un SBOM. Le cache de build utilise le cache GitHub Actions. La compilation est croisée dans le Dockerfile, ce qui évite l'émulation QEMU pour arm64.

**Publier une version :**

```bash
git tag v1.0.0 && git push origin v1.0.0
```

Aucun secret à configurer : le workflow utilise le `GITHUB_TOKEN` fourni par GitHub. Les droits sont limités job par job (`contents: write` pour la release, `packages: write` pour GHCR). Les actions sont épinglées par SHA de commit. Dependabot (`.github/dependabot.yml`) ouvre chaque semaine des pull requests groupées pour mettre à jour les actions, les modules Go et les images Docker de base.

Pour tester la production des binaires en local : `make snapshot` (GoReleaser requis).

## Sécurité

- Les identifiants PISTE ne sont lus que depuis l'environnement. Ils ne sont jamais journalisés, et les erreurs d'authentification ne reprennent pas la réponse du serveur OAuth.
- Les URL de l'API et du serveur OAuth doivent être en HTTPS.
- En mode HTTP :
  - le serveur refuse de démarrer sans `MCP_AUTH_TOKEN` (32 caractères minimum), sauf si `MCP_ALLOW_UNAUTHENTICATED=true` ;
  - le jeton est comparé en temps constant, via son empreinte SHA-256 ;
  - les requêtes cross-origin sont refusées, et le SDK active la protection contre le DNS rebinding quand le serveur écoute sur une adresse locale ;
  - plusieurs limites protègent la mémoire et le quota PISTE : nombre de requêtes simultanées (`MCP_MAX_CONCURRENCY`), taille des requêtes entrantes (256 Ko), des réponses de l'API (32 Mo) et des arguments des outils ;
  - pour une exposition sur Internet, placer le service derrière un reverse proxy TLS.
- Les instructions envoyées au client MCP précisent que les documents renvoyés sont des données, pas des instructions. Cela limite le risque d'injection de prompt par un contenu tiers, comme un accord d'entreprise.
- L'image finale est basée sur `distroless/static` : pas de shell, utilisateur non root, binaire statique.
- Pistes d'amélioration :
  - épingler les images de base par digest ;
  - signer les images avec cosign et produire un SBOM ;
  - ajouter un cache court des documents, pour que la pagination ne rappelle pas l'API.

## Limites connues

- Le serveur ne couvre pas encore certains endpoints peu courants : débats parlementaires, questions écrites, dossiers législatifs, bulletins officiels des conventions collectives, liens de concordance.
- La consultation d'un code complet par `get_document` sur son `LEGITEXT` peut être très volumineuse ; mieux vaut passer par `get_table_of_contents` puis `get_code_section`.
- Les quotas d'appels de PISTE s'appliquent. Au-delà, l'API renvoie 429 et le serveur réessaie dans la limite de deux nouvelles tentatives.
