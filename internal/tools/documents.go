package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetDocumentInput décrit les paramètres de l'outil « get_document ».
type GetDocumentInput struct {
	ID   string `json:"id" jsonschema:"Légifrance identifier (LEGIARTI, LEGITEXT, JORFTEXT, JURITEXT, CETATEXT, CONSTEXT, JUFITEXT, KALITEXT, KALICONT, ACCOTEXT, CNILTEXT) or numeric circular identifier"`
	Date string `json:"date,omitempty" jsonschema:"version date YYYY-MM-DD for LEGITEXT texts (default today)"`
	PageInput
}

// ArticleByNumberInput décrit les paramètres de l'outil « get_article_by_number ».
type ArticleByNumberInput struct {
	TextID string `json:"text_id" jsonschema:"LEGITEXT identifier of the code or text, e.g. LEGITEXT000006070721 for the Code civil"`
	Number string `json:"number" jsonschema:"article number as written in the code, e.g. \"1240\", \"L1234-5\", \"R4127-1\""`
	PageInput
}

// ArticleHistoryInput décrit les paramètres de l'outil « get_article_history ».
type ArticleHistoryInput struct {
	ArticleID string `json:"article_id" jsonschema:"LEGIARTI identifier of any version of the article"`
}

// TableOfContentsInput décrit les paramètres de l'outil « get_table_of_contents ».
type TableOfContentsInput struct {
	TextID string `json:"text_id" jsonschema:"LEGITEXT identifier of a code or a consolidated text"`
	Date   string `json:"date,omitempty" jsonschema:"version date YYYY-MM-DD (default today)"`
	Depth  int    `json:"depth,omitempty" jsonschema:"maximum depth of sections to display (default 3, 0 = unlimited)"`
	PageInput
}

// CodeSectionInput décrit les paramètres de l'outil « get_code_section ».
type CodeSectionInput struct {
	TextID    string `json:"text_id" jsonschema:"LEGITEXT identifier of the code"`
	SectionID string `json:"section_id" jsonschema:"LEGISCTA identifier of the section, from get_table_of_contents"`
	Date      string `json:"date,omitempty" jsonschema:"version date YYYY-MM-DD (default today)"`
	PageInput
}

// CollectiveAgreementInput décrit les paramètres de l'outil « get_collective_agreement ».
type CollectiveAgreementInput struct {
	Idcc string `json:"idcc" jsonschema:"IDCC number of the collective agreement, e.g. \"1486\" (Syntec) or \"3248\" (métallurgie)"`
	PageInput
}

// NorInput décrit les paramètres de l'outil « get_text_by_nor ».
type NorInput struct {
	Nor string `json:"nor" jsonschema:"NOR identifier of a text published in the Journal officiel, e.g. ECOX1515436L"`
	PageInput
}

// Motifs partagés entre les schémas d'entrée et la validation côté serveur.
const (
	idccSchemaPattern = `^\d{1,5}$`
	norSchemaPattern  = `^[A-Za-z]{4}\d{7}[A-Za-z]$`
)

var (
	idccPattern     = regexp.MustCompile(idccSchemaPattern)
	norPattern      = regexp.MustCompile(`^[A-Z]{4}\d{7}[A-Z]$`)
	circularPattern = regexp.MustCompile(`^\d{1,12}$`)
)

func pageSchema(s *jsonschema.Schema) {
	withRange(0, 1e9, "start_index")(s)
	withRange(1000, 200000, "max_length")(s)
}

