package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"legifrance-mcp/internal/legifrance"
	"legifrance-mcp/internal/textutil"
)

var (
	// Valeurs reprises des énumérations du Swagger (LODAListRequest, CodeListRequest).
	legalStatuses = []string{
		"VIGUEUR", "ABROGE_DIFF", "VIGUEUR_DIFF", "VIGUEUR_ETEN", "VIGUEUR_NON_ETEN", "ABROGE", "PERIME",
		"ANNULE", "MODIFIE", "DISJOINT", "SUBSTITUE", "TRANSFERE", "INITIALE", "MODIFIE_MORT_NE",
		"SANS_ETAT", "DENONCE", "REMPLACE",
	}
	lodaNatures = []string{
		"LOI", "ORDONNANCE", "DECRET", "DECRET_LOI", "ARRETE", "CONSTITUTION", "DECISION",
		"CONVENTION", "DECLARATION", "ACCORD_FONCTION_PUBLIQUE",
	}
	lodaSorts = []string{"PUBLICATION_DATE_DESC", "PUBLICATION_DATE_ASC", "SIGNATURE_DATE_DESC", "SIGNATURE_DATE_ASC"}
)

// ListCodesInput décrit les paramètres de l'outil « list_codes ».
type ListCodesInput struct {
	Name     string   `json:"name,omitempty" jsonschema:"part of the code title, e.g. \"travail\" or \"Code civil\""`
	States   []string `json:"states,omitempty" jsonschema:"legal statuses to keep (default VIGUEUR)"`
	Page     int      `json:"page,omitempty" jsonschema:"page number starting at 1 (default 1)"`
	PageSize int      `json:"page_size,omitempty" jsonschema:"results per page (default 20, max 100)"`
}

// ListTextsInput décrit les paramètres de l'outil « list_texts ».
type ListTextsInput struct {
	Natures       []string `json:"natures,omitempty" jsonschema:"text natures to keep, e.g. [\"LOI\", \"ORDONNANCE\"]"`
	States        []string `json:"states,omitempty" jsonschema:"legal statuses to keep"`
	SignedFrom    string   `json:"signed_from,omitempty" jsonschema:"signature date lower bound YYYY-MM-DD"`
	SignedTo      string   `json:"signed_to,omitempty" jsonschema:"signature date upper bound YYYY-MM-DD"`
	PublishedFrom string   `json:"published_from,omitempty" jsonschema:"publication date lower bound YYYY-MM-DD"`
	PublishedTo   string   `json:"published_to,omitempty" jsonschema:"publication date upper bound YYYY-MM-DD"`
	Sort          string   `json:"sort,omitempty" jsonschema:"sort order (default PUBLICATION_DATE_DESC)"`
	Page          int      `json:"page,omitempty" jsonschema:"page number starting at 1 (default 1)"`
	PageSize      int      `json:"page_size,omitempty" jsonschema:"results per page (default 20, max 100)"`
}

// LatestJournalsInput décrit les paramètres de l'outil « get_latest_journals ».
type LatestJournalsInput struct {
	Count int `json:"count,omitempty" jsonschema:"number of issues to return (default 5, max 50)"`
}

func (h *handler) registerListTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_codes",
		Description: "Lists French codes (Code civil, Code du travail…) with their LEGITEXT identifiers, optionally filtered by title.",
		Annotations: readOnly("List codes"),
		InputSchema: schemaFor[ListCodesInput](
			withMaxLength(200, "name"), withEnum(legalStatuses, "states"), withRange(1, 10000, "page"), withRange(1, maxPageSize, "page_size")),
	}, h.listCodes)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_texts",
		Description: "Lists laws, ordinances, decrees and orders (LODA) by nature and signature or publication period, e.g. the laws published last month.",
		Annotations: readOnly("List laws and decrees"),
		InputSchema: schemaFor[ListTextsInput](
			withEnum(lodaNatures, "natures"), withEnum(legalStatuses, "states"), withEnum(lodaSorts, "sort"),
			withPattern(datePattern, "signed_from"), withPattern(datePattern, "signed_to"),
			withPattern(datePattern, "published_from"), withPattern(datePattern, "published_to"),
			withRange(1, 10000, "page"), withRange(1, maxPageSize, "page_size")),
	}, h.listTexts)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_latest_journals",
		Description: "Lists the latest issues of the Journal officiel (JORF) with their identifiers and publication dates.",
		Annotations: readOnly("Latest Journal officiel issues"),
		InputSchema: schemaFor[LatestJournalsInput](withRange(1, 50, "count")),
	}, h.latestJournals)
}

