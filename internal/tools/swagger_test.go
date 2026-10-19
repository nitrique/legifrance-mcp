package tools

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// Ce test vérifie que les énumérations exposées dans les schémas des outils
// restent cohérentes avec la spécification Swagger fournie par la DILA.

type swaggerProp struct {
	Enum  []string `json:"enum"`
	Items struct {
		Enum []string `json:"enum"`
	} `json:"items"`
}

func loadSwaggerEnums(t *testing.T) func(def, prop string) []string {
	t.Helper()
	raw, err := os.ReadFile("../../doc/swagger.json")
	if err != nil {
		t.Skipf("swagger not available: %v", err)
	}
	var spec struct {
		Definitions map[string]struct {
			Properties map[string]swaggerProp `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	return func(def, prop string) []string {
		p, ok := spec.Definitions[def].Properties[prop]
		if !ok {
			t.Fatalf("swagger has no %s.%s", def, prop)
		}
		if len(p.Enum) > 0 {
			return p.Enum
		}
		return p.Items.Enum
	}
}

func TestEnumsMatchSwagger(t *testing.T) {
	enumOf := loadSwaggerEnums(t)
	cases := []struct {
		name     string
		ours     []string
		def      string
		prop     string
		complete bool // toutes les valeurs du Swagger doivent être exposées
	}{
		{"search fonds", searchFonds, "SearchRequestDTO", "fond", true},
		{"search fields", searchFieldTypes, "ChampDTO", "typeChamp", true},
		{"match types", searchMatchTypes, "CritereDTO", "typeRecherche", true},
		{"suggest supplies", suggestSupplies, "SuggestSupplyRequest", "supplies", true},
		{"LODA natures", lodaNatures, "LODAListRequest", "natures", true},
		{"legal statuses", legalStatuses, "LODAListRequest", "legalStatus", true},
		{"code states", legalStatuses, "CodeListRequest", "states", true},
		{"LODA sorts", lodaSorts, "LODAListRequest", "sort", false},
	}
	for _, tc := range cases {
		spec := enumOf(tc.def, tc.prop)
		for _, v := range tc.ours {
			if !slices.Contains(spec, v) {
				t.Errorf("%s: %q is not in the swagger enum %v", tc.name, v, spec)
			}
		}
		if tc.complete {
			for _, v := range spec {
				if !slices.Contains(tc.ours, v) {
					t.Errorf("%s: swagger value %q is missing", tc.name, v)
				}
			}
		}
	}
}
