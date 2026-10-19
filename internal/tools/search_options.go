package tools

// Référentiel des champs, filtres et tris de recherche par fonds, repris du
// document « description-des-tris-et-filtres-de-l-api.xlsx » de la DILA.

type facetDoc struct {
	Name  string // valeur à transmettre dans « facet »
	Label string
	Kind  string // "values", "period" ou "date"
}

type fondDoc struct {
	Label   string
	Fields  []string
	Filters []facetDoc
	Sorts   []string
}

// Fonds acceptés par /search.
var searchFonds = []string{
	"ALL", "JORF", "CODE_DATE", "CODE_ETAT", "LODA_DATE", "LODA_ETAT",
	"CIRC", "CONSTIT", "CETAT", "JURI", "JUFI", "KALI", "ACCO", "CNIL",
}

// Fonds acceptés par /suggest (SuggestSupplyRequest.supplies).
var suggestSupplies = []string{
	"ALL", "ALL_SUGGEST", "LODA_LIST", "CODE_LIST", "CODE_RELEASE_DATE", "CODE_RELEASE_DATE_SUGGEST",
	"CODE_LEGAL_STATUS", "LODA_RELEASE_DATE", "LODA_RELEASE_DATE_SUGGEST", "LODA_LEGAL_STATUS", "KALI",
	"KALI_TEXT", "CONSTIT", "CETAT", "JUFI", "JURI", "JORF", "JORF_SUGGEST", "CNIL", "ARTICLE", "CIRC", "ACCO", "PDF",
}

// Types de champ acceptés par /search (ChampDTO.typeChamp).
var searchFieldTypes = []string{
	"ALL", "TITLE", "TABLE", "NOR", "NUM", "ADVANCED_TEXTE_ID", "NUM_DELIB", "NUM_DEC",
	"NUM_ARTICLE", "ARTICLE", "MINISTERE", "VISA", "NOTICE", "VISA_NOTICE", "TRAVAUX_PREP",
	"SIGNATURE", "NOTA", "NUM_AFFAIRE", "ABSTRATS", "RESUMES", "TEXTE", "ECLI", "NUM_LOI_DEF",
	"TYPE_DECISION", "NUMERO_INTERNE", "REF_PUBLI", "RESUME_CIRC", "TEXTE_REF", "TITRE_LOI_DEF",
	"RAISON_SOCIALE", "MOTS_CLES", "IDCC",
}

// Types de correspondance acceptés par /search (CritereDTO.typeRecherche).
var searchMatchTypes = []string{
	"TOUS_LES_MOTS_DANS_UN_CHAMP", "UN_DES_MOTS", "EXACTE",
	"AUCUN_DES_MOTS", "AUCUNE_CORRESPONDANCE_A_CETTE_EXPRESSION",
}

const (
	kindValues = "values"
	kindPeriod = "period"
	kindDate   = "date"
)

var (
	codeFilters = []facetDoc{
		{"ARTICLE_LEGAL_STATUS", "article legal status (VIGUEUR, ABROGE, VIGUEUR_DIFF…)", kindValues},
		{"TEXT_LEGAL_STATUS", "text legal status (VIGUEUR, ABROGE, VIGUEUR_DIFF…)", kindValues},
		{"DATE_VERSION", "version date (CODE_DATE only, defaulted to today)", kindDate},
		{"NOM_CODE", "code name, e.g. \"Code civil\" (CODE_DATE; use TEXT_NOM_CODE for CODE_ETAT)", kindValues},
		{"TEXT_NOM_CODE", "code name (CODE_ETAT)", kindValues},
		{"NUM_ARTICLE", "article number, e.g. \"L1234-5\"", kindValues},
	}
	lodaFilters = []facetDoc{
		{"NATURE", "nature: LOI, ORDONNANCE, DECRET, ARRETE… (LODA_DATE; TEXT_NATURE for LODA_ETAT)", kindValues},
		{"TEXT_NATURE", "nature (LODA_ETAT)", kindValues},
		{"DATE_SIGNATURE", "signature date (LODA_DATE; TEXT_DATE_SIGNATURE for LODA_ETAT)", kindPeriod},
		{"TEXT_DATE_SIGNATURE", "signature date (LODA_ETAT)", kindPeriod},
		{"DATE_PUBLICATION", "publication date (LODA_DATE; TEXT_DATE_PUBLICATION for LODA_ETAT)", kindPeriod},
		{"TEXT_DATE_PUBLICATION", "publication date (LODA_ETAT)", kindPeriod},
		{"ARTICLE_LEGAL_STATUS", "article legal status", kindValues},
		{"TEXT_LEGAL_STATUS", "text legal status", kindValues},
		{"NOR", "NOR number", kindValues},
		{"NUM_TEXTE", "text number, e.g. \"2016-1088\"", kindValues},
		{"NUM_ARTICLE", "article number", kindValues},
		{"DATE_VERSION", "version date (LODA_DATE only, defaulted to today)", kindDate},
	}
	dateSorts      = []string{"PERTINENCE", "DATE_DESC", "DATE_ASC"}
	signPubliSorts = []string{"PERTINENCE", "SIGNATURE_DATE_DESC", "SIGNATURE_DATE_ASC", "PUBLICATION_DATE_DESC", "PUBLICATION_DATE_ASC"}
)

