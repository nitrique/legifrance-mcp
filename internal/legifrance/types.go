package legifrance

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// DateLayout est le format des dates attendu par l'API dans les requêtes.
const DateLayout = "2006-01-02"

// Date accepte les différents formats de date renvoyés par l'API :
// horodatage Unix en millisecondes, chaîne ISO 8601 ou date simple.
// Les DTO sont réduits aux champs exploités par le serveur MCP.
type Date struct{ time.Time }

// openEndedYear est l'année sentinelle utilisée par Légifrance pour une
// version sans date de fin (« 2999-01-01 »).
const openEndedYear = 2999

var dateLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.000-0700",
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05",
	DateLayout,
	"02/01/2006",
}

// UnmarshalJSON implémente json.Unmarshaler.
func (d *Date) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	if data[0] != '"' {
		ms, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			// Valeur inattendue : elle est ignorée plutôt que de faire échouer tout le décodage.
			return nil //nolint:nilerr // tolérance volontaire
		}
		d.Time = time.UnixMilli(ms).UTC()
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return nil //nolint:nilerr // tolérance volontaire
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		d.Time = time.UnixMilli(ms).UTC()
		return nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			d.Time = t
			return nil
		}
	}
	return nil
}

// MarshalJSON renvoie la date au format AAAA-MM-JJ (ou null).
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Format(DateLayout))
}

// OpenEnded indique une date de fin « sans limite » (2999-01-01).
func (d Date) OpenEnded() bool { return !d.IsZero() && d.Year() >= openEndedYear }

// String renvoie la date au format AAAA-MM-JJ, ou une chaîne vide.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return d.Format(DateLayout)
}

// FlexString accepte une chaîne, un nombre ou un booléen JSON.
// Certains champs documentés comme chaînes sont parfois renvoyés en nombre.
type FlexString string

// UnmarshalJSON implémente json.Unmarshaler.
func (f *FlexString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	if data[0] == '{' || data[0] == '[' {
		return nil
	}
	*f = FlexString(data)
	return nil
}

// ---------------------------------------------------------------------------
// Recherche (/search)
// ---------------------------------------------------------------------------

// SearchRequest correspond à SearchRequestDTO.
type SearchRequest struct {
	Fond      string         `json:"fond"`
	Recherche SearchCriteria `json:"recherche"`
}

// SearchCriteria correspond à RechercheSpecifiqueDTO.
type SearchCriteria struct {
	Champs         []SearchField  `json:"champs"`
	Filtres        []SearchFilter `json:"filtres,omitempty"`
	Operateur      string         `json:"operateur"`
	PageNumber     int            `json:"pageNumber"`
	PageSize       int            `json:"pageSize"`
	Sort           string         `json:"sort"`
	SecondSort     string         `json:"secondSort,omitempty"`
	TypePagination string         `json:"typePagination"`
}

// SearchField correspond à ChampDTO.
type SearchField struct {
	TypeChamp string            `json:"typeChamp"`
	Operateur string            `json:"operateur"`
	Criteres  []SearchCriterion `json:"criteres"`
}

// SearchCriterion correspond à CritereDTO.
type SearchCriterion struct {
	Valeur        string `json:"valeur"`
	TypeRecherche string `json:"typeRecherche"`
	Operateur     string `json:"operateur"`
	Proximite     int    `json:"proximite,omitempty"`
}

// SearchFilter correspond à FiltreDTO.
type SearchFilter struct {
	Facette    string      `json:"facette"`
	Valeurs    []string    `json:"valeurs,omitempty"`
	Dates      *DatePeriod `json:"dates,omitempty"`
	SingleDate string      `json:"singleDate,omitempty"`
}

// DatePeriod correspond à DatePeriod (bornes au format AAAA-MM-JJ).
type DatePeriod struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// SearchResponse correspond à SearchResponseDTO.
type SearchResponse struct {
	TotalResultNumber int            `json:"totalResultNumber"`
	Results           []SearchResult `json:"results"`
}

// SearchResult correspond à SearchResult.
type SearchResult struct {
	Titles          []SearchTitle   `json:"titles"`
	Sections        []SearchSection `json:"sections"`
	Nature          string          `json:"nature"`
	Etat            string          `json:"etat"`
	Origin          string          `json:"origin"`
	Nor             string          `json:"nor"`
	Num             FlexString      `json:"num"`
	Idcc            FlexString      `json:"idcc"`
	RaisonSociale   string          `json:"raisonSociale"`
	Date            Date            `json:"date"`
	DateSignature   Date            `json:"dateSignature"`
	DatePublication Date            `json:"datePublication"`
	Text            string          `json:"text"`
	ResumePrincipal []string        `json:"resumePrincipal"`
}

