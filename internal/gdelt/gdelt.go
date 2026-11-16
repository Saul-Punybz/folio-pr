// Package gdelt consulta el archivo de noticias de GDELT (API DOC 2.0), que
// indexa artículos de medios de todo el mundo sin necesidad de llave.
//
// Datos verificados en vivo (16 nov 2026):
//   - Endpoint: https://api.gdeltproject.org/api/v2/doc/doc?query=...&mode=artlist&format=json
//   - Sin llave. Límite documentado: una petición cada 5 segundos; si se
//     excede responde HTTP 429 con un texto plano.
//   - Sin resultados responde 200 con "{}". Errores de consulta (p. ej. frase
//     demasiado corta) responden 200 con texto plano, no JSON.
//   - Sintaxis de query: frases exactas entre comillas ("Project Makers"),
//     exclusión con guion (-"New York"), alternativas con (a OR b) y filtros
//     como sourcelang:spanish o sourcecountry:RQ.
//   - maxrecords admite hasta 250; timespan acepta min, h, d, w, m (p. ej.
//     "1week", "3months").
package gdelt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL es el endpoint público de la API DOC 2.0.
	DefaultBaseURL = "https://api.gdeltproject.org/api/v2/doc/doc"
	// MinInterval es el mínimo entre dos peticiones (límite publicado por GDELT).
	MinInterval = 5 * time.Second
	// MaxRecords es el tope que acepta la API por petición.
	MaxRecords = 250

	requestTimeout = 90 * time.Second // las respuestas reales tardan 10-25 s
	userAgent      = "Folio/1.0 (+https://github.com/Saul-Punybz/folio)"
)

// ErrRateLimited indica que GDELT respondió 429: hay que esperar y reintentar.
var ErrRateLimited = errors.New("gdelt: límite de frecuencia (una petición cada 5 s)")

// Article es un artículo del archivo de GDELT.
type Article struct {
	URL      string
	Title    string
	Seen     time.Time
	Domain   string
	Language string
	Country  string // nombre del país de la fuente, como lo da GDELT ("Puerto Rico", "United States")
}

// Client hace las consultas respetando el espaciado entre peticiones.
type Client struct {
	BaseURL     string
	HTTP        *http.Client
	MinInterval time.Duration

	mu   sync.Mutex
	last time.Time
}

// NewClient crea un cliente con los valores por defecto.
func NewClient() *Client {
	// El servidor de GDELT tarda en el saludo TLS (se vieron >10 s), más que
	// el límite por defecto de Go.
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSHandshakeTimeout = 30 * time.Second
	return &Client{BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: requestTimeout, Transport: tr}, MinInterval: MinInterval}
}

var defaultClient = NewClient()

// Search consulta GDELT con el cliente compartido del paquete (que garantiza
// una petición cada 5 s entre todas las llamadas).
func Search(ctx context.Context, query, timespan string, max int) ([]Article, error) {
	return defaultClient.Search(ctx, query, timespan, max)
}

// Phrase devuelve s como frase exacta para usar dentro de una query.
func Phrase(s string) string {
	return `"` + strings.ReplaceAll(strings.TrimSpace(s), `"`, "") + `"`
}

// Search busca artículos que coincidan con query en la ventana timespan
// (p. ej. "1week", "3months"), hasta max resultados (1-250), los más recientes
// primero.
func (c *Client) Search(ctx context.Context, query, timespan string, max int) ([]Article, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("gdelt: query vacía")
	}
	if max <= 0 {
		max = 50
	}
	if max > MaxRecords {
		max = MaxRecords
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("mode", "artlist")
	q.Set("format", "json")
	q.Set("sort", "datedesc")
	q.Set("maxrecords", strconv.Itoa(max))
	if ts := strings.TrimSpace(timespan); ts != "" {
		q.Set("timespan", ts)
	}

	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("gdelt: crear petición: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gdelt: petición: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("gdelt: leer respuesta: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("gdelt: HTTP %d", resp.StatusCode)
	}
	return parse(body)
}

// wait respeta MinInterval entre el inicio de una petición y la siguiente.
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := c.MinInterval - time.Since(c.last); d > 0 && !c.last.IsZero() {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	c.last = time.Now()
	return nil
}

type rawResponse struct {
	Articles []struct {
		URL      string `json:"url"`
		Title    string `json:"title"`
		SeenDate string `json:"seendate"`
		Domain   string `json:"domain"`
		Language string `json:"language"`
		Country  string `json:"sourcecountry"`
	} `json:"articles"`
}

// parse decodifica la respuesta de artlist.
func parse(body []byte) ([]Article, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || trimmed == "{}" {
		return nil, nil
	}
	if trimmed[0] != '{' {
		// Errores de consulta: llegan como texto plano con HTTP 200.
		return nil, fmt.Errorf("gdelt: %s", truncate(trimmed, 200))
	}
	var raw rawResponse
	if err := json.Unmarshal([]byte(stripControl(trimmed)), &raw); err != nil {
		return nil, fmt.Errorf("gdelt: respuesta no es JSON válido: %w", err)
	}
	out := make([]Article, 0, len(raw.Articles))
	for _, a := range raw.Articles {
		if a.URL == "" {
			continue
		}
		seen, _ := time.Parse("20060102T150405Z", a.SeenDate)
		out = append(out, Article{
			URL: a.URL, Title: strings.TrimSpace(a.Title), Seen: seen,
			Domain: a.Domain, Language: a.Language, Country: a.Country,
		})
	}
	return out, nil
}

// stripControl quita caracteres de control que GDELT a veces deja dentro de
// los títulos y que rompen el JSON.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