var fondDocs = map[string]fondDoc{
	"ALL": {
		Label:   "all collections",
		Fields:  []string{"ALL", "TITLE"},
		Filters: []facetDoc{{"FOND", "restrict to collections (values such as JORF, CODE, LODA, JURI…)", kindValues}},
		Sorts:   []string{"PERTINENCE"},
	},
	"JORF": {
		Label:  "Journal officiel",
		Fields: []string{"ALL", "TITLE", "NOR", "NUM", "NUM_ARTICLE", "ARTICLE", "VISA", "NOTICE", "VISA_NOTICE", "TRAVAUX_PREP", "SIGNATURE", "NOTA"},
		Filters: []facetDoc{
			{"NATURE", "nature: LOI, ORDONNANCE, DECRET, ARRETE…", kindValues},
			{"DATE_SIGNATURE", "signature date", kindPeriod},
			{"DATE_PUBLICATION", "publication date", kindPeriod},
			{"MINISTERE", "issuing ministry", kindValues},
			{"EMETTEUR", "issuing authority", kindValues},
			{"NOR", "NOR number", kindValues},
			{"NUM_TEXTE", "text number", kindValues},
			{"NUM_ARTICLE", "article number", kindValues},
			{"DECORATION", "decoration", kindValues},
			{"DELEGATION", "delegation", kindValues},
		},
		Sorts: signPubliSorts,
	},
	"CODE_DATE": {
		Label:   "codes, version in force at a given date",
		Fields:  []string{"ALL", "TITLE", "TABLE", "NUM_ARTICLE", "ARTICLE"},
		Filters: codeFilters,
		Sorts:   []string{"PERTINENCE"},
	},
	"CODE_ETAT": {
		Label:   "codes, filtered by legal status",
		Fields:  []string{"ALL", "TITLE", "TABLE", "NUM_ARTICLE", "ARTICLE"},
		Filters: codeFilters,
		Sorts:   []string{"PERTINENCE"},
	},
	"LODA_DATE": {
		Label:   "laws, ordinances, decrees and orders, version in force at a given date",
		Fields:  []string{"ALL", "TITLE", "NOR", "NUM", "NUM_ARTICLE", "ARTICLE", "VISA", "NOTICE", "VISA_NOTICE", "TRAVAUX_PREP", "SIGNATURE", "NOTA"},
		Filters: lodaFilters,
		Sorts:   signPubliSorts,
	},
	"LODA_ETAT": {
		Label:   "laws, ordinances, decrees and orders, filtered by legal status",
		Fields:  []string{"ALL", "TITLE", "NOR", "NUM", "NUM_ARTICLE", "ARTICLE", "VISA", "NOTICE", "VISA_NOTICE", "TRAVAUX_PREP", "SIGNATURE", "NOTA"},
		Filters: lodaFilters,
		Sorts:   signPubliSorts,
	},
	"CIRC": {
		Label:  "circulars and instructions",
		Fields: []string{"ALL", "TITLE", "NOR", "RESUME_CIRC", "TEXTE_REF"},
		Filters: []facetDoc{
			{"DATE_MEL", "online publication date", kindPeriod},
			{"DATE_SIGNATURE", "signature date", kindPeriod},
			{"DATE_MEA", "date of entry into application", kindPeriod},
			{"OPPOSABILITE", "enforceability", kindValues},
			{"DOMAINE", "domain", kindValues},
			{"MIN_DEPOSANT", "filing ministry", kindValues},
			{"MIN_CONCERNE", "ministry concerned", kindValues},
			{"MOTS_CLEFS", "keywords", kindValues},
			{"NUMERO_INTERNE", "internal number", kindValues},
			{"NOR", "NOR number", kindValues},
			{"REF_PUBLI", "JO or BO publication reference", kindValues},
		},
		Sorts: []string{"PERTINENCE", "SIGNATURE_DATE_DESC", "SIGNATURE_DATE_ASC", "PUBLI_DATE_DESC", "PUBLI_DATE_ASC"},
	},
	"CONSTIT": {
		Label:  "Conseil constitutionnel decisions",
		Fields: []string{"ALL", "TITLE", "NUM_DEC", "TEXTE"},
		Filters: []facetDoc{
			{"NATURE_CONSTIT", "constitutionality review type", kindValues},
			{"NATURE_NORME_AUTRE", "other review types", kindValues},
			{"SOLUTION_CONSTIT", "review outcome", kindValues},
			{"TITRE_DEFEREE", "title of the referred law", kindValues},
			{"NUM_LOI", "number of the referred law", kindValues},
			{"DATE_LOI", "signature date of the referred law", kindPeriod},
			{"TYPE_DECISION", "electoral litigation decision type", kindValues},
			{"SOLUTION_ELECT", "electoral litigation outcome", kindValues},
			{"NATURE_AUTRE", "other decision type", kindValues},
			{"SOLUTION_AUTRE", "other decision outcome", kindValues},
			{"DATE_DECISION", "decision date", kindPeriod},
			{"NUMERO_DECISION", "decision number", kindValues},
			{"NOR", "NOR number", kindValues},
		},
		Sorts: dateSorts,
	},
	"CETAT": {
		Label:  "administrative case law (Conseil d'État, CAA, TA)",
		Fields: []string{"ALL", "TITLE", "NUM_DEC", "ABSTRATS", "TEXTE", "RESUMES"},
		Filters: []facetDoc{
			{"JURIDICTION_NATURE", "court, e.g. CONSEIL_ETAT, COURS_APPEL, TRIBUNAL_ADMINISTATIF", kindValues},
			{"DATE_DECISION", "decision date", kindPeriod},
			{"DATE_VERSEMENT", "date added to the database", kindPeriod},
			{"PUBLICATION_RECUEIL", "publication in the Recueil Lebon", kindValues},
			{"NUMERO_DECISION", "decision number", kindValues},
			{"ECLI", "ECLI identifier", kindValues},
		},
		Sorts: dateSorts,
	},
	"JURI": {
		Label:  "judicial case law (Cour de cassation, cours d'appel…)",
		Fields: []string{"ALL", "TITLE", "ABSTRATS", "NUM_AFFAIRE", "TEXTE", "RESUMES"},
		Filters: []facetDoc{
			{"JURIDICTION_JUDICIAIRE", "court", kindValues},
			{"CASSATION_TYPE_PUBLICATION_BULLETIN", "publication in the Bulletin", kindValues},
			{"NUM_BULLETIN", "Bulletin number", kindValues},
			{"ANNEE_BULLETIN", "Bulletin year", kindValues},
			{"CASSATION_NATURE_DECISION", "Cour de cassation decision type", kindValues},
			{"CASSATION_FORMATION", "Cour de cassation chamber", kindValues},
			{"CASSATION_DECISION_ATTAQUEE", "contested decision", kindValues},
			{"LIEU_DECISION", "place of the contested decision", kindValues},
			{"DATE_DECISION_ATTAQUEE", "date of the contested decision", kindPeriod},
			{"APPEL_SIEGE_APPEL", "court of appeal seat", kindValues},
			{"PREMIER_DEGRE_TYPE_JURIDICTION", "first instance court type", kindValues},
			{"PREMIER_DEGRE_SIEGE", "first instance court seat", kindValues},
			{"DATE_DECISION", "decision date", kindPeriod},
			{"NUM_AFFAIRE", "case number, e.g. \"20-12.345\"", kindValues},
			{"ECLI", "ECLI identifier", kindValues},
		},
		Sorts: dateSorts,
	},
	"JUFI": {
		Label:  "financial case law (Cour des comptes…)",
		Fields: []string{"ALL", "TITLE", "NUM_DEC", "ABSTRATS", "TEXTE"},
		Filters: []facetDoc{
			{"PUBLICATION_RECUEIL", "publication in the Recueil", kindValues},
			{"JURIDICTION_NATURE", "court", kindValues},
			{"DATE_DECISION", "decision date", kindPeriod},
			{"NUMERO_DECISION", "decision number", kindValues},
		},
		Sorts: dateSorts,
	},
	"KALI": {
		Label:  "collective agreements (conventions collectives)",
		Fields: []string{"ALL", "TITLE", "IDCC", "MOTS_CLES", "ARTICLE"},
		Filters: []facetDoc{
			{"LEGAL_STATUS", "text legal status", kindValues},
			{"ARTICLE_LEGAL_STATUS", "article legal status", kindValues},
			{"ACTIVITE", "business activity", kindValues},
			{"ARTICLE_QUESTION_USUELLE", "frequently asked question", kindValues},
			{"DATE_SIGNATURE", "signature date", kindPeriod},
			{"DATE_PUBLICATION", "publication date", kindPeriod},
			{"NATURE_TEXTE_CITE", "cited text nature", kindValues},
			{"NUM_TEXTE_CITE", "cited text number", kindValues},
			{"DATE_PUBLI_TEXTE_CITE", "cited text publication date", kindPeriod},
			{"NOM_CODE_CITE", "cited code name", kindValues},
			{"NUM_ARTICLE_CODE_CITE", "cited code article number", kindValues},
			{"IDCC", "IDCC number", kindValues},
			{"NOR", "NOR number", kindValues},
			{"CODE_NAF_OU_APE", "NAF/APE code", kindValues},
			{"NUMERO_BO", "BO number", kindValues},
			{"ARTICLE_NUMERO", "article number", kindValues},
		},
		Sorts: []string{"PERTINENCE", "MODIFICATION_DATE_DESC", "SIGNATURE_DATE_DESC", "SIGNATURE_DATE_ASC"},
	},
	"ACCO": {
		Label:  "company agreements (accords d'entreprise)",
		Fields: []string{"ALL", "TITLE", "RAISON_SOCIALE", "IDCC"},
		Filters: []facetDoc{
			{"THEME", "agreement theme", kindValues},
			{"SIGNATAIRE", "signatory union", kindValues},
			{"DATE_SIGNATURE", "signature date", kindPeriod},
			{"ACTIVITE_PRINCIPALE", "main business activity", kindValues},
			{"CODE_APE", "APE code", kindValues},
			{"IDCC", "IDCC number", kindValues},
			{"VILLE", "establishment city", kindValues},
			{"CODE_POSTAL", "establishment postcode", kindValues},
			{"DATE_DIFFUSION", "publication date", kindPeriod},
			{"SIRET_RAISON_SOCIALE", "SIRET or company name", kindValues},
		},
		Sorts: dateSorts,
	},
	"CNIL": {
		Label:  "CNIL deliberations",
		Fields: []string{"ALL", "TITLE", "NOR", "TEXTE", "NUM_DELIB"},
		Filters: []facetDoc{
			{"TYPE", "type", kindValues},
			{"NATURE_DELIB", "deliberation nature", kindValues},
			{"DATE_DELIB", "deliberation date", kindPeriod},
			{"NUMERO_DELIB", "deliberation number", kindValues},
			{"NOR", "NOR number", kindValues},
		},
		Sorts: []string{"PERTINENCE", "DATE_DECISION_DESC", "DATE_DECISION_ASC"},
	},
}
