package tools

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"legifrance-mcp/internal/legifrance"
	"legifrance-mcp/internal/textutil"
)

const (
	defaultPageSize   = 10
	maxPageSize       = 100
	maxExtractsPerHit = 3
	maxExtractLength  = 400
	maxQueryLength    = 500
)

// SearchInput décrit les paramètres de l'outil « search ».
type SearchInput struct {
	Query    string        `json:"query" jsonschema:"words or expression to search for"`
	Fond     string        `json:"fond,omitempty" jsonschema:"collection to search (default ALL). CODE_DATE/LODA_DATE search the version in force at 'date'; CODE_ETAT/LODA_ETAT filter by legal status"`
	Field    string        `json:"field,omitempty" jsonschema:"field to search in (default ALL). Allowed fields depend on the collection, see describe_search_options"`
	Match    string        `json:"match,omitempty" jsonschema:"how words are matched (default TOUS_LES_MOTS_DANS_UN_CHAMP = all words; EXACTE = exact phrase; UN_DES_MOTS = any word)"`
	Filters  []FilterInput `json:"filters,omitempty" jsonschema:"facet filters combined with AND, see describe_search_options for the facets of each collection"`
	Sort     string        `json:"sort,omitempty" jsonschema:"sort order (default PERTINENCE), see describe_search_options"`
	Date     string        `json:"date,omitempty" jsonschema:"version date YYYY-MM-DD for CODE_DATE and LODA_DATE (default today)"`
	Page     int           `json:"page,omitempty" jsonschema:"page number starting at 1 (default 1)"`
	PageSize int           `json:"page_size,omitempty" jsonschema:"results per page (default 10, max 100)"`
}

// FilterInput décrit un filtre de recherche (facette).
type FilterInput struct {
	Facet  string   `json:"facet" jsonschema:"facet name, e.g. NATURE, DATE_SIGNATURE, JURIDICTION_JUDICIAIRE"`
	Values []string `json:"values,omitempty" jsonschema:"accepted values for a value facet, e.g. [\"LOI\", \"DECRET\"]"`
	Start  string   `json:"start,omitempty" jsonschema:"period start YYYY-MM-DD for a date facet"`
	End    string   `json:"end,omitempty" jsonschema:"period end YYYY-MM-DD for a date facet"`
	Date   string   `json:"date,omitempty" jsonschema:"single date YYYY-MM-DD for a single-date facet such as DATE_VERSION"`
}

// SearchOptionsInput décrit les paramètres de l'outil « describe_search_options ».
type SearchOptionsInput struct {
	Fond string `json:"fond" jsonschema:"collection to describe"`
}

// SuggestInput décrit les paramètres de l'outil « suggest ».
type SuggestInput struct {
	Text  string   `json:"text" jsonschema:"beginning of a title, a common name (e.g. \"loi Macron\") or an article reference"`
	Fonds []string `json:"fonds,omitempty" jsonschema:"collections to look into (default all), e.g. [\"LODA_LIST\", \"CODE_LIST\", \"JURI\"]"`
}

func (h *handler) registerSearchTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "search",
		Description: "Full-text search in Légifrance: codes, laws and regulations (LODA), Journal officiel, " +
			"case law (JURI, CETAT, CONSTIT, JUFI), collective and company agreements (KALI, ACCO), CNIL, circulars. " +
			"Returns titles, identifiers, legal status and matching excerpts. Use describe_search_options to " +
			"learn the fields, filters and sort orders available for a collection.",
		Annotations: readOnly("Search Légifrance"),
		InputSchema: schemaFor[SearchInput](
			withEnum(searchFonds, "fond"),
			withEnum(searchFieldTypes, "field"),
			withEnum(searchMatchTypes, "match"),
			withPattern(datePattern, "date"),
			withPattern(datePattern, "filters", "start"),
			withPattern(datePattern, "filters", "end"),
			withPattern(datePattern, "filters", "date"),
			withRange(1, 10000, "page"),
			withRange(1, maxPageSize, "page_size"),
			withMaxLength(maxQueryLength, "query"),
			withMaxLength(100, "sort"),
			withMaxItems(20, "filters"),
			withMaxLength(100, "filters", "facet"),
			withMaxItems(50, "filters", "values"),
			withMaxLength(200, "filters", "values"),
		),
	}, h.search)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "describe_search_options",
		Description: "Lists the search fields, filter facets and sort orders accepted by the 'search' tool for a given collection.",
		Annotations: &mcp.ToolAnnotations{Title: "Describe search options", ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: schemaFor[SearchOptionsInput](withEnum(searchFonds, "fond")),
	}, h.describeSearchOptions)

	mcp.AddTool(s, &mcp.Tool{
		Name: "suggest",
		Description: "Autocompletes document titles from a few words (e.g. \"code du travail\", \"loi Macron\"). " +
			"Useful to quickly find the identifier of a well-known text.",
		Annotations: readOnly("Suggest titles"),
		InputSchema: schemaFor[SuggestInput](
			withMaxLength(200, "text"), withMaxItems(len(suggestSupplies), "fonds"), withEnum(suggestSupplies, "fonds")),
	}, h.suggest)
}

