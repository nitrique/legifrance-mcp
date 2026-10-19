package tools

import (
	"fmt"
	"strings"

	"legifrance-mcp/internal/legifrance"
	"legifrance-mcp/internal/textutil"
)

// Fonctions de rendu Markdown des documents Légifrance.

// metaList accumule des lignes « **Libellé** : valeur » en ignorant les valeurs vides.
type metaList []string

func (m *metaList) add(label, value string) {
	if value = strings.TrimSpace(value); value != "" {
		*m = append(*m, fmt.Sprintf("- **%s**: %s", label, value))
	}
}

func (m *metaList) addID(label, id string) {
	if id != "" {
		m.add(label, "`"+id+"`")
	}
}

func (m metaList) write(b *strings.Builder) {
	if len(m) > 0 {
		b.WriteString(strings.Join(m, "\n"))
		b.WriteString("\n\n")
	}
}

// validity décrit la période de validité d'une version.
func validity(start, end legifrance.Date) string {
	switch {
	case start.IsZero() && (end.IsZero() || end.OpenEnded()):
		return ""
	case end.IsZero() || end.OpenEnded():
		return "since " + start.String()
	case start.IsZero():
		return "until " + end.String()
	default:
		return start.String() + " → " + end.String()
	}
}

// richText renvoie la version texte d'un contenu disponible en HTML et/ou en texte brut.
func richText(html, plain string) string {
	if strings.TrimSpace(html) != "" {
		return textutil.HTMLToText(html)
	}
	return textutil.HTMLToText(plain)
}

func formatArticle(a *legifrance.Article) string {
	var b strings.Builder
	title := "Article"
	if a.Num != "" {
		title += " " + string(a.Num)
	}
	if len(a.TextTitles) > 0 {
		title += " — " + firstNonEmpty(a.TextTitles[0].Titre, a.TextTitles[0].TitreLong)
	}
	fmt.Fprintf(&b, "# %s\n\n", title)

	var m metaList
	m.addID("ID", a.ID)
	m.addID("Article CID", a.Cid)
	m.add("Status", a.Etat)
	m.add("Validity", validity(a.DateDebut, a.DateFin))
	if len(a.TextTitles) > 0 {
		m.addID("Text", firstNonEmpty(a.TextTitles[0].Cid, a.TextTitles[0].ID))
	}
	m.add("Location", firstNonEmpty(a.FullSectionsTitre, a.SectionParentTitre))
	m.addID("Section", a.SectionParentCid)
	m.write(&b)

	b.WriteString(richText(a.TexteHTML, a.Texte))
	b.WriteString("\n")
	if nota := richText(a.NotaHTML, a.Nota); nota != "" {
		fmt.Fprintf(&b, "\n**Nota**: %s\n", nota)
	}
	if n := len(a.ArticleVersions); n > 1 {
		fmt.Fprintf(&b, "\n_%d versions exist: use get_article_history for the full history._\n", n)
	}
	return b.String()
}

func formatArticleHistory(a *legifrance.Article) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# History of article %s\n\n", firstNonEmpty(string(a.Num), a.ID))
	b.WriteString("| Version | ID | Status | Validity |\n|---|---|---|---|\n")
	for _, v := range a.ArticleVersions {
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", firstNonEmpty(v.Version, v.Numero), v.ID, v.Etat, validity(v.DateDebut, v.DateFin))
	}
	b.WriteString("\nRead a version with get_document and its ID.\n")
	return b.String()
}

func formatArticleVersionList(a *legifrance.Article, list []legifrance.Article) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# History of article %s\n\n", firstNonEmpty(string(a.Num), a.ID))
	b.WriteString("| ID | Status | Validity |\n|---|---|---|\n")
	for _, v := range list {
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", v.ID, v.Etat, validity(v.DateDebut, v.DateFin))
	}
	b.WriteString("\nRead a version with get_document and its ID.\n")
	return b.String()
}

