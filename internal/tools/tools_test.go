package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"legifrance-mcp/internal/legifrance"
)

// fakeAPI enregistre les appels et renvoie des réponses prédéfinies.
// Les méthodes non surchargées échouent via l'interface embarquée nil.
type fakeAPI struct {
	API
	lastSearch legifrance.SearchRequest
	lastCall   string
	lastLODA   legifrance.LODAListRequest
	article    *legifrance.Article
	text       *legifrance.Text
}

func (f *fakeAPI) Search(_ context.Context, req legifrance.SearchRequest) (*legifrance.SearchResponse, error) {
	f.lastSearch = req
	return &legifrance.SearchResponse{
		TotalResultNumber: 25,
		Results: []legifrance.SearchResult{{
			Titles: []legifrance.SearchTitle{{ID: "LEGITEXT000006070721", Title: "Code <mark>civil</mark>", LegalStatus: "VIGUEUR"}},
			Sections: []legifrance.SearchSection{{Extracts: []legifrance.SearchExtract{{
				ID: "LEGIARTI000032041571", Num: "1240", Values: []string{"Tout fait quelconque de l'<mark>homme</mark>"},
			}}}},
		}},
	}, nil
}

func (f *fakeAPI) GetArticle(_ context.Context, id string) (*legifrance.Article, error) {
	f.lastCall = "GetArticle " + id
	if f.article == nil {
		return nil, legifrance.ErrNotFound
	}
	return f.article, nil
}

func (f *fakeAPI) GetLegiText(_ context.Context, id, date string) (*legifrance.Text, error) {
	f.lastCall = "GetLegiText " + id + " " + date
	return f.text, nil
}

func (f *fakeAPI) GetJuri(_ context.Context, id string) (*legifrance.SimpleText, error) {
	f.lastCall = "GetJuri " + id
	return &legifrance.SimpleText{ID: id, Titre: "Cour de cassation", TexteHTML: "<p>Attendu que…</p>"}, nil
}

func connect(t *testing.T, api API, maxLen int) *mcp.ClientSession {
	t.Helper()
	server := NewServer(api, Options{
		Version:          "test",
		MaxContentLength: maxLen,
		Now:              func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) },
	})
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

func TestListTools(t *testing.T) {
	cs := connect(t, &fakeAPI{}, 0)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"search", "describe_search_options", "suggest", "get_document", "get_article_by_number",
		"get_article_history", "get_table_of_contents", "get_code_section", "get_collective_agreement",
		"get_text_by_nor", "list_codes", "list_texts", "get_latest_journals",
	}
	got := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		got[tool.Name] = tool
	}
	for _, name := range want {
		tool, ok := got[name]
		if !ok {
			t.Errorf("missing tool %s", name)
			continue
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s must be annotated read-only", name)
		}
	}
	if len(res.Tools) != len(want) {
		t.Errorf("got %d tools, want %d", len(res.Tools), len(want))
	}
}

func TestSearchAddsVersionDateAndFormatsResults(t *testing.T) {
	api := &fakeAPI{}
	cs := connect(t, api, 0)

	out, isErr := call(t, cs, "search", map[string]any{
		"query":   "responsabilité",
		"fond":    "CODE_DATE",
		"filters": []map[string]any{{"facet": "nom_code", "values": []string{"Code civil"}}},
	})
	if isErr {
		t.Fatalf("unexpected error: %s", out)
	}
	req := api.lastSearch
	if req.Fond != "CODE_DATE" || req.Recherche.PageSize != defaultPageSize || req.Recherche.Sort != "PERTINENCE" {
		t.Errorf("unexpected request: %+v", req)
	}
	filters, _ := json.Marshal(req.Recherche.Filtres)
	if !strings.Contains(string(filters), `"facette":"NOM_CODE"`) ||
		!strings.Contains(string(filters), `{"facette":"DATE_VERSION","singleDate":"2026-10-16"}`) {
		t.Errorf("unexpected filters: %s", filters)
	}
	for _, want := range []string{"25 results — page 1/3", "Code civil", "`LEGITEXT000006070721`", "Art. 1240", "homme", "page=2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "<mark>") {
		t.Error("HTML must be stripped from the output")
	}
}

