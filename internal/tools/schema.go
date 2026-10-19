package tools

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// schemaFor infère le JSON Schema d'entrée d'un outil à partir du type T
// (descriptions issues des tags `jsonschema`), puis applique des ajustements
// que l'inférence ne sait pas exprimer : énumérations, bornes, valeurs par défaut.
func schemaFor[T any](patches ...func(*jsonschema.Schema)) *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		// Erreur de programmation : le type d'entrée n'est pas représentable.
		panic(fmt.Sprintf("infer schema for %T: %v", *new(T), err))
	}
	for _, p := range patches {
		p(s)
	}
	return s
}

// prop renvoie le schéma d'une propriété, en descendant dans les éléments
// d'un tableau si nécessaire (« filters.facet » désigne la propriété facet des
// éléments du tableau filters).
func prop(s *jsonschema.Schema, path ...string) *jsonschema.Schema {
	cur := s
	for _, name := range path {
		if cur.Items != nil {
			cur = cur.Items
		}
		next, ok := cur.Properties[name]
		if !ok {
			panic(fmt.Sprintf("schema has no property %q", name))
		}
		cur = next
	}
	return cur
}

func enum(values ...string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

// withEnum restreint une propriété chaîne (ou les éléments d'un tableau de chaînes) à une liste de valeurs.
func withEnum(values []string, path ...string) func(*jsonschema.Schema) {
	return func(s *jsonschema.Schema) {
		p := prop(s, path...)
		if p.Items != nil {
			p = p.Items
		}
		p.Enum = enum(values...)
	}
}

// withRange borne une propriété numérique.
func withRange(minimum, maximum float64, path ...string) func(*jsonschema.Schema) {
	return func(s *jsonschema.Schema) {
		p := prop(s, path...)
		p.Minimum = &minimum
		p.Maximum = &maximum
	}
}

// withPattern impose une expression régulière à une propriété chaîne.
func withPattern(pattern string, path ...string) func(*jsonschema.Schema) {
	return func(s *jsonschema.Schema) {
		prop(s, path...).Pattern = pattern
	}
}

// withMaxLength borne la longueur d'une propriété chaîne (ou des éléments d'un tableau de chaînes).
func withMaxLength(n int, path ...string) func(*jsonschema.Schema) {
	return func(s *jsonschema.Schema) {
		p := prop(s, path...)
		if p.Items != nil {
			p = p.Items
		}
		p.MaxLength = &n
	}
}

// withMaxItems borne le nombre d'éléments d'une propriété tableau.
func withMaxItems(n int, path ...string) func(*jsonschema.Schema) {
	return func(s *jsonschema.Schema) {
		prop(s, path...).MaxItems = &n
	}
}

// datePattern est le motif AAAA-MM-JJ appliqué aux dates d'entrée.
const datePattern = `^\d{4}-\d{2}-\d{2}$`
