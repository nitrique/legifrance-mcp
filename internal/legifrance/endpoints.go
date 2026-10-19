package legifrance

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// ErrNotFound est renvoyée lorsque l'API répond sans erreur mais sans document.
var ErrNotFound = errors.New("legifrance: document not found")

// Search lance une recherche (/search).
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	return call[SearchResponse](ctx, c, "/search", req)
}

// Suggest renvoie des suggestions de titres (/suggest).
func (c *Client) Suggest(ctx context.Context, req SuggestRequest) (*SuggestResponse, error) {
	return call[SuggestResponse](ctx, c, "/suggest", req)
}

// GetArticle renvoie un article à partir de son identifiant LEGIARTI (/consult/getArticle).
func (c *Client) GetArticle(ctx context.Context, id string) (*Article, error) {
	var out articleResponse
	if err := c.post(ctx, "/consult/getArticle", map[string]string{"id": id}, &out); err != nil {
		return nil, err
	}
	return requireDoc(out.Article)
}

// GetArticleByNumber renvoie l'article en vigueur portant le numéro donné dans
// un texte LEGITEXT (/consult/getArticleWithIdAndNum).
func (c *Client) GetArticleByNumber(ctx context.Context, textID, num string) (*Article, error) {
	var out articleResponse
	body := map[string]string{"id": textID, "num": num}
	if err := c.post(ctx, "/consult/getArticleWithIdAndNum", body, &out); err != nil {
		return nil, err
	}
	return requireDoc(out.Article)
}

// GetArticleVersions renvoie toutes les versions d'un article à partir de son
// identifiant chronique (/consult/getArticleByCid).
func (c *Client) GetArticleVersions(ctx context.Context, cid string) ([]Article, error) {
	var out articleListResponse
	if err := c.post(ctx, "/consult/getArticleByCid", map[string]string{"cid": cid}, &out); err != nil {
		return nil, err
	}
	if len(out.ListArticle) == 0 {
		return nil, ErrNotFound
	}
	return out.ListArticle, nil
}

