package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ArchiveItem es lo que el archivador necesita de una mención pendiente.
type ArchiveItem struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	SourceType  string
	Title       string
	URL         string
	Snippet     string
	PublishedAt *time.Time
	Attempts    int
	Status      string
	Edited      bool
	Authors     []string
	SiteName    string
	WaybackURL  string
	// NeedFetch: falta bajar la página. Si no, solo falta el enlace de respaldo.
	NeedFetch bool
}

// PendingArchive devuelve hasta limit menciones por archivar: las nunca
// intentadas, las «sin copia» con menos de MaxArchiveAttempts intentos, y las
// recientes que aún no tienen enlace en Internet Archive. exclude son las ya
// vistas en esta corrida. Las más nuevas primero.
func (s *WatchlistHitStore) PendingArchive(ctx context.Context, limit int, exclude []uuid.UUID) ([]ArchiveItem, error) {
	if exclude == nil {
		exclude = []uuid.UUID{}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, source_type, title, url, snippet, published_at, archive_attempts, archive_status,
		       edited, authors, site_name, wayback_url,
		       (archive_status = 'pendiente' OR (archive_status = 'sin_copia' AND archive_attempts < $3))
		FROM watchlist_hits
		WHERE id <> ALL($2)
		  AND (
		    archive_status = 'pendiente'
		    OR (archive_status = 'sin_copia' AND archive_attempts < $3)
		    OR (wayback_url = '' AND archive_status <> 'pendiente'
		        AND (wayback_checked_at IS NULL OR wayback_checked_at < now() - interval '3 days')
		        AND created_at > now() - interval '30 days')
		  )
		ORDER BY created_at DESC
		LIMIT $1
	`, limit, exclude, MaxArchiveAttempts)
	if err != nil {
		return nil, fmt.Errorf("archive pending: %w", err)
	}
	defer rows.Close()
	var out []ArchiveItem
	for rows.Next() {
		var it ArchiveItem
		var authors []byte
		if err := rows.Scan(&it.ID, &it.OrgID, &it.SourceType, &it.Title, &it.URL, &it.Snippet, &it.PublishedAt,
			&it.Attempts, &it.Status, &it.Edited, &authors, &it.SiteName, &it.WaybackURL, &it.NeedFetch); err != nil {
			return nil, fmt.Errorf("archive pending scan: %w", err)
		}
		it.Authors = scanJSONStringSlice(authors)
		out = append(out, it)
	}
	return out, rows.Err()
}

// ArchiveResult es lo que dejó un intento de archivar. Los campos vacíos no
// pisan lo que ya hay; si la usuaria corrigió la mención (edited), tampoco se
// toca título, medio, autores, fecha ni resumen.
type ArchiveResult struct {
	Status       string // "" = no cambia (solo se buscó el enlace de respaldo)
	Err          string
	Fetched      bool // se hizo un intento de bajar la página: suma un intento
	Title        string
	SiteName     string
	Authors      []string
	PublishedAt  *time.Time
	Lang         string
	CanonicalURL string
	ResolvedURL  string
	ImageURL     string
	Summary      string
	ContentText  string
	WaybackURL   string
	WaybackTried bool
}

// SaveArchive guarda el resultado de un intento.
func (s *WatchlistHitStore) SaveArchive(ctx context.Context, id uuid.UUID, r ArchiveResult) error {
	authors, _ := json.Marshal(nonNilStrings(r.Authors))
	inc := 0
	if r.Fetched {
		inc = 1
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE watchlist_hits SET
		  archive_status   = CASE WHEN $2::text = '' THEN archive_status ELSE $2::text END,
		  archive_error    = CASE WHEN $2::text = '' THEN archive_error ELSE $3::text END,
		  archive_attempts = archive_attempts + $4::int,
		  archived_at      = CASE WHEN $2::text IN ('copia', 'parcial') THEN now() ELSE archived_at END,
		  title            = CASE WHEN NOT edited AND $5::text <> '' THEN $5::text ELSE title END,
		  site_name        = CASE WHEN NOT edited AND $6::text <> '' THEN $6::text ELSE site_name END,
		  authors          = CASE WHEN NOT edited AND $7::jsonb <> '[]'::jsonb THEN $7::jsonb ELSE authors END,
		  published_at     = CASE WHEN NOT edited AND $8::timestamptz IS NOT NULL THEN $8::timestamptz ELSE published_at END,
		  lang             = CASE WHEN $9::text <> '' THEN $9::text ELSE lang END,
		  canonical_url    = CASE WHEN $10::text <> '' THEN $10::text ELSE canonical_url END,
		  resolved_url     = CASE WHEN $11::text <> '' THEN $11::text ELSE resolved_url END,
		  image_url        = CASE WHEN $12::text <> '' THEN $12::text ELSE image_url END,
		  summary          = CASE WHEN NOT edited AND $13::text <> '' THEN $13::text ELSE summary END,
		  content_text     = CASE WHEN $14::text <> '' THEN $14::text ELSE content_text END,
		  wayback_url      = CASE WHEN $15::text <> '' THEN $15::text ELSE wayback_url END,
		  wayback_checked_at = CASE WHEN $16::bool THEN now() ELSE wayback_checked_at END
		WHERE id = $1
	`, id, r.Status, r.Err, inc, r.Title, r.SiteName, authors, r.PublishedAt, r.Lang, r.CanonicalURL,
		r.ResolvedURL, r.ImageURL, r.Summary, r.ContentText, r.WaybackURL, r.WaybackTried)
	if err != nil {
		return fmt.Errorf("archive save: %w", err)
	}
	return nil
}