func (h *handler) search(ctx context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, any, error) {
	req, err := h.buildSearchRequest(in)
	if err != nil {
		return nil, nil, err
	}
	resp, err := h.api.Search(ctx, req)
	if err != nil {
		return nil, nil, apiError(err, "search results")
	}
	return textResult(formatSearch(strings.TrimSpace(in.Query), req, resp)), nil, nil
}

// buildSearchRequest traduit les paramètres simplifiés de l'outil en SearchRequestDTO.
func (h *handler) buildSearchRequest(in SearchInput) (legifrance.SearchRequest, error) {
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return legifrance.SearchRequest{}, fmt.Errorf("query must not be empty")
	}
	if len([]rune(query)) > maxQueryLength {
		return legifrance.SearchRequest{}, fmt.Errorf("query must not exceed %d characters", maxQueryLength)
	}
	fond := upperOr(in.Fond, "ALL")
	if !slices.Contains(searchFonds, fond) {
		return legifrance.SearchRequest{}, fmt.Errorf("unknown fond %q", in.Fond)
	}
	field := upperOr(in.Field, "ALL")
	match := upperOr(in.Match, "TOUS_LES_MOTS_DANS_UN_CHAMP")
	page := max(in.Page, 1)
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	pageSize = min(pageSize, maxPageSize)

	filters, err := buildFilters(in.Filters)
	if err != nil {
		return legifrance.SearchRequest{}, err
	}
	if in.Date != "" && !strings.HasSuffix(fond, "_DATE") {
		return legifrance.SearchRequest{}, fmt.Errorf("date only applies to CODE_DATE and LODA_DATE; use a date filter for %s", fond)
	}
	// Les fonds « _DATE » portent sur la version en vigueur à une date :
	// sans filtre DATE_VERSION explicite, la date du jour (ou « date ») est utilisée.
	if strings.HasSuffix(fond, "_DATE") && !slices.ContainsFunc(filters, func(f legifrance.SearchFilter) bool {
		return f.Facette == "DATE_VERSION"
	}) {
		date, err := h.dateOrToday(in.Date)
		if err != nil {
			return legifrance.SearchRequest{}, err
		}
		filters = append(filters, legifrance.SearchFilter{Facette: "DATE_VERSION", SingleDate: date})
	}

	return legifrance.SearchRequest{
		Fond: fond,
		Recherche: legifrance.SearchCriteria{
			Champs: []legifrance.SearchField{{
				TypeChamp: field,
				Operateur: "ET",
				Criteres: []legifrance.SearchCriterion{{
					Valeur:        query,
					TypeRecherche: match,
					Operateur:     "ET",
				}},
			}},
			Filtres:        filters,
			Operateur:      "ET",
			PageNumber:     page,
			PageSize:       pageSize,
			Sort:           upperOr(in.Sort, "PERTINENCE"),
			TypePagination: "DEFAUT",
		},
	}, nil
}

