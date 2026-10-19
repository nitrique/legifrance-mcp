package textutil

import (
	"strings"
	"testing"
)

func TestHTMLToText(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain", "  Tout   fait  ", "Tout fait"},
		{"paragraphs", "<p>Alinéa 1.</p><p>Alinéa&nbsp;2 &amp; suite.</p>", "Alinéa 1.\n\nAlinéa 2 & suite."},
		{"line break", "ligne 1<br/>ligne 2", "ligne 1\nligne 2"},
		{"list", "<ul><li>un</li><li>deux</li></ul>", "- un\n- deux"},
		{"highlight", "le <mark>mariage</mark> civil", "le mariage civil"},
		{"script", "<p>ok</p><script>alert(1)</script>", "ok"},
		{"table", "<table><tr><td>a</td><td>b</td></tr></table>", "| a | b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HTMLToText(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPaginate(t *testing.T) {
	text := strings.Repeat("ligne é\n", 100) // 800 caractères
	page := Paginate(text, 0, 300)
	if page.Next <= 0 || page.Next > 300 || !strings.HasSuffix(page.Content, "\n") {
		t.Fatalf("first page: next=%d content end=%q", page.Next, page.Content[len(page.Content)-3:])
	}
	var rebuilt strings.Builder
	rebuilt.WriteString(page.Content)
	for page.Next >= 0 {
		page = Paginate(text, page.Next, 300)
		rebuilt.WriteString(page.Content)
	}
	if rebuilt.String() != text {
		t.Error("pages do not rebuild the original text")
	}
	if p := Paginate(text, 10000, 300); p.Content != "" || p.Next != -1 {
		t.Errorf("out of range page: %+v", p)
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("éléphant", 3); got != "élé…" {
		t.Errorf("got %q", got)
	}
	if got := Truncate("court", 10); got != "court" {
		t.Errorf("got %q", got)
	}
}