func (h *handler) registerDocumentTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "get_document",
		Description: "Reads a Légifrance document from its identifier: article (LEGIARTI…), consolidated law or decree " +
			"(LEGITEXT…, version at 'date'), Journal officiel text (JORFTEXT…), court decision (JURITEXT, CETATEXT, " +
			"CONSTEXT, JUFITEXT), collective agreement text or container (KALITEXT, KALICONT), company agreement " +
			"(ACCOTEXT), CNIL deliberation (CNILTEXT) or circular (numeric id). For whole codes, prefer " +
			"get_table_of_contents + get_code_section.",
		Annotations: readOnly("Read a document"),
		InputSchema: schemaFor[GetDocumentInput](withMaxLength(40, "id"), withPattern(datePattern, "date"), pageSchema),
	}, h.getDocument)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_article_by_number",
		Description: "Returns the article currently in force with the given number in a code or consolidated text (e.g. Code civil, article 1240).",
		Annotations: readOnly("Article by number"),
		InputSchema: schemaFor[ArticleByNumberInput](
			withPattern(idSchemaPattern, "text_id"), withMaxLength(50, "number"), pageSchema),
	}, h.getArticleByNumber)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_article_history",
		Description: "Lists every version of an article with its validity period and legal status, to know which wording applied at a given date.",
		Annotations: readOnly("Article history"),
		InputSchema: schemaFor[ArticleHistoryInput](withPattern(idSchemaPattern, "article_id")),
	}, h.getArticleHistory)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_table_of_contents",
		Description: "Returns the structure (books, titles, chapters, sections and article numbers with identifiers) of a code or consolidated text at a given date.",
		Annotations: readOnly("Table of contents"),
		InputSchema: schemaFor[TableOfContentsInput](
			withPattern(idSchemaPattern, "text_id"), withPattern(datePattern, "date"), withRange(0, 20, "depth"), pageSchema),
	}, h.getTableOfContents)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_code_section",
		Description: "Returns the full content of the articles of a code section (LEGISCTA…) at a given date.",
		Annotations: readOnly("Code section"),
		InputSchema: schemaFor[CodeSectionInput](
			withPattern(idSchemaPattern, "text_id"), withPattern(idSchemaPattern, "section_id"), withPattern(datePattern, "date"), pageSchema),
	}, h.getCodeSection)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_collective_agreement",
		Description: "Returns a collective agreement (convention collective) from its IDCC number: title and the list of its base text, amendments and attached texts with their identifiers (read them with get_document).",
		Annotations: readOnly("Collective agreement"),
		InputSchema: schemaFor[CollectiveAgreementInput](withPattern(idccSchemaPattern, "idcc"), pageSchema),
	}, h.getCollectiveAgreement)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_text_by_nor",
		Description: "Returns a text published in the Journal officiel from its NOR identifier.",
		Annotations: readOnly("Text by NOR"),
		InputSchema: schemaFor[NorInput](withPattern(norSchemaPattern, "nor"), pageSchema),
	}, h.getTextByNor)
}

func (h *handler) getDocument(ctx context.Context, _ *mcp.CallToolRequest, in GetDocumentInput) (*mcp.CallToolResult, any, error) {
	raw := strings.TrimSpace(in.ID)
	if circularPattern.MatchString(raw) {
		circ, err := h.api.GetCircular(ctx, raw)
		if err != nil {
			return nil, nil, apiError(err, "circular "+raw)
		}
		return h.paginate(formatCircular(circ), in.PageInput), nil, nil
	}

	id, err := normalizeID(raw)
	if err != nil {
		return nil, nil, err
	}
	prefix := strings.TrimRight(id, "0123456789")
	var md string
	switch {
	case strings.HasSuffix(prefix, "ARTI"):
		art, err := h.api.GetArticle(ctx, id)
		if err != nil {
			return nil, nil, apiError(err, "article "+id)
		}
		md = formatArticle(art)
	case prefix == "LEGITEXT":
		date, err := h.dateOrToday(in.Date)
		if err != nil {
			return nil, nil, err
		}
		text, err := h.api.GetLegiText(ctx, id, date)
		if err != nil {
			return nil, nil, apiError(err, "text "+id)
		}
		md = formatText(text, "Version in force on "+date)
	case prefix == "JORFTEXT":
		text, err := h.api.GetJorfText(ctx, id)
		if err != nil {
			return nil, nil, apiError(err, "text "+id)
		}
		md = formatText(text, "Journal officiel")
	case prefix == "KALITEXT":
		text, err := h.api.GetKaliText(ctx, id)
		if err != nil {
			return nil, nil, apiError(err, "text "+id)
		}
		md = formatText(text, "Collective agreement text")
	case prefix == "KALICONT":
		cont, err := h.api.GetKaliContainer(ctx, id)
		if err != nil {
			return nil, nil, apiError(err, "collective agreement "+id)
		}
		md = formatKaliContainer(cont)
	case prefix == "JURITEXT", prefix == "CETATEXT", prefix == "CONSTEXT", prefix == "JUFITEXT":
		dec, err := h.api.GetJuri(ctx, id)
		if err != nil {
			return nil, nil, apiError(err, "decision "+id)
		}
		md = formatSimpleText(dec)
	case prefix == "CNILTEXT":
		delib, err := h.api.GetCnil(ctx, id)
		if err != nil {
			return nil, nil, apiError(err, "deliberation "+id)
		}
		md = formatSimpleText(delib)
	case prefix == "ACCOTEXT":
		acco, err := h.api.GetAgreement(ctx, id)
		if err != nil {
			return nil, nil, apiError(err, "company agreement "+id)
		}
		md = formatAgreement(acco)
	case prefix == "LEGISCTA":
		return nil, nil, fmt.Errorf("%s is a code section: use get_code_section with the LEGITEXT identifier of the code", id)
	default:
		return nil, nil, fmt.Errorf("unsupported identifier prefix %q", prefix)
	}
	return h.paginate(md, in.PageInput), nil, nil
}

