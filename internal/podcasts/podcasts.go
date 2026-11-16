// Package podcasts busca episodios de podcasts en la API pública de búsqueda
// de iTunes / Apple Podcasts.
//
// Datos verificados en vivo (16 nov 2026):
//   - Endpoint: https://itunes.apple.com/search?media=podcast&entity=podcastEpisode&term=...
//   - Sin llave. Apple documenta unas 20 peticiones por minuto por IP; al
//     excederlas responde 403 o 429.
//   - limit admite de 1 a 200. El parámetro country exige un código válido de
//     tienda (PR no lo es), así que no se usa.
//   - La búsqueda es difusa, no de frase exacta: "Oasis360" devuelve podcasts
//     de oasis y de 360. Use FilterMentions para quedarse con los episodios que
//     de verdad nombran el término.
//   - Campos usados por episodio: trackName, collectionName, trackViewUrl,
//     episodeUrl, releaseDate (RFC 3339) y description.
package podcasts

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
	// DefaultBaseURL es el endpoint público de búsqueda.
	DefaultBaseURL = "https://itunes.apple.com/search"
	// MinInterval espacia las peticiones para quedar bajo ~20 por minuto.
	MinInterval = 3 * time.Second
	// MaxLimit es el tope de resultados que acepta la API.
	MaxLimit = 200

	requestTimeout = 30 * time.Second
	userAgent      = "Folio/1.0 (+https://github.com/Saul-Punybz/folio)"
)

// ErrRateLimited indica que Apple limitó las peticiones (403/429).
var ErrRateLimited = errors.New("podcasts: límite de frecuencia de Apple Podcasts")

// Episode es un episodio de podcast.
type Episode struct {
	Title       string
	Show        string
	URL         string // página del episodio en Apple Podcasts (o el audio si falta)
	Released    time.Time
	Description string
}

// Client hace las consultas espaciando las peticiones.
type Client struct {
	BaseURL     string
	HTTP        *http.Client
	MinInterval time.Duration

	mu   sync.Mutex
	last time.Time
}

// NewClient crea un cliente con los valores por defecto.
func NewClient() *Client {
	return &Client{BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: requestTimeout}, MinInterval: MinInterval}
}

var defaultClient = NewClient()

// SearchEpisodes busca episodios con el cliente compartido del paquete.
func SearchEpisodes(ctx context.Context, term string, max int) ([]Episode, error) {
	return defaultClient.SearchEpisodes(ctx, term, max)
}

// SearchEpisodes busca hasta max episodios (1-200) relacionados con term.
func (c *Client) SearchEpisodes(ctx context.Context, term string, max int) ([]Episode, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, errors.New("podcasts: término vacío")
	}
	if max <= 0 {
		max = 25
	}
	if max > MaxLimit {
		max = MaxLimit
	}
	q := url.Values{}
	q.Set("media", "podcast")
	q.Set("entity", "podcastEpisode")
	q.Set("term", term)
	q.Set("limit", strconv.Itoa(max))

	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("podcasts: crear petición: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("podcasts: petición: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("podcasts: leer respuesta: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden:
		return nil, ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("podcasts: HTTP %d", resp.StatusCode)
	}
	return parse(body)
}

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
	Results []struct {
		Kind           string `json:"kind"`
		TrackName      string `json:"trackName"`
		CollectionName string `json:"collectionName"`
		TrackViewURL   string `json:"trackViewUrl"`
		EpisodeURL     string `json:"episodeUrl"`
		ReleaseDate    string `json:"releaseDate"`
		Description    string `json:"description"`
		ShortDesc      string `json:"shortDescription"`
	} `json:"results"`
}

func parse(body []byte) ([]Episode, error) {
	var raw rawResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("podcasts: respuesta no es JSON válido: %w", err)
	}
	out := make([]Episode, 0, len(raw.Results))
	for _, r := range raw.Results {
		if r.Kind != "" && r.Kind != "podcast-episode" {
			continue
		}
		link := r.TrackViewURL
		if link == "" {
			link = r.EpisodeURL
		}
		if link == "" || r.TrackName == "" {
			continue
		}
		released, _ := time.Parse(time.RFC3339, r.ReleaseDate)
		desc := r.Description
		if desc == "" {
			desc = r.ShortDesc
		}
		out = append(out, Episode{
			Title: strings.TrimSpace(r.TrackName), Show: strings.TrimSpace(r.CollectionName),
			URL: link, Released: released, Description: strings.TrimSpace(desc),
		})
	}
	return out, nil
}

// FilterMentions deja solo los episodios cuyo título, programa o descripción
// contienen phrase completa, sin importar mayúsculas ni acentos.
func FilterMentions(eps []Episode, phrase string) []Episode {
	p := fold(phrase)
	if p == "" {
		return eps
	}
	var out []Episode
	for _, e := range eps {
		if strings.Contains(fold(e.Title+" "+e.Show+" "+e.Description), p) {
			out = append(out, e)
		}
	}
	return out
}

var acentos = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u",
)

func fold(s string) string { return acentos.Replace(strings.ToLower(s)) }