func buildFilters(in []FilterInput) ([]legifrance.SearchFilter, error) {
	out := make([]legifrance.SearchFilter, 0, len(in)+1)
	for _, f := range in {
		facet := strings.ToUpper(strings.TrimSpace(f.Facet))
		if facet == "" {
			return nil, fmt.Errorf("filter facet must not be empty")
		}
		filter := legifrance.SearchFilter{Facette: facet}
		for _, d := range []string{f.Start, f.End, f.Date} {
			if d != "" {
				if err := validateDate(d); err != nil {
					return nil, fmt.Errorf("filter %s: %w", facet, err)
				}
			}
		}
		kinds := 0
		for _, set := range []bool{f.Date != "", f.Start != "" || f.End != "", len(f.Values) > 0} {
			if set {
				kinds++
			}
		}
		if kinds != 1 {
			return nil, fmt.Errorf("filter %s needs exactly one of values, date or start/end", facet)
		}
		switch {
		case f.Date != "":
			filter.SingleDate = f.Date
		case len(f.Values) > 0:
			filter.Valeurs = f.Values
		default:
			filter.Dates = &legifrance.DatePeriod{Start: f.Start, End: f.End}
		}
		out = append(out, filter)
	}
	return out, nil
}

func formatSearch(query string, req legifrance.SearchRequest, resp *legifrance.SearchResponse) string {
	var b strings.Builder
	crit := req.Recherche
	fmt.Fprintf(&b, "# Search \"%s\" in %s\n\n", query, req.Fond)
	pages := (resp.TotalResultNumber + crit.PageSize - 1) / crit.PageSize
	if len(resp.Results) == 0 {
		if resp.TotalResultNumber > 0 {
			fmt.Fprintf(&b, "Page %d is beyond the last page (%d results, %d pages).\n", crit.PageNumber, resp.TotalResultNumber, pages)
		} else {
			b.WriteString("No result. Try fewer words, another match type (UN_DES_MOTS) or another collection.\n")
		}
		return b.String()
	}
	fmt.Fprintf(&b, "%d results — page %d/%d.\n", resp.TotalResultNumber, crit.PageNumber, max(pages, 1))

	for i, r := range resp.Results {
		rank := (crit.PageNumber-1)*crit.PageSize + i + 1
		writeSearchResult(&b, rank, r)
	}
	if crit.PageNumber < pages {
		fmt.Fprintf(&b, "\nMore results available: call again with page=%d.\n", crit.PageNumber+1)
	}
	return b.String()
}

func writeSearchResult(b *strings.Builder, rank int, r legifrance.SearchResult) {
	title, id := "(untitled)", ""
	var status string
	if len(r.Titles) > 0 {
		t := r.Titles[0]
		title = textutil.HTMLToText(t.Title)
		id = firstNonEmpty(t.ID, t.Cid)
		status = t.LegalStatus
	}
	fmt.Fprintf(b, "\n## %d. %s\n", rank, title)

	meta := []string{}
	addMeta := func(label, value string) {
		if value != "" {
			meta = append(meta, label+": "+value)
		}
	}
	if id != "" {
		addMeta("ID", "`"+id+"`")
	}
	addMeta("Nature", r.Nature)
	addMeta("Status", firstNonEmpty(r.Etat, status))
	addMeta("Date", firstNonEmpty(r.DateSignature.String(), r.Date.String(), r.DatePublication.String()))
	addMeta("NOR", r.Nor)
	addMeta("Number", string(r.Num))
	addMeta("IDCC", string(r.Idcc))
	addMeta("Company", r.RaisonSociale)
	if len(meta) > 0 {
		fmt.Fprintf(b, "%s\n", strings.Join(meta, " · "))
	}

	shown := 0
	for _, sec := range r.Sections {
		for _, ex := range sec.Extracts {
			if shown >= maxExtractsPerHit {
				break
			}
			text := textutil.Truncate(textutil.HTMLToText(strings.Join(ex.Values, " … ")), maxExtractLength)
			if text == "" {
				continue
			}
			label := "Excerpt"
			if ex.Num != "" {
				label = "Art. " + string(ex.Num)
			}
			if ex.ID != "" {
				label += " (`" + ex.ID + "`)"
			}
			if ex.LegalStatus != "" {
				label += " [" + ex.LegalStatus + "]"
			}
			fmt.Fprintf(b, "- %s: %s\n", label, text)
			shown++
		}
	}
	if shown == 0 {
		summary := r.Text
		if summary == "" && len(r.ResumePrincipal) > 0 {
			summary = r.ResumePrincipal[0]
		}
		if summary = textutil.Truncate(textutil.HTMLToText(summary), maxExtractLength); summary != "" {
			fmt.Fprintf(b, "> %s\n", summary)
		}
	}
}