// SearchTitle correspond à SearchTitle.
type SearchTitle struct {
	ID          string `json:"id"`
	Cid         string `json:"cid"`
	Title       string `json:"title"`
	LegalStatus string `json:"legalStatus"`
	StartDate   Date   `json:"startDate"`
	EndDate     Date   `json:"endDate"`
	Nature      string `json:"nature"`
}

// SearchSection correspond à SearchSection.
type SearchSection struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	LegalStatus string          `json:"legalStatus"`
	Extracts    []SearchExtract `json:"extracts"`
}

// SearchExtract correspond à SearchExtract.
type SearchExtract struct {
	ID          string     `json:"id"`
	Num         FlexString `json:"num"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	LegalStatus string     `json:"legalStatus"`
	DateDebut   Date       `json:"dateDebut"`
	DateFin     Date       `json:"dateFin"`
	Values      []string   `json:"values"`
}

// ---------------------------------------------------------------------------
// Suggestions (/suggest)
// ---------------------------------------------------------------------------

// SuggestRequest correspond à SuggestSupplyRequest.
type SuggestRequest struct {
	SearchText    string   `json:"searchText"`
	Supplies      []string `json:"supplies,omitempty"`
	DocumentsDits bool     `json:"documentsDits"`
}

// SuggestResponse correspond à SuggestResponse. La structure de « results »
// (maps imbriquées par fonds puis par identifiant) étant peu documentée, elle
// est parcourue de façon générique par [SuggestResponse.Values].
type SuggestResponse struct {
	TotalResultNumber int             `json:"totalResultNumber"`
	Results           json.RawMessage `json:"results"`
}

// Values extrait toutes les suggestions, quel que soit leur niveau d'imbrication.
func (r *SuggestResponse) Values() []SuggestValue {
	var root any
	if len(r.Results) == 0 || json.Unmarshal(r.Results, &root) != nil {
		return nil
	}
	var out []SuggestValue
	seen := make(map[string]bool)
	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			if _, ok := v["label"].(string); ok {
				raw, _ := json.Marshal(v)
				var sv SuggestValue
				if json.Unmarshal(raw, &sv) == nil && !seen[sv.ID+sv.Label] {
					seen[sv.ID+sv.Label] = true
					out = append(out, sv)
				}
				return
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(root)
	return out
}

// SuggestValue correspond à SuggestValue.
type SuggestValue struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Nature       string   `json:"nature"`
	Origin       string   `json:"origin"`
	IDTexte      string   `json:"idTexte"`
	Appellations []string `json:"appellations"`
}

// ---------------------------------------------------------------------------
// Articles
// ---------------------------------------------------------------------------

// Article correspond au DTO Article (réduit).
type Article struct {
	ID                 string           `json:"id"`
	Cid                string           `json:"cid"`
	Num                FlexString       `json:"num"`
	Etat               string           `json:"etat"`
	DateDebut          Date             `json:"dateDebut"`
	DateFin            Date             `json:"dateFin"`
	Texte              string           `json:"texte"`
	TexteHTML          string           `json:"texteHtml"`
	Nota               string           `json:"nota"`
	NotaHTML           string           `json:"notaHtml"`
	VersionArticle     string           `json:"versionArticle"`
	FullSectionsTitre  string           `json:"fullSectionsTitre"`
	SectionParentTitre string           `json:"sectionParentTitre"`
	SectionParentCid   string           `json:"sectionParentCid"`
	TextTitles         []ArticleTextRef `json:"textTitles"`
	ArticleVersions    []ArticleVersion `json:"articleVersions"`
}

// ArticleTextRef correspond à TextTitle : texte auquel appartient l'article.
type ArticleTextRef struct {
	ID        string `json:"id"`
	Cid       string `json:"cid"`
	Titre     string `json:"titre"`
	TitreLong string `json:"titreLong"`
	Nature    string `json:"nature"`
	Etat      string `json:"etat"`
}

// ArticleVersion correspond à ArticleVersion.
type ArticleVersion struct {
	ID        string `json:"id"`
	Etat      string `json:"etat"`
	Version   string `json:"version"`
	Numero    string `json:"numero"`
	DateDebut Date   `json:"dateDebut"`
	DateFin   Date   `json:"dateFin"`
}

type articleResponse struct {
	Article *Article `json:"article"`
}

type articleListResponse struct {
	ListArticle []Article `json:"listArticle"`
}

// ---------------------------------------------------------------------------
// Textes structurés (LEGI, JORF, KALI)
// ---------------------------------------------------------------------------

// Text regroupe les champs communs à ConsultTextResponse, ConsultJorfResponse
// et ConsultKaliTextResponse.
type Text struct {
	ID               string        `json:"id"`
	Cid              string        `json:"cid"`
	Title            string        `json:"title"`
	Nature           string        `json:"nature"`
	Etat             string        `json:"etat"`
	Nor              string        `json:"nor"`
	Eli              string        `json:"eli"`
	TextNumber       FlexString    `json:"textNumber"`
	DateTexte        Date          `json:"dateTexte"`
	DateParution     Date          `json:"dateParution"`
	DateDebutVersion Date          `json:"dateDebutVersion"`
	DateFinVersion   Date          `json:"dateFinVersion"`
	Visa             string        `json:"visa"`
	VisasHTML        string        `json:"visasHtml"`
	Signers          string        `json:"signers"`
	Notice           string        `json:"notice"`
	Nota             string        `json:"nota"`
	Resume           string        `json:"resume"`
	Articles         []TextArticle `json:"articles"`
	Sections         []TextSection `json:"sections"`
}

// TextSection correspond à ConsultSection (récursif).
type TextSection struct {
	ID        string        `json:"id"`
	Cid       string        `json:"cid"`
	Title     string        `json:"title"`
	Etat      string        `json:"etat"`
	DateDebut Date          `json:"dateDebut"`
	DateFin   Date          `json:"dateFin"`
	Articles  []TextArticle `json:"articles"`
	Sections  []TextSection `json:"sections"`
}

// TextArticle correspond à ConsultArticle.
type TextArticle struct {
	ID        string     `json:"id"`
	Cid       string     `json:"cid"`
	Num       FlexString `json:"num"`
	Etat      string     `json:"etat"`
	DateDebut Date       `json:"dateDebut"`
	DateFin   Date       `json:"dateFin"`
	Content   string     `json:"content"`
	Nota      string     `json:"nota"`
}

// KaliContainer correspond à ConsultKaliContResponse (convention collective).
type KaliContainer struct {
	ID       string        `json:"id"`
	Titre    string        `json:"titre"`
	Num      FlexString    `json:"num"`
	Nature   string        `json:"nature"`
	Sections []TextSection `json:"sections"`
}

// ---------------------------------------------------------------------------
// Textes simples (JURI, CNIL)
// ---------------------------------------------------------------------------

// SimpleText correspond à TexteSimple (réduit).
type SimpleText struct {
	ID            string        `json:"id"`
	Cid           string        `json:"cid"`
	Titre         string        `json:"titre"`
	TitreLong     string        `json:"titreLong"`
	Nature        string        `json:"nature"`
	Etat          string        `json:"etat"`
	Juridiction   string        `json:"juridiction"`
	Formation     string        `json:"formation"`
	Solution      string        `json:"solution"`
	Ecli          string        `json:"ecli"`
	Nor           string        `json:"nor"`
	Num           FlexString    `json:"num"`
	NumeroAffaire []string      `json:"numeroAffaire"`
	DateTexte     Date          `json:"dateTexte"`
	President     string        `json:"president"`
	Avocats       string        `json:"avocats"`
	Rapporteur    string        `json:"rapporteur"`
	Texte         string        `json:"texte"`
	TexteHTML     string        `json:"texteHtml"`
	Sommaire      []TextSummary `json:"sommaire"`
	NatureDelib   string        `json:"natureDelib"`
}

// TextSummary correspond à TexteSommaire (sommaire d'une décision).
type TextSummary struct {
	ResumePrincipal string `json:"resumePrincipal"`
	Abstrats        string `json:"abstrats"`
}

type simpleTextResponse struct {
	Text *SimpleText `json:"text"`
}

// ---------------------------------------------------------------------------
// Accords d'entreprise et circulaires
// ---------------------------------------------------------------------------

// Agreement correspond à Accord (réduit).
type Agreement struct {
	ID             string       `json:"id"`
	TitreTexte     string       `json:"titreTexte"`
	RaisonSociale  string       `json:"raisonSociale"`
	Siret          string       `json:"siret"`
	CodeIdcc       FlexString   `json:"codeIdcc"`
	CodeApe        string       `json:"codeApe"`
	Nature         string       `json:"nature"`
	Secteur        string       `json:"secteur"`
	DateTexte      Date         `json:"dateTexte"`
	DateEffet      Date         `json:"dateEffet"`
	DateFin        Date         `json:"dateFin"`
	DateDepot      Date         `json:"dateDepot"`
	Themes         []LabelValue `json:"themes"`
	Syndicats      []LabelValue `json:"syndicats"`
	Data           string       `json:"data"`
	AttachementURL string       `json:"attachementUrl"`
}

// LabelValue correspond aux DTO Theme et Syndicat.
type LabelValue struct {
	Code    string `json:"code"`
	Libelle string `json:"libelle"`
}

type agreementResponse struct {
	Acco *Agreement `json:"acco"`
}

// Circular correspond à Circulaire (réduit).
type Circular struct {
	ID                  string     `json:"id"`
	Titre               string     `json:"titre"`
	Nor                 string     `json:"nor"`
	NumeroInterne       string     `json:"numeroInterne"`
	Etat                string     `json:"etat"`
	Resume              string     `json:"resume"`
	Auteur              string     `json:"auteur"`
	Signataire          string     `json:"signataire"`
	Destinataire        string     `json:"destinataire"`
	MinisteresDeposants []string   `json:"ministeresDeposants"`
	Domaines            []string   `json:"domaines"`
	DateSignature       Date       `json:"dateSignature"`
	DateMiseApplication Date       `json:"dateMiseApplication"`
	Opposable           FlexString `json:"opposable"`
	Data                string     `json:"data"`
	AttachmentURL       string     `json:"attachmentUrl"`
}

type circularResponse struct {
	Circulaire *Circular `json:"circulaire"`
}

// ---------------------------------------------------------------------------
// Listes (/list/*) et Journal officiel
// ---------------------------------------------------------------------------

// CodeListRequest correspond à CodeListRequest.
type CodeListRequest struct {
	CodeName   string   `json:"codeName,omitempty"`
	States     []string `json:"states,omitempty"`
	PageNumber int      `json:"pageNumber"`
	PageSize   int      `json:"pageSize"`
	Sort       string   `json:"sort,omitempty"`
}

// CodeListResponse correspond à CodeListResponse.
type CodeListResponse struct {
	TotalResultNumber int `json:"totalResultNumber"`
	Results           []struct {
		ID         string `json:"id"`
		Cid        string `json:"cid"`
		Titre      string `json:"titre"`
		Etat       string `json:"etat"`
		DateDebut  Date   `json:"dateDebut"`
		LastUpdate Date   `json:"lastUpdate"`
	} `json:"results"`
}

// LODAListRequest correspond à LODAListRequest.
type LODAListRequest struct {
	Natures         []string    `json:"natures,omitempty"`
	LegalStatus     []string    `json:"legalStatus,omitempty"`
	SignatureDate   *DatePeriod `json:"signatureDate,omitempty"`
	PublicationDate *DatePeriod `json:"publicationDate,omitempty"`
	Sort            string      `json:"sort,omitempty"`
	PageNumber      int         `json:"pageNumber"`
	PageSize        int         `json:"pageSize"`
}

// LODAListResponse correspond à LODAListResponse.
type LODAListResponse struct {
	TotalResultNumber int `json:"totalResultNumber"`
	Results           []struct {
		ID         string `json:"id"`
		Cid        string `json:"cid"`
		Titre      string `json:"titre"`
		Etat       string `json:"etat"`
		DateDebut  Date   `json:"dateDebut"`
		DateFin    Date   `json:"dateFin"`
		LastUpdate Date   `json:"lastUpdate"`
	} `json:"results"`
}

// JournalContainer correspond à Conteneur (numéro du Journal officiel).
type JournalContainer struct {
	ID        string     `json:"id"`
	Titre     string     `json:"titre"`
	Num       FlexString `json:"num"`
	Nature    string     `json:"nature"`
	DatePubli Date       `json:"datePubli"`
}

type journalContainersResponse struct {
	Containers []JournalContainer `json:"containers"`
}
