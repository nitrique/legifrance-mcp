// Package textutil convertit le HTML renvoyé par Légifrance en texte brut
// lisible et découpe les contenus volumineux.
package textutil

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTMLToText convertit un fragment HTML en texte brut : les blocs deviennent
// des paragraphes, les listes des puces et les cellules de tableau sont
// séparées par « | ». Les balises <script> et <style> sont ignorées.
func HTMLToText(src string) string {
	if !strings.ContainsAny(src, "<&") {
		return normalize(src)
	}
	var b strings.Builder
	z := html.NewTokenizer(strings.NewReader(src))
	skip := 0
	for {
		switch z.Next() {
		case html.ErrorToken:
			return normalize(b.String())
		case html.TextToken:
			if skip == 0 {
				b.Write(z.Text())
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			a := atom.Lookup(name)
			switch a {
			case atom.Script, atom.Style:
				skip++
			case atom.Br:
				b.WriteByte('\n')
			case atom.Li:
				b.WriteString("\n- ")
			case atom.Td, atom.Th:
				b.WriteString(" | ")
			default:
				if isBlock(a) {
					b.WriteString("\n\n")
				}
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			a := atom.Lookup(name)
			switch {
			case a == atom.Script || a == atom.Style:
				if skip > 0 {
					skip--
				}
			case a == atom.Tr:
				b.WriteByte('\n')
			case isBlock(a):
				b.WriteString("\n\n")
			}
		}
	}
}

func isBlock(a atom.Atom) bool {
	switch a {
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Blockquote,
		atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6,
		atom.Ul, atom.Ol, atom.Table, atom.Pre, atom.Center:
		return true
	}
	return false
}

// normalize compacte les espaces de chaque ligne et limite les lignes vides
// consécutives à une seule.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, " ", " ")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
			}
			blank = true
			continue
		}
		blank = false
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// Page représente une portion d'un contenu découpé.
type Page struct {
	Content string
	// Start est l'index (en caractères) du début de la portion.
	Start int
	// Next est l'index du début de la portion suivante, ou -1 s'il n'y en a pas.
	Next int
	// Total est la longueur totale du contenu (en caractères).
	Total int
}

// Paginate renvoie la portion de s commençant au caractère start et
// contenant au plus maxLen caractères. La coupe est reculée jusqu'à la fin de
// ligne précédente lorsque c'est possible, pour ne pas couper un paragraphe.
func Paginate(s string, start, maxLen int) Page {
	runes := []rune(s)
	total := len(runes)
	if start < 0 {
		start = 0
	}
	if start >= total {
		return Page{Start: start, Next: -1, Total: total}
	}
	end := start + maxLen
	if maxLen <= 0 || end >= total {
		return Page{Content: string(runes[start:]), Start: start, Next: -1, Total: total}
	}
	// Recul jusqu'au dernier saut de ligne situé dans la seconde moitié de la page.
	for i := end; i > start+maxLen/2; i-- {
		if runes[i-1] == '\n' {
			end = i
			break
		}
	}
	return Page{Content: string(runes[start:end]), Start: start, Next: end, Total: total}
}

// Truncate coupe s à maxLen caractères en ajoutant « … » si nécessaire.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 || utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:maxLen])) + "…"
}
