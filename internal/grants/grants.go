// Package grants busca oportunidades en la API pública de Grants.gov (search2, sin llave).
package grants

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultEndpoint = "https://api.grants.gov/v1/api/search2"
	publicURLFmt    = "https://www.grants.gov/search-results-detail/%s"
	dateLayout      = "01/02/2006" // Grants.gov usa MM/DD/YYYY
)

// Item es una oportunidad de financiamiento.
type Item struct {
	ID        string // id de la oportunidad en Grants.gov
	Number    string // opportunity number
	Title     string
	Agency    string
	OpenDate  *time.Time
	CloseDate *time.Time
	Status    string // "posted" | "forecasted" | "closed" | "archived"
	URL       string // enlace público a la oportunidad
}

// Client consulta Grants.gov.
type Client struct {
	client   *http.Client
	endpoint string
}

// New crea un Client. Con client nil usa uno con timeout de 20 s.
func New(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{client: client, endpoint: defaultEndpoint}
}

type response struct {
	ErrorCode int    `json:"errorcode"`
	Msg       string `json:"msg"`
	Data      struct {
		OppHits []struct {
			ID        string `json:"id"`
			Number    string `json:"number"`
			Title     string `json:"title"`
			Agency    string `json:"agency"`
			OpenDate  string `json:"openDate"`
			CloseDate string `json:"closeDate"`
			OppStatus string `json:"oppStatus"`
		} `json:"oppHits"`
	} `json:"data"`
}

// Search busca por palabra clave (solo oportunidades posted y forecasted).
func (c *Client) Search(ctx context.Context, keyword string, limit int) ([]Item, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	body, _ := json.Marshal(map[string]any{
		"keyword":     keyword,
		"oppStatuses": "forecasted|posted",
		"rows":        limit,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
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
	return parse(raw)
}

func parse(raw []byte) ([]Item, error) {
	var r response
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("grants.gov: respuesta no es JSON válido: %w", err)
	}
	if r.ErrorCode != 0 {
		return nil, fmt.Errorf("grants.gov: errorcode %d: %s", r.ErrorCode, r.Msg)
	}
	items := make([]Item, 0, len(r.Data.OppHits))
	for _, h := range r.Data.OppHits {
		st := strings.ToLower(h.OppStatus)
		if st != "posted" && st != "forecasted" {
			continue
		}
		items = append(items, Item{
			ID: h.ID, Number: h.Number, Title: h.Title, Agency: h.Agency,
			OpenDate: parseDate(h.OpenDate), CloseDate: parseDate(h.CloseDate),
			Status: st, URL: fmt.Sprintf(publicURLFmt, h.ID),
		})
	}
	return items, nil
}

func parseDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return nil
	}
	return &t
}

func snippet(b []byte) string {
	if len(b) > 300 {
		b = b[:300]
	}
	return strings.TrimSpace(string(b))
}