// GetTableOfContents renvoie la table des matières d'un code ou d'un texte
// LODA à une date donnée (/consult/legi/tableMatieres).
func (c *Client) GetTableOfContents(ctx context.Context, textID, date string) (*Text, error) {
	var out Text
	body := map[string]string{"textId": textID, "date": date}
	if err := c.post(ctx, "/consult/legi/tableMatieres", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCodeSection renvoie le contenu d'une section de code à une date donnée (/consult/code).
func (c *Client) GetCodeSection(ctx context.Context, textID, sectionCid, date string) (*Text, error) {
	var out Text
	body := map[string]string{"textId": textID, "sctCid": sectionCid, "date": date}
	if err := c.post(ctx, "/consult/code", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetLegiText renvoie le contenu d'un texte LODA (loi, décret, ordonnance…)
// dans sa version en vigueur à une date donnée (/consult/lawDecree).
func (c *Client) GetLegiText(ctx context.Context, textID, date string) (*Text, error) {
	var out Text
	body := map[string]string{"textId": textID, "date": date}
	if err := c.post(ctx, "/consult/lawDecree", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetJorfText renvoie un texte publié au Journal officiel (/consult/jorf).
func (c *Client) GetJorfText(ctx context.Context, textCid string) (*Text, error) {
	return call[Text](ctx, c, "/consult/jorf", map[string]string{"textCid": textCid})
}

// GetJorfTextByNor renvoie un texte du Journal officiel à partir de son NOR (/consult/getJoWithNor).
func (c *Client) GetJorfTextByNor(ctx context.Context, nor string) (*Text, error) {
	return call[Text](ctx, c, "/consult/getJoWithNor", map[string]string{"nor": nor})
}

// GetKaliText renvoie un texte de convention collective (/consult/kaliText).
func (c *Client) GetKaliText(ctx context.Context, id string) (*Text, error) {
	return call[Text](ctx, c, "/consult/kaliText", map[string]string{"id": id})
}

// GetKaliContainer renvoie le conteneur d'une convention collective à partir
// de son identifiant KALICONT (/consult/kaliCont).
func (c *Client) GetKaliContainer(ctx context.Context, id string) (*KaliContainer, error) {
	return call[KaliContainer](ctx, c, "/consult/kaliCont", map[string]string{"id": id})
}

// GetKaliContainerByIdcc renvoie le conteneur d'une convention collective à
// partir de son numéro IDCC (/consult/kaliContIdcc).
func (c *Client) GetKaliContainerByIdcc(ctx context.Context, idcc string) (*KaliContainer, error) {
	return call[KaliContainer](ctx, c, "/consult/kaliContIdcc", map[string]string{"id": idcc})
}

// GetJuri renvoie une décision de jurisprudence (judiciaire, administrative,
// constitutionnelle ou financière) (/consult/juri).
func (c *Client) GetJuri(ctx context.Context, textID string) (*SimpleText, error) {
	var out simpleTextResponse
	if err := c.post(ctx, "/consult/juri", map[string]string{"textId": textID}, &out); err != nil {
		return nil, err
	}
	return requireDoc(out.Text)
}

// GetCnil renvoie une délibération de la CNIL (/consult/cnil).
func (c *Client) GetCnil(ctx context.Context, textID string) (*SimpleText, error) {
	var out simpleTextResponse
	if err := c.post(ctx, "/consult/cnil", map[string]string{"textId": textID}, &out); err != nil {
		return nil, err
	}
	return requireDoc(out.Text)
}

// GetAgreement renvoie un accord d'entreprise (/consult/acco).
func (c *Client) GetAgreement(ctx context.Context, id string) (*Agreement, error) {
	var out agreementResponse
	if err := c.post(ctx, "/consult/acco", map[string]string{"id": id}, &out); err != nil {
		return nil, err
	}
	return requireDoc(out.Acco)
}

// GetCircular renvoie une circulaire ou une instruction (/consult/circulaire).
func (c *Client) GetCircular(ctx context.Context, id string) (*Circular, error) {
	var out circularResponse
	if err := c.post(ctx, "/consult/circulaire", map[string]string{"id": id}, &out); err != nil {
		return nil, err
	}
	return requireDoc(out.Circulaire)
}

// ListCodes renvoie la liste paginée des codes (/list/code).
func (c *Client) ListCodes(ctx context.Context, req CodeListRequest) (*CodeListResponse, error) {
	return call[CodeListResponse](ctx, c, "/list/code", req)
}

// ListLODA renvoie la liste paginée des lois, ordonnances, décrets et arrêtés (/list/loda).
func (c *Client) ListLODA(ctx context.Context, req LODAListRequest) (*LODAListResponse, error) {
	return call[LODAListResponse](ctx, c, "/list/loda", req)
}

// LatestJournals renvoie les n derniers numéros du Journal officiel (/consult/lastNJo).
func (c *Client) LatestJournals(ctx context.Context, n int) ([]JournalContainer, error) {
	var out journalContainersResponse
	if err := c.post(ctx, "/consult/lastNJo", map[string]int{"nbElement": n}, &out); err != nil {
		return nil, err
	}
	return out.Containers, nil
}

// Ping vérifie la disponibilité de l'API et la validité des identifiants (/search/ping).
func (c *Client) Ping(ctx context.Context) error {
	if err := c.do(ctx, http.MethodGet, "/search/ping", nil, nil); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}

// call envoie la requête et décode la réponse dans une nouvelle valeur de type T.
func call[T any](ctx context.Context, c *Client, path string, body any) (*T, error) {
	var out T
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func requireDoc[T any](doc *T) (*T, error) {
	if doc == nil {
		return nil, ErrNotFound
	}
	return doc, nil
}