func (h *handler) getArticleByNumber(ctx context.Context, _ *mcp.CallToolRequest, in ArticleByNumberInput) (*mcp.CallToolResult, any, error) {
	textID, err := normalizeID(in.TextID, "LEGITEXT")
	if err != nil {
		return nil, nil, err
	}
	num := strings.TrimSpace(in.Number)
	num = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(num, "Article"), "article"))
	if num == "" {
		return nil, nil, fmt.Errorf("number must not be empty")
	}
	art, err := h.api.GetArticleByNumber(ctx, textID, num)
	if err != nil {
		return nil, nil, apiError(err, fmt.Sprintf("article %s of %s", num, textID))
	}
	return h.paginate(formatArticle(art), in.PageInput), nil, nil
}

func (h *handler) getArticleHistory(ctx context.Context, _ *mcp.CallToolRequest, in ArticleHistoryInput) (*mcp.CallToolResult, any, error) {
	id, err := normalizeID(in.ArticleID)
	if err != nil {
		return nil, nil, err
	}
	// L'endpoint attend l'identifiant chronique : on le récupère depuis l'article.
	art, err := h.api.GetArticle(ctx, id)
	if err != nil {
		return nil, nil, apiError(err, "article "+id)
	}
	versions := art.ArticleVersions
	if len(versions) == 0 && art.Cid == "" {
		return nil, nil, fmt.Errorf("no version history available for article %s", id)
	}
	if len(versions) == 0 {
		list, err := h.api.GetArticleVersions(ctx, art.Cid)
		if err != nil {
			return nil, nil, apiError(err, "versions of article "+id)
		}
		return textResult(formatArticleVersionList(art, list)), nil, nil
	}
	return textResult(formatArticleHistory(art)), nil, nil
}

func (h *handler) getTableOfContents(ctx context.Context, _ *mcp.CallToolRequest, in TableOfContentsInput) (*mcp.CallToolResult, any, error) {
	textID, err := normalizeID(in.TextID, "LEGITEXT", "JORFTEXT")
	if err != nil {
		return nil, nil, err
	}
	date, err := h.dateOrToday(in.Date)
	if err != nil {
		return nil, nil, err
	}
	depth := in.Depth
	if depth == 0 {
		depth = 3
	}
	toc, err := h.api.GetTableOfContents(ctx, textID, date)
	if err != nil {
		return nil, nil, apiError(err, "table of contents of "+textID)
	}
	return h.paginate(formatTableOfContents(toc, date, depth), in.PageInput), nil, nil
}

func (h *handler) getCodeSection(ctx context.Context, _ *mcp.CallToolRequest, in CodeSectionInput) (*mcp.CallToolResult, any, error) {
	textID, err := normalizeID(in.TextID, "LEGITEXT")
	if err != nil {
		return nil, nil, err
	}
	sectionID, err := normalizeID(in.SectionID, "LEGISCTA")
	if err != nil {
		return nil, nil, err
	}
	date, err := h.dateOrToday(in.Date)
	if err != nil {
		return nil, nil, err
	}
	text, err := h.api.GetCodeSection(ctx, textID, sectionID, date)
	if err != nil {
		return nil, nil, apiError(err, "section "+sectionID)
	}
	return h.paginate(formatText(text, "Version in force on "+date), in.PageInput), nil, nil
}

func (h *handler) getCollectiveAgreement(ctx context.Context, _ *mcp.CallToolRequest, in CollectiveAgreementInput) (*mcp.CallToolResult, any, error) {
	// L'IDCC est transmis tel que saisi : l'API attend la forme publiée (ex. « 0016 » ou « 1486 »).
	idcc := strings.TrimSpace(in.Idcc)
	if !idccPattern.MatchString(idcc) {
		return nil, nil, fmt.Errorf("invalid IDCC %q: expected up to 5 digits", in.Idcc)
	}
	cont, err := h.api.GetKaliContainerByIdcc(ctx, idcc)
	if err != nil {
		return nil, nil, apiError(err, "collective agreement IDCC "+idcc)
	}
	return h.paginate(formatKaliContainer(cont), in.PageInput), nil, nil
}

func (h *handler) getTextByNor(ctx context.Context, _ *mcp.CallToolRequest, in NorInput) (*mcp.CallToolResult, any, error) {
	nor := strings.ToUpper(strings.TrimSpace(in.Nor))
	if !norPattern.MatchString(nor) {
		return nil, nil, fmt.Errorf("invalid NOR %q (expected e.g. ECOX1515436L)", in.Nor)
	}
	text, err := h.api.GetJorfTextByNor(ctx, nor)
	if err != nil {
		return nil, nil, apiError(err, "text with NOR "+nor)
	}
	return h.paginate(formatText(text, "Journal officiel"), in.PageInput), nil, nil
}