// formatText rend un texte structuré (LODA, JORF, KALI, section de code) avec son contenu.
func formatText(t *legifrance.Text, contextLabel string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", firstNonEmpty(textutil.HTMLToText(t.Title), t.Cid, t.ID))

	var m metaList
	m.addID("ID", t.ID)
	if t.Cid != t.ID {
		m.addID("CID", t.Cid)
	}
	m.add("Context", contextLabel)
	m.add("Nature", t.Nature)
	m.add("Status", t.Etat)
	m.add("Number", string(t.TextNumber))
	m.add("NOR", t.Nor)
	m.add("ELI", t.Eli)
	m.add("Date", t.DateTexte.String())
	m.add("Published", t.DateParution.String())
	m.add("Version validity", validity(t.DateDebutVersion, t.DateFinVersion))
	m.write(&b)

	if visa := richText(t.VisasHTML, t.Visa); visa != "" {
		fmt.Fprintf(&b, "%s\n\n", visa)
	}
	for _, a := range t.Articles {
		writeTextArticle(&b, a)
	}
	for _, s := range t.Sections {
		writeTextSection(&b, s, 2)
	}
	if signers := textutil.HTMLToText(t.Signers); signers != "" {
		fmt.Fprintf(&b, "\n**Signatories**: %s\n", signers)
	}
	if nota := textutil.HTMLToText(t.Nota); nota != "" {
		fmt.Fprintf(&b, "\n**Nota**: %s\n", nota)
	}
	return b.String()
}

func writeTextSection(b *strings.Builder, s legifrance.TextSection, level int) {
	heading := strings.Repeat("#", min(level, 6))
	fmt.Fprintf(b, "\n%s %s", heading, textutil.HTMLToText(s.Title))
	if id := firstNonEmpty(s.Cid, s.ID); id != "" {
		fmt.Fprintf(b, " (`%s`)", id)
	}
	b.WriteString("\n\n")
	for _, a := range s.Articles {
		writeTextArticle(b, a)
	}
	for _, child := range s.Sections {
		writeTextSection(b, child, level+1)
	}
}

func writeTextArticle(b *strings.Builder, a legifrance.TextArticle) {
	label := "Article"
	if a.Num != "" {
		label += " " + string(a.Num)
	}
	fmt.Fprintf(b, "**%s**", label)
	var tags []string
	if a.ID != "" {
		tags = append(tags, "`"+a.ID+"`")
	}
	if a.Etat != "" {
		tags = append(tags, a.Etat)
	}
	if v := validity(a.DateDebut, a.DateFin); v != "" {
		tags = append(tags, v)
	}
	if len(tags) > 0 {
		fmt.Fprintf(b, " (%s)", strings.Join(tags, ", "))
	}
	b.WriteString("\n\n")
	if content := textutil.HTMLToText(a.Content); content != "" {
		b.WriteString(content)
		b.WriteString("\n\n")
	}
	if nota := textutil.HTMLToText(a.Nota); nota != "" {
		fmt.Fprintf(b, "_Nota_: %s\n\n", nota)
	}
}

// formatTableOfContents rend l'arborescence d'un texte sans le contenu des articles.
func formatTableOfContents(t *legifrance.Text, date string, maxDepth int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Table of contents — %s\n\n", firstNonEmpty(textutil.HTMLToText(t.Title), t.Cid, t.ID))
	var m metaList
	m.addID("Text", firstNonEmpty(t.Cid, t.ID))
	m.add("Version in force on", date)
	m.write(&b)

	if len(t.Articles) > 0 {
		b.WriteString(articleNumbers(t.Articles, ""))
	}
	for _, s := range t.Sections {
		writeTocSection(&b, s, 0, maxDepth)
	}
	b.WriteString("\nRead a section with get_code_section (section ID) or an article with get_document (article ID).\n")
	return b.String()
}

func writeTocSection(b *strings.Builder, s legifrance.TextSection, depth, maxDepth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(b, "%s- %s", indent, textutil.HTMLToText(s.Title))
	if id := firstNonEmpty(s.Cid, s.ID); id != "" {
		fmt.Fprintf(b, " `%s`", id)
	}
	if s.Etat != "" && s.Etat != "VIGUEUR" {
		fmt.Fprintf(b, " [%s]", s.Etat)
	}
	b.WriteString("\n")
	if maxDepth > 0 && depth+1 >= maxDepth {
		if n := countSubsections(s); n > 0 {
			fmt.Fprintf(b, "%s  - … %d subsections (increase depth or open this section)\n", indent, n)
		}
		return
	}
	if len(s.Articles) > 0 {
		b.WriteString(articleNumbers(s.Articles, indent+"  "))
	}
	for _, child := range s.Sections {
		writeTocSection(b, child, depth+1, maxDepth)
	}
}

// articleNumbers résume les articles d'une section sur une seule ligne.
func articleNumbers(articles []legifrance.TextArticle, indent string) string {
	parts := make([]string, 0, len(articles))
	for _, a := range articles {
		num := firstNonEmpty(string(a.Num), "?")
		parts = append(parts, fmt.Sprintf("%s `%s`", num, a.ID))
	}
	return fmt.Sprintf("%s- Articles: %s\n", indent, strings.Join(parts, ", "))
}