// MetaEdit son los metadatos que la usuaria puede corregir.
type MetaEdit struct {
	Title        string
	SiteName     string
	Authors      []string
	PublishedAt  *time.Time
	Summary      string
	CanonicalURL string
}

// UpdateMeta guarda las correcciones de la usuaria y marca la mención como
// editada (la copia automática ya no pisa esos campos).
func (s *WatchlistHitStore) UpdateMeta(ctx context.Context, id, userID uuid.UUID, e MetaEdit) error {
	authors, _ := json.Marshal(nonNilStrings(e.Authors))
	tag, err := s.pool.Exec(ctx, `
		UPDATE watchlist_hits wh SET title = $3, site_name = $4, authors = $5, published_at = $6,
		       summary = $7, canonical_url = $8, edited = true
		FROM watchlist_orgs wo
		WHERE wh.id = $1 AND wh.org_id = wo.id AND wo.user_id = $2
	`, id, userID, e.Title, e.SiteName, authors, e.PublishedAt, e.Summary, e.CanonicalURL)
	if err != nil {
		return fmt.Errorf("archive update meta: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrHitNotFound
	}
	return nil
}

// GetForUser devuelve una mención de esa usuaria (sin el texto).
func (s *WatchlistHitStore) GetForUser(ctx context.Context, hitID, userID uuid.UUID) (*WatchlistHit, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+hitColumns+`
		FROM watchlist_hits wh JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wh.id = $1 AND wo.user_id = $2
	`, hitID, userID)
	h, err := scanHit(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHitNotFound
		}
		return nil, err
	}
	return &h, nil
}

// GetText devuelve el texto archivado de una mención de esa usuaria.
func (s *WatchlistHitStore) GetText(ctx context.Context, hitID, userID uuid.UUID) (string, error) {
	var t string
	err := s.pool.QueryRow(ctx, `
		SELECT wh.content_text FROM watchlist_hits wh JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wh.id = $1 AND wo.user_id = $2
	`, hitID, userID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrHitNotFound
	}
	return t, err
}

// ArchiveFilter filtra el archivo de una marca.
type ArchiveFilter struct {
	Query  string
	From   *time.Time // incluye
	To     *time.Time // excluye
	Source string     // red: noticias, youtube, bluesky…
	Limit  int
	Offset int
}