func TestSearchRejectsInvalidInput(t *testing.T) {
	cs := connect(t, &fakeAPI{}, 0)
	if _, isErr := call(t, cs, "search", map[string]any{"query": "x", "fond": "NOPE"}); !isErr {
		t.Error("unknown fond must be rejected")
	}
	if _, isErr := call(t, cs, "search", map[string]any{"query": "x", "date": "16/10/2026"}); !isErr {
		t.Error("invalid date must be rejected")
	}
	if _, isErr := call(t, cs, "search", map[string]any{"query": "  "}); !isErr {
		t.Error("empty query must be rejected")
	}
}

func TestGetDocumentDispatchesOnPrefix(t *testing.T) {
	api := &fakeAPI{
		article: &legifrance.Article{ID: "LEGIARTI000032041571", Num: "1240", Etat: "VIGUEUR", TexteHTML: "<p>Tout fait quelconque</p>"},
		text:    &legifrance.Text{ID: "LEGITEXT000006069414", Title: "Loi n° 78-17"},
	}
	cs := connect(t, api, 0)

	cases := []struct {
		args     map[string]any
		wantCall string
		wantText string
	}{
		{map[string]any{"id": "legiarti000032041571"}, "GetArticle LEGIARTI000032041571", "Tout fait quelconque"},
		{map[string]any{"id": "LEGITEXT000006069414", "date": "2020-01-01"}, "GetLegiText LEGITEXT000006069414 2020-01-01", "Loi n° 78-17"},
		{map[string]any{"id": "JURITEXT000007041234"}, "GetJuri JURITEXT000007041234", "Attendu que"},
		{map[string]any{"id": "CETATEXT000007041234"}, "GetJuri CETATEXT000007041234", "Cour de cassation"},
	}
	for _, tc := range cases {
		out, isErr := call(t, cs, "get_document", tc.args)
		if isErr || api.lastCall != tc.wantCall || !strings.Contains(out, tc.wantText) {
			t.Errorf("%v: call=%q isErr=%v out=%s", tc.args, api.lastCall, isErr, out)
		}
	}

	if out, isErr := call(t, cs, "get_document", map[string]any{"id": "LEGISCTA000006136059"}); !isErr || !strings.Contains(out, "get_code_section") {
		t.Errorf("section id must point to get_code_section: %s", out)
	}
	if _, isErr := call(t, cs, "get_document", map[string]any{"id": "../../etc"}); !isErr {
		t.Error("malformed id must be rejected")
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	cs := connect(t, &fakeAPI{}, 0)
	out, isErr := call(t, cs, "get_document", map[string]any{"id": "LEGIARTI000000000001"})
	if !isErr || !strings.Contains(out, "not found") {
		t.Errorf("isErr=%v out=%s", isErr, out)
	}
}

func TestLongContentIsPaginated(t *testing.T) {
	long := strings.Repeat("<p>Alinéa du texte consolidé.</p>", 200)
	api := &fakeAPI{text: &legifrance.Text{ID: "LEGITEXT000006069414", Title: "Long", Articles: []legifrance.TextArticle{{Num: "1", Content: long}}}}
	cs := connect(t, api, 1000)

	out, _ := call(t, cs, "get_document", map[string]any{"id": "LEGITEXT000006069414"})
	if !strings.Contains(out, "Call again with start_index=") {
		t.Fatalf("expected a truncation notice:\n%s", out)
	}
	idx := strings.LastIndex(out, "start_index=")
	var next int
	if _, err := fmt.Sscanf(out[idx:], "start_index=%d", &next); err != nil {
		t.Fatal(err)
	}
	out2, _ := call(t, cs, "get_document", map[string]any{"id": "LEGITEXT000006069414", "start_index": next})
	if !strings.HasPrefix(out2, "[Continuation from character") {
		t.Errorf("second page must start with a continuation marker:\n%s", out2[:80])
	}
}

func TestDescribeSearchOptions(t *testing.T) {
	cs := connect(t, &fakeAPI{}, 0)
	out, isErr := call(t, cs, "describe_search_options", map[string]any{"fond": "JURI"})
	if isErr || !strings.Contains(out, "JURIDICTION_JUDICIAIRE") || !strings.Contains(out, "DATE_DESC") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestEveryFondIsDocumented(t *testing.T) {
	for _, f := range searchFonds {
		if _, ok := fondDocs[f]; !ok {
			t.Errorf("fond %s has no documentation", f)
		}
	}
}

func (f *fakeAPI) GetKaliContainerByIdcc(_ context.Context, idcc string) (*legifrance.KaliContainer, error) {
	f.lastCall = "GetKaliContainerByIdcc " + idcc
	return &legifrance.KaliContainer{ID: "KALICONT000005635221", Titre: "Convention Syntec", Sections: []legifrance.TextSection{
		{Title: "Texte de base", Sections: []legifrance.TextSection{{Title: "Convention collective", ID: "KALITEXT000005644209"}}},
	}}, nil
}

func (f *fakeAPI) ListLODA(_ context.Context, req legifrance.LODAListRequest) (*legifrance.LODAListResponse, error) {
	f.lastCall = "ListLODA"
	f.lastLODA = req
	return &legifrance.LODAListResponse{TotalResultNumber: 0}, nil
}

func TestSearchFilterValidation(t *testing.T) {
	cs := connect(t, &fakeAPI{}, 0)
	bad := []map[string]any{
		{"query": "x", "filters": []map[string]any{{"facet": "NATURE"}}},
		{"query": "x", "filters": []map[string]any{{"facet": "DATE_SIGNATURE", "values": []string{"LOI"}, "start": "2020-01-01"}}},
		{"query": "x", "fond": "JURI", "date": "2020-01-01"},
	}
	for _, args := range bad {
		if out, isErr := call(t, cs, "search", args); !isErr {
			t.Errorf("%v must be rejected, got %s", args, out)
		}
	}
}

func TestArticleHistory(t *testing.T) {
	api := &fakeAPI{article: &legifrance.Article{ID: "LEGIARTI000032041571", Num: "1240", ArticleVersions: []legifrance.ArticleVersion{
		{ID: "LEGIARTI000006438819", Etat: "MODIFIE", Version: "1.0"},
		{ID: "LEGIARTI000032041571", Etat: "VIGUEUR", Version: "2.0"},
	}}}
	cs := connect(t, api, 0)
	out, isErr := call(t, cs, "get_article_history", map[string]any{"article_id": "LEGIARTI000032041571"})
	if isErr || !strings.Contains(out, "LEGIARTI000006438819") || !strings.Contains(out, "MODIFIE") {
		t.Errorf("isErr=%v out=%s", isErr, out)
	}

	api.article = &legifrance.Article{ID: "LEGIARTI000032041571"}
	if _, isErr := call(t, cs, "get_article_history", map[string]any{"article_id": "LEGIARTI000032041571"}); !isErr {
		t.Error("an article without history must be reported as an error")
	}
}

func TestCollectiveAgreementKeepsIdccAsTyped(t *testing.T) {
	api := &fakeAPI{}
	cs := connect(t, api, 0)
	out, isErr := call(t, cs, "get_collective_agreement", map[string]any{"idcc": "0016"})
	if isErr || api.lastCall != "GetKaliContainerByIdcc 0016" || !strings.Contains(out, "KALITEXT000005644209") {
		t.Errorf("call=%q isErr=%v out=%s", api.lastCall, isErr, out)
	}
}

func TestListTextsBuildsPeriods(t *testing.T) {
	api := &fakeAPI{}
	cs := connect(t, api, 0)
	_, isErr := call(t, cs, "list_texts", map[string]any{"natures": []string{"LOI"}, "published_from": "2026-01-01"})
	if isErr || api.lastLODA.PublicationDate == nil || api.lastLODA.PublicationDate.Start != "2026-01-01" ||
		api.lastLODA.SignatureDate != nil || api.lastLODA.Sort != "PUBLICATION_DATE_DESC" {
		t.Errorf("unexpected request: %+v", api.lastLODA)
	}
	if _, isErr := call(t, cs, "list_texts", map[string]any{"natures": []string{"CIRCULAIRE"}}); !isErr {
		t.Error("natures outside the swagger enum must be rejected")
	}
}