func countSubsections(s legifrance.TextSection) int {
	n := len(s.Sections)
	for _, child := range s.Sections {
		n += countSubsections(child)
	}
	return n
}

func formatKaliContainer(c *legifrance.KaliContainer) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", firstNonEmpty(textutil.HTMLToText(c.Titre), c.ID))
	var m metaList
	m.addID("ID", c.ID)
	m.add("IDCC", string(c.Num))
	m.add("Nature", c.Nature)
	m.write(&b)
	for _, s := range c.Sections {
		writeTocSection(&b, s, 0, 0)
	}
	b.WriteString("\nRead a text with get_document and its KALITEXT identifier.\n")
	return b.String()
}

func formatSimpleText(t *legifrance.SimpleText) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", firstNonEmpty(textutil.HTMLToText(t.TitreLong), textutil.HTMLToText(t.Titre), t.ID))
	var m metaList
	m.addID("ID", t.ID)
	m.add("Court", t.Juridiction)
	m.add("Chamber", t.Formation)
	m.add("Nature", firstNonEmpty(t.Nature, t.NatureDelib))
	m.add("Date", t.DateTexte.String())
	m.add("Case number", strings.Join(t.NumeroAffaire, ", "))
	m.add("Number", string(t.Num))
	m.add("ECLI", t.Ecli)
	m.add("NOR", t.Nor)
	m.add("Outcome", t.Solution)
	m.add("Status", t.Etat)
	m.add("President", t.President)
	m.add("Reporting judge", t.Rapporteur)
	m.add("Lawyers", t.Avocats)
	m.write(&b)

	for _, s := range t.Sommaire {
		if abs := textutil.HTMLToText(s.Abstrats); abs != "" {
			fmt.Fprintf(&b, "**Keywords**: %s\n\n", abs)
		}
		if sum := textutil.HTMLToText(s.ResumePrincipal); sum != "" {
			fmt.Fprintf(&b, "**Summary**: %s\n\n", sum)
		}
	}
	b.WriteString("## Full text\n\n")
	b.WriteString(richText(t.TexteHTML, t.Texte))
	b.WriteString("\n")
	return b.String()
}

func formatAgreement(a *legifrance.Agreement) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", firstNonEmpty(textutil.HTMLToText(a.TitreTexte), a.ID))
	var m metaList
	m.addID("ID", a.ID)
	m.add("Company", a.RaisonSociale)
	m.add("SIRET", a.Siret)
	m.add("IDCC", string(a.CodeIdcc))
	m.add("APE code", a.CodeApe)
	m.add("Sector", a.Secteur)
	m.add("Nature", a.Nature)
	m.add("Signed", a.DateTexte.String())
	m.add("Effective", validity(a.DateEffet, a.DateFin))
	m.add("Filed", a.DateDepot.String())
	m.add("Themes", joinLabels(a.Themes))
	m.add("Signatory unions", joinLabels(a.Syndicats))
	m.add("Attachment", a.AttachementURL)
	m.write(&b)
	if content := textutil.HTMLToText(a.Data); content != "" {
		fmt.Fprintf(&b, "## Content\n\n%s\n", content)
	}
	return b.String()
}

func formatCircular(c *legifrance.Circular) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", firstNonEmpty(textutil.HTMLToText(c.Titre), c.ID))
	var m metaList
	m.addID("ID", c.ID)
	m.add("NOR", c.Nor)
	m.add("Internal number", c.NumeroInterne)
	m.add("Status", c.Etat)
	m.add("Signed", c.DateSignature.String())
	m.add("Applicable from", c.DateMiseApplication.String())
	m.add("Enforceable", string(c.Opposable))
	m.add("Author", c.Auteur)
	m.add("Signatory", c.Signataire)
	m.add("Recipients", c.Destinataire)
	m.add("Ministries", strings.Join(c.MinisteresDeposants, ", "))
	m.add("Domains", strings.Join(c.Domaines, ", "))
	m.add("Attachment", c.AttachmentURL)
	m.write(&b)
	if summary := textutil.HTMLToText(c.Resume); summary != "" {
		fmt.Fprintf(&b, "## Summary\n\n%s\n\n", summary)
	}
	if content := textutil.HTMLToText(c.Data); content != "" {
		fmt.Fprintf(&b, "## Content\n\n%s\n", content)
	}
	return b.String()
}

func joinLabels(values []legifrance.LabelValue) string {
	labels := make([]string, 0, len(values))
	for _, v := range values {
		if l := firstNonEmpty(v.Libelle, v.Code); l != "" {
			labels = append(labels, l)
		}
	}
	return strings.Join(labels, ", ")
}