// ArchiveStats resume el archivo de una marca.
type ArchiveStats struct {
	Total      int `json:"total"`
	Copia      int `json:"copia"`
	Parcial    int `json:"parcial"`
	SinCopia   int `json:"sin_copia"`
	Pendiente  int `json:"pendiente"`
	ConAutor   int `json:"con_autor"`
	ConFecha   int `json:"con_fecha"`
	ConWayback int `json:"con_wayback"`
}

// ListArchive devuelve las menciones de una marca de esa usuaria, de siempre
// (nuevas y vistas), más recientes primero, con el total que cumple el filtro.
func (s *WatchlistHitStore) ListArchive(ctx context.Context, orgID, userID uuid.UUID, f ArchiveFilter) ([]WatchlistHit, int, error) {
	if f.Limit <= 0 || f.Limit > 5000 {
		f.Limit = 1000
	}
	where := `wh.org_id = $1 AND wo.user_id = $2
		AND ($3::text = '' OR unaccent(lower(wh.title || ' ' || wh.site_name || ' ' || wh.authors::text || ' ' || wh.summary || ' ' || wh.snippet || ' ' || wh.content_text))
		     LIKE '%' || unaccent(lower($3::text)) || '%')
		AND ($4::timestamptz IS NULL OR COALESCE(wh.published_at, wh.created_at) >= $4::timestamptz)
		AND ($5::timestamptz IS NULL OR COALESCE(wh.published_at, wh.created_at) < $5::timestamptz)
		AND ($6::text = '' OR wh.network = $6::text)`
	args := []any{orgID, userID, strings.TrimSpace(f.Query), f.From, f.To, f.Source}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM watchlist_hits wh JOIN watchlist_orgs wo ON wo.id = wh.org_id WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("archive count: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+hitColumns+`
		FROM watchlist_hits wh JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE `+where+`
		ORDER BY COALESCE(wh.published_at, wh.created_at) DESC, wh.created_at DESC
		LIMIT $7 OFFSET $8
	`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("archive list: %w", err)
	}
	defer rows.Close()
	hits, err := scanHitRows(rows)
	return hits, total, err
}

// ArchiveStatsByOrg cuenta el archivo de una marca de esa usuaria.
func (s *WatchlistHitStore) ArchiveStatsByOrg(ctx context.Context, orgID, userID uuid.UUID) (ArchiveStats, error) {
	var st ArchiveStats
	err := s.pool.QueryRow(ctx, `
		SELECT count(*),
		  count(*) FILTER (WHERE archive_status = 'copia'),
		  count(*) FILTER (WHERE archive_status = 'parcial'),
		  count(*) FILTER (WHERE archive_status = 'sin_copia'),
		  count(*) FILTER (WHERE archive_status = 'pendiente'),
		  count(*) FILTER (WHERE authors <> '[]'::jsonb),
		  count(*) FILTER (WHERE published_at IS NOT NULL),
		  count(*) FILTER (WHERE wayback_url <> '')
		FROM watchlist_hits wh JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wh.org_id = $1 AND wo.user_id = $2
	`, orgID, userID).Scan(&st.Total, &st.Copia, &st.Parcial, &st.SinCopia, &st.Pendiente, &st.ConAutor, &st.ConFecha, &st.ConWayback)
	return st, err
}

// RetryArchive devuelve a «pendiente» una mención de esa usuaria (botón
// «Reintentar copia»): reinicia los intentos.
func (s *WatchlistHitStore) RetryArchive(ctx context.Context, hitID, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE watchlist_hits wh SET archive_status = 'pendiente', archive_attempts = 0, archive_error = ''
		FROM watchlist_orgs wo
		WHERE wh.id = $1 AND wh.org_id = wo.id AND wo.user_id = $2 AND wh.archive_status <> 'copia'
	`, hitID, userID)
	if err != nil {
		return fmt.Errorf("archive retry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrHitNotFound
	}
	return nil
}

// Pool expone el pool (pruebas de integración).
func (s *WatchlistHitStore) Pool() *pgxpool.Pool { return s.pool }
