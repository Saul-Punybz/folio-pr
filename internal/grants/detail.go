package grants

// Detalle de una oportunidad (fetchOpportunity) y búsqueda filtrada (search2).
//
// Parámetros de la API CONFIRMADOS en vivo (nov 2026, sin llave, HTTP 200):
//
//	POST https://api.grants.gov/v1/api/search2
//	  keyword        texto libre ("" = sin texto; todo lo demás sigue filtrando)
//	  oppStatuses    "posted|forecasted" (el | es OR)
//	  eligibilities  códigos de applicant type separados por |: 12 = nonprofits
//	                 501(c)(3), 13 = nonprofits sin 501(c)(3), 25 = «Others»,
//	                 99 = sin restricción. Es OR: "12|25" devolvió 39 de 50 con
//	                 keyword "Puerto Rico". Otros: 07/11 tribus, 06/20 educación
//	                 superior, 23 pequeñas empresas, 21 individuos, 00-04 gobiernos.
//	  fundingCategories  CD, ED, HL, HO, AR, ENV, ELT, ISS, FN, DPR, HU... (OR con |)
//	  sortBy         "closeDate|asc" (también openDate|desc, etc.)
//	  rows           hasta 100 por llamada
//	  startRecordNum paginación (0-based)
//	  agencies       código de agencia (p. ej. HHS) — disponible, no se usa
//	  Respuesta: data.hitCount + data.oppHits[] (id, number, title, agency,
//	  openDate, closeDate MM/DD/YYYY, oppStatus) + facetas con conteos.
//
//	POST https://api.grants.gov/v1/api/fetchOpportunity   {"opportunityId":361754}
//	  Respuesta: errorcode/msg/token (token de sesión, no se guarda) y data con
//	  opportunityNumber, opportunityTitle, y UNO de:
//	    synopsis (posted): synopsisDesc (HTML), applicantEligibilityDesc (texto
//	      libre de elegibilidad), applicantTypes [{id,description}], awardCeiling,
//	      awardFloor, estimatedFunding (números), responseDate / postingDate
//	      ("Jan 11, 2027 12:00:00 AM EST"), agencyName, costSharing.
//	    forecast (forecasted): forecastDesc, applicantTypes, awardCeiling...,
//	      estApplicationResponseDate (sin texto de elegibilidad libre).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const defaultDetailEndpoint = "https://api.grants.gov/v1/api/fetchOpportunity"

// Códigos de elegibilidad de Grants.gov que se piden por defecto: nonprofits
// con y sin 501(c)(3), «otros» y sin restricción.
const DefaultEligibilities = "12|13|25|99"

// Query son los filtros de search2.
type Query struct {
	Keyword    string // texto libre; vacío = solo filtros
	Categories string // fundingCategories, p. ej. "CD|ED"
	Eligible   string // eligibilities; vacío = DefaultEligibilities
	Rows       int    // 1..100 (por defecto 25)
}

// SearchQuery busca con filtros (posted y forecasted, orden por cierre más próximo).
func (c *Client) SearchQuery(ctx context.Context, q Query) ([]Item, error) {
	if q.Rows <= 0 {
		q.Rows = 25
	}
	if q.Rows > 100 {
		q.Rows = 100
	}
	if q.Eligible == "" {
		q.Eligible = DefaultEligibilities
	}
	m := map[string]any{
		"keyword":       q.Keyword,
		"oppStatuses":   "forecasted|posted",
		"eligibilities": q.Eligible,
		"sortBy":        "closeDate|asc",
		"rows":          q.Rows,
	}
	if q.Categories != "" {
		m["fundingCategories"] = q.Categories
	}
	body, _ := json.Marshal(m)
	raw, err := c.post(ctx, c.endpoint, body)
	if err != nil {
		return nil, err
	}
	return parse(raw)
}

// Detail es el detalle de una oportunidad ya limpio.
type Detail struct {
	ID               string
	Number           string
	Title            string
	Agency           string
	Forecast         bool
	Description      string   // texto plano, sin HTML
	EligibilityText  string   // texto libre de elegibilidad (vacío en forecasts)
	ApplicantTypes   []string // ids de applicant type ("12", "25", ...)
	ApplicantLabels  []string
	AwardCeiling     int64 // dólares; 0 = no informado
	AwardFloor       int64
	EstimatedFunding int64
	PostDate         *time.Time
	CloseDate        *time.Time // en forecasts: fecha estimada de cierre
}

type detailResp struct {
	ErrorCode int    `json:"errorcode"`
	Msg       string `json:"msg"`
	Data      struct {
		ID            flexStr `json:"id"`
		Number        string  `json:"opportunityNumber"`
		Title         string  `json:"opportunityTitle"`
		AgencyDetails struct {
			AgencyName string `json:"agencyName"`
		} `json:"agencyDetails"`
		TopAgencyDetails struct {
			AgencyName string `json:"agencyName"`
		} `json:"topAgencyDetails"`
		Synopsis *detailBody `json:"synopsis"`
		Forecast *detailBody `json:"forecast"`
	} `json:"data"`
}