func pagination(page, size int) (int, int) {
	if size <= 0 {
		size = 20
	}
	return max(page, 1), min(size, maxPageSize)
}

func (h *handler) listCodes(ctx context.Context, _ *mcp.CallToolRequest, in ListCodesInput) (*mcp.CallToolResult, any, error) {
	page, size := pagination(in.Page, in.PageSize)
	states := in.States
	if len(states) == 0 {
		states = []string{"VIGUEUR"}
	}
	resp, err := h.api.ListCodes(ctx, legifrance.CodeListRequest{
		CodeName:   strings.TrimSpace(in.Name),
		States:     states,
		PageNumber: page,
		PageSize:   size,
		Sort:       "TITLE_ASC",
	})
	if err != nil {
		return nil, nil, apiError(err, "codes")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Codes (%d results, page %d)\n\n", resp.TotalResultNumber, page)
	b.WriteString("| Code | ID | Status | Last update |\n|---|---|---|---|\n")
	for _, c := range resp.Results {
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", escapeCell(c.Titre), firstNonEmpty(c.Cid, c.ID), c.Etat, c.LastUpdate)
	}
	writeMorePages(&b, resp.TotalResultNumber, page, size)
	return textResult(b.String()), nil, nil
}

func (h *handler) listTexts(ctx context.Context, _ *mcp.CallToolRequest, in ListTextsInput) (*mcp.CallToolResult, any, error) {
	page, size := pagination(in.Page, in.PageSize)
	for _, d := range []string{in.SignedFrom, in.SignedTo, in.PublishedFrom, in.PublishedTo} {
		if d != "" {
			if err := validateDate(d); err != nil {
				return nil, nil, err
			}
		}
	}
	req := legifrance.LODAListRequest{
		Natures:     in.Natures,
		LegalStatus: in.States,
		Sort:        upperOr(in.Sort, "PUBLICATION_DATE_DESC"),
		PageNumber:  page,
		PageSize:    size,
	}
	if in.SignedFrom != "" || in.SignedTo != "" {
		req.SignatureDate = &legifrance.DatePeriod{Start: in.SignedFrom, End: in.SignedTo}
	}
	if in.PublishedFrom != "" || in.PublishedTo != "" {
		req.PublicationDate = &legifrance.DatePeriod{Start: in.PublishedFrom, End: in.PublishedTo}
	}
	resp, err := h.api.ListLODA(ctx, req)
	if err != nil {
		return nil, nil, apiError(err, "texts")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Laws and regulations (%d results, page %d)\n\n", resp.TotalResultNumber, page)
	b.WriteString("| Title | ID | Status | Validity |\n|---|---|---|---|\n")
	for _, t := range resp.Results {
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", escapeCell(t.Titre), firstNonEmpty(t.Cid, t.ID), t.Etat, validity(t.DateDebut, t.DateFin))
	}
	writeMorePages(&b, resp.TotalResultNumber, page, size)
	return textResult(b.String()), nil, nil
}

func (h *handler) latestJournals(ctx context.Context, _ *mcp.CallToolRequest, in LatestJournalsInput) (*mcp.CallToolResult, any, error) {
	count := in.Count
	if count <= 0 {
		count = 5
	}
	count = min(count, 50)
	journals, err := h.api.LatestJournals(ctx, count)
	if err != nil {
		return nil, nil, apiError(err, "latest Journal officiel issues")
	}
	var b strings.Builder
	b.WriteString("# Latest Journal officiel issues\n\n")
	if len(journals) == 0 {
		b.WriteString("No issue returned.\n")
	}
	for _, j := range journals {
		fmt.Fprintf(&b, "- %s", firstNonEmpty(textutil.HTMLToText(j.Titre), "JORF n° "+string(j.Num)))
		if !j.DatePubli.IsZero() {
			fmt.Fprintf(&b, " — %s", j.DatePubli)
		}
		fmt.Fprintf(&b, " (`%s`)\n", j.ID)
	}
	b.WriteString("\nFind the texts of an issue with search (fond JORF, filter DATE_PUBLICATION).\n")
	return textResult(b.String()), nil, nil
}

func writeMorePages(b *strings.Builder, total, page, size int) {
	if page*size < total {
		fmt.Fprintf(b, "\nMore results available: call again with page=%d.\n", page+1)
	}
}

// escapeCell neutralise les caractères qui casseraient un tableau Markdown.
func escapeCell(s string) string {
	s = textutil.HTMLToText(s)
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}