func (h *handler) describeSearchOptions(_ context.Context, _ *mcp.CallToolRequest, in SearchOptionsInput) (*mcp.CallToolResult, any, error) {
	fond := strings.ToUpper(strings.TrimSpace(in.Fond))
	doc, ok := fondDocs[fond]
	if !ok {
		return nil, nil, fmt.Errorf("unknown fond %q, expected one of %s", in.Fond, strings.Join(searchFonds, ", "))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Search options for %s (%s)\n\n", fond, doc.Label)
	fmt.Fprintf(&b, "**Fields** (`field`): %s\n\n", strings.Join(doc.Fields, ", "))
	b.WriteString("**Filters** (`filters[].facet`):\n")
	for _, f := range doc.Filters {
		usage := "`values`"
		switch f.Kind {
		case kindPeriod:
			usage = "`start`/`end`"
		case kindDate:
			usage = "`date`"
		}
		fmt.Fprintf(&b, "- `%s` — %s (use %s)\n", f.Name, f.Label, usage)
	}
	fmt.Fprintf(&b, "\n**Sort orders** (`sort`): %s\n", strings.Join(doc.Sorts, ", "))
	b.WriteString("\nLegal status values: VIGUEUR (in force), ABROGE (repealed), VIGUEUR_DIFF (deferred entry into force), MODIFIE, PERIME, ANNULE.\n")
	return textResult(b.String()), nil, nil
}

func (h *handler) suggest(ctx context.Context, _ *mcp.CallToolRequest, in SuggestInput) (*mcp.CallToolResult, any, error) {
	text := strings.TrimSpace(in.Text)
	if len([]rune(text)) < 2 {
		return nil, nil, fmt.Errorf("text must contain at least 2 characters")
	}
	fonds := make([]string, 0, len(in.Fonds))
	for _, f := range in.Fonds {
		f = strings.ToUpper(strings.TrimSpace(f))
		if !slices.Contains(suggestSupplies, f) {
			return nil, nil, fmt.Errorf("unknown fonds value %q, expected one of %s", f, strings.Join(suggestSupplies, ", "))
		}
		fonds = append(fonds, f)
	}
	resp, err := h.api.Suggest(ctx, legifrance.SuggestRequest{SearchText: text, Supplies: fonds})
	if err != nil {
		return nil, nil, apiError(err, "suggestions")
	}
	values := resp.Values()
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].Origin != values[j].Origin {
			return values[i].Origin < values[j].Origin
		}
		return values[i].Label < values[j].Label
	})

	var b strings.Builder
	fmt.Fprintf(&b, "# Suggestions for \"%s\"\n\n", text)
	if len(values) == 0 {
		b.WriteString("No suggestion. Try the 'search' tool instead.\n")
	}
	for _, v := range values {
		line := "- " + textutil.HTMLToText(v.Label)
		if v.ID != "" {
			line += " — `" + v.ID + "`"
		}
		var extra []string
		if v.Origin != "" {
			extra = append(extra, v.Origin)
		}
		if v.Nature != "" {
			extra = append(extra, v.Nature)
		}
		if v.IDTexte != "" && v.IDTexte != v.ID {
			extra = append(extra, "text `"+v.IDTexte+"`")
		}
		if len(v.Appellations) > 0 {
			extra = append(extra, "aka "+strings.Join(v.Appellations, ", "))
		}
		if len(extra) > 0 {
			line += " (" + strings.Join(extra, ", ") + ")"
		}
		fmt.Fprintf(&b, "%s\n", line)
	}
	return textResult(b.String()), nil, nil
}

func upperOr(v, fallback string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	if v == "" {
		return fallback
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