type detailBody struct {
	AgencyName    string  `json:"agencyName"`
	SynopsisDesc  string  `json:"synopsisDesc"`
	ForecastDesc  string  `json:"forecastDesc"`
	Eligibility   string  `json:"applicantEligibilityDesc"`
	AwardCeiling  flexNum `json:"awardCeiling"`
	AwardFloor    flexNum `json:"awardFloor"`
	EstFunding    flexNum `json:"estimatedFunding"`
	ResponseDate  string  `json:"responseDate"`
	EstResponse   string  `json:"estApplicationResponseDate"`
	PostingDate   string  `json:"postingDate"`
	ApplicantType []struct {
		ID          string `json:"id"`
		Description string `json:"description"`
	} `json:"applicantTypes"`
}

// Detail trae el detalle de una oportunidad (fetchOpportunity).
func (c *Client) Detail(ctx context.Context, id string) (*Detail, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("grants.gov: id inválido %q", id)
	}
	body, _ := json.Marshal(map[string]any{"opportunityId": n})
	raw, err := c.post(ctx, c.detailEndpoint, body)
	if err != nil {
		return nil, err
	}
	return parseDetail(raw)
}

func parseDetail(raw []byte) (*Detail, error) {
	var r detailResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("grants.gov: detalle no es JSON válido: %w", err)
	}
	if r.ErrorCode != 0 {
		return nil, fmt.Errorf("grants.gov: errorcode %d: %s", r.ErrorCode, r.Msg)
	}
	b := r.Data.Synopsis
	forecast := false
	if b == nil {
		b, forecast = r.Data.Forecast, true
	}
	if b == nil {
		return nil, fmt.Errorf("grants.gov: detalle sin synopsis ni forecast")
	}
	d := &Detail{
		ID: string(r.Data.ID), Number: r.Data.Number, Title: r.Data.Title,
		Agency: firstNonEmpty(r.Data.AgencyDetails.AgencyName, firstNonEmpty(r.Data.TopAgencyDetails.AgencyName, b.AgencyName)), Forecast: forecast,
		EligibilityText:  PlainText(b.Eligibility),
		AwardCeiling:     num(b.AwardCeiling),
		AwardFloor:       num(b.AwardFloor),
		EstimatedFunding: num(b.EstFunding),
		PostDate:         parseLongDate(b.PostingDate),
	}
	if forecast {
		d.Description = PlainText(b.ForecastDesc)
		d.CloseDate = parseLongDate(b.EstResponse)
	} else {
		d.Description = PlainText(b.SynopsisDesc)
		d.CloseDate = parseLongDate(b.ResponseDate)
	}
	for _, t := range b.ApplicantType {
		d.ApplicantTypes = append(d.ApplicantTypes, t.ID)
		d.ApplicantLabels = append(d.ApplicantLabels, t.Description)
	}
	return d, nil
}

// flexNum acepta número, texto numérico, "none" o null.
type flexNum string

func (f *flexNum) UnmarshalJSON(b []byte) error {
	*f = flexNum(strings.Trim(strings.TrimSpace(string(b)), `"`))
	return nil
}

// flexStr acepta número o texto.
type flexStr string

func (f *flexStr) UnmarshalJSON(b []byte) error {
	*f = flexStr(strings.Trim(strings.TrimSpace(string(b)), `"`))
	return nil
}

func num(n flexNum) int64 {
	s := strings.ReplaceAll(strings.TrimSpace(string(n)), ",", "")
	if i, err := strconv.ParseInt(s, 10, 64); err == nil && i > 0 {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 0 {
		return int64(f)
	}
	return 0
}

// parseLongDate lee "Jan 11, 2027 12:00:00 AM EST" (solo importa el día).
func parseLongDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if len(s) < 12 {
		return nil
	}
	// "Jan 11, 2027": los primeros 12 caracteres cuando el día tiene 2 dígitos.
	i := strings.Index(s, ", ")
	if i < 0 || len(s) < i+6 {
		return nil
	}
	t, err := time.Parse("Jan 2, 2006", s[:i+6])
	if err != nil {
		return nil
	}
	return &t
}

var (
	reTag   = regexp.MustCompile(`(?s)<[^>]*>`)
	reSpace = regexp.MustCompile(`\s+`)
)

// PlainText quita HTML y entidades comunes y colapsa espacios.
func PlainText(s string) string {
	s = strings.NewReplacer("<br>", " ", "<br/>", " ", "<br />", " ", "</p>", " ", "</li>", " ").Replace(s)
	s = reTag.ReplaceAllString(s, "")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`,
		"&#39;", "'", "&rsquo;", "'", "&lsquo;", "'", "&ldquo;", `"`, "&rdquo;", `"`,
		"&ndash;", "-", "&mdash;", "-").Replace(s)
	return strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
}

// Truncate corta a n runas en límite de palabra, con «…» si recortó.
func Truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	out := string(r)
	if i := strings.LastIndexByte(out, ' '); i > n/2 {
		out = out[:i]
	}
	return strings.TrimRight(out, " ,;:.") + "…"
}

func (c *Client) post(ctx context.Context, url string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("grants.gov: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("grants.gov: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("grants.gov: HTTP %d: %s", resp.StatusCode, snippet(raw))
	}
	return raw, nil
}

// NewWithEndpoints crea un Client apuntando a otros endpoints (pruebas).
func NewWithEndpoints(client *http.Client, search, detail string) *Client {
	c := New(client)
	c.endpoint, c.detailEndpoint = search, detail
	return c
}
