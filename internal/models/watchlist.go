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

// Tipos de lo que se vigila.
const (
	KindMarca   = "marca"
	KindTema    = "tema"
	KindPersona = "persona"

	// MaxActiveBrands: tope de marcas activas por usuaria.
	MaxActiveBrands = 10
)

// ErrBrandLimit: ya hay MaxActiveBrands marcas activas.
var ErrBrandLimit = errors.New("límite de marcas activas")

// ErrOrgNotFound: no existe o no es de esta usuaria.
var ErrOrgNotFound = errors.New("watchlist org not found")

// Social es un perfil en una red social.
type Social struct {
	Network string `json:"network"`
	Handle  string `json:"handle"`
	URL     string `json:"url"`
}

// Person es una persona clave de una marca.
type Person struct {
	Name string `json:"name"`
	Role string `json:"role"`
	URL  string `json:"url"`
}

// Evidence dice de dónde salió un dato de la plantilla.
type Evidence struct {
	Dato string `json:"dato"`
	URL  string `json:"url"`
}

// WatchlistOrg es una marca, un tema o una persona que se vigila.
type WatchlistOrg struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	Name            string     `json:"name"`
	Kind            string     `json:"kind"`
	Website         string     `json:"website"`
	Keywords        []string   `json:"keywords"`
	YouTubeChannels []string   `json:"youtube_channels"`
	Aliases         []string   `json:"aliases"`
	ExcludeTerms    []string   `json:"exclude_terms"`
	Hashtags        []string   `json:"hashtags"`
	Socials         []Social   `json:"socials"`
	ToConfirm       bool       `json:"to_confirm"`
	SourcesEvidence []Evidence `json:"sources_evidence"`
	AlertFeeds      []string   `json:"alert_feeds"`
	ContextAliases  []string   `json:"context_aliases"`
	People          []Person   `json:"people"`
	Active          bool       `json:"active"`
	Unseen          int        `json:"unseen"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// IsBrand dice si es una marca.
func (o WatchlistOrg) IsBrand() bool { return o.Kind == KindMarca }

// WatchlistHit represents a single mention found by a scanning agent.
type WatchlistHit struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	OrgName     string     `json:"org_name,omitempty"`
	SourceType  string     `json:"source_type"`
	Network     string     `json:"network"`
	Hashtag     string     `json:"matched_hashtag"`
	Person      string     `json:"matched_person"`
	TitleKey    string     `json:"-"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	URLHash     string     `json:"url_hash"`
	Snippet     string     `json:"snippet"`
	Sentiment   string     `json:"sentiment"`
	AIDraft     *string    `json:"ai_draft"`
	Seen        bool       `json:"seen"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// NetworkFor da la red de una fuente: las de noticias se agrupan juntas.
func NetworkFor(sourceType string) string {
	switch sourceType {
	case "google_news", "bing_news", "local", "alertas":
		return "noticias"
	default:
		return sourceType
	}
}

// ── WatchlistOrgStore ────────────────────────────────────────────

type WatchlistOrgStore struct {
	pool *pgxpool.Pool
}

func NewWatchlistOrgStore(pool *pgxpool.Pool) *WatchlistOrgStore {
	return &WatchlistOrgStore{pool: pool}
}

const orgColumns = `wo.id, wo.user_id, wo.name, wo.kind, wo.website, wo.keywords, wo.youtube_channels,
	wo.aliases, wo.exclude_terms, wo.hashtags, wo.socials, wo.to_confirm, wo.sources_evidence,
	wo.alert_feeds, wo.context_aliases, wo.people, wo.active, wo.created_at, wo.updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOrg(row rowScanner, extra ...any) (WatchlistOrg, error) {
	var o WatchlistOrg
	var kw, yt, al, ex, ht, so, ev, af, ca, pe []byte
	dest := []any{&o.ID, &o.UserID, &o.Name, &o.Kind, &o.Website, &kw, &yt, &al, &ex, &ht, &so, &o.ToConfirm, &ev,
		&af, &ca, &pe, &o.Active, &o.CreatedAt, &o.UpdatedAt}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return o, err
	}
	o.Keywords = scanJSONStringSlice(kw)
	o.YouTubeChannels = scanJSONStringSlice(yt)
	o.Aliases = scanJSONStringSlice(al)
	o.ExcludeTerms = scanJSONStringSlice(ex)
	o.Hashtags = scanJSONStringSlice(ht)
	o.AlertFeeds = scanJSONStringSlice(af)
	o.ContextAliases = scanJSONStringSlice(ca)
	_ = json.Unmarshal(so, &o.Socials)
	_ = json.Unmarshal(ev, &o.SourcesEvidence)
	_ = json.Unmarshal(pe, &o.People)
	o.normalizeNil()
	return o, nil
}

func (o *WatchlistOrg) normalizeNil() {
	if o.Keywords == nil {
		o.Keywords = []string{}
	}
	if o.YouTubeChannels == nil {
		o.YouTubeChannels = []string{}
	}
	if o.Aliases == nil {
		o.Aliases = []string{}
	}
	if o.ExcludeTerms == nil {
		o.ExcludeTerms = []string{}
	}
	if o.Hashtags == nil {
		o.Hashtags = []string{}
	}
	if o.Socials == nil {
		o.Socials = []Social{}
	}
	if o.People == nil {
		o.People = []Person{}
	}
	if o.ContextAliases == nil {
		o.ContextAliases = []string{}
	}
	if o.AlertFeeds == nil {
		o.AlertFeeds = []string{}
	}
	if o.SourcesEvidence == nil {
		o.SourcesEvidence = []Evidence{}
	}
	if o.Kind == "" {
		o.Kind = KindTema
	}
}

type orgJSON struct{ kw, yt, al, ex, ht, so, ev, af, ca, pe []byte }

func marshalOrg(o *WatchlistOrg) (orgJSON, error) {
	o.normalizeNil()
	var j orgJSON
	var err error
	for _, p := range []struct {
		dst *[]byte
		v   any
	}{{&j.kw, o.Keywords}, {&j.yt, o.YouTubeChannels}, {&j.al, o.Aliases}, {&j.ex, o.ExcludeTerms},
		{&j.ht, o.Hashtags}, {&j.so, o.Socials}, {&j.ev, o.SourcesEvidence}, {&j.af, o.AlertFeeds}, {&j.ca, o.ContextAliases}, {&j.pe, o.People}} {
		if *p.dst, err = json.Marshal(p.v); err != nil {
			return j, err
		}
	}
	return j, nil
}

func collectOrgs(rows pgx.Rows) ([]WatchlistOrg, error) {
	defer rows.Close()
	var orgs []WatchlistOrg
	for rows.Next() {
		o, err := scanOrg(rows)
		if err != nil {
			return nil, fmt.Errorf("watchlist orgs scan: %w", err)
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

// ListByUser devuelve todo lo que vigila la usuaria, con el conteo de
// menciones sin ver. Marcas primero.
func (s *WatchlistOrgStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]WatchlistOrg, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+orgColumns+`,
		       (SELECT count(*) FROM watchlist_hits wh WHERE wh.org_id = wo.id AND wh.seen = false)
		FROM watchlist_orgs wo
		WHERE wo.user_id = $1
		ORDER BY (wo.kind = 'marca') DESC, wo.name ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("watchlist orgs list: %w", err)
	}
	defer rows.Close()
	var orgs []WatchlistOrg
	for rows.Next() {
		var unseen int
		o, err := scanOrg(rows, &unseen)
		if err != nil {
			return nil, fmt.Errorf("watchlist orgs scan: %w", err)
		}
		o.Unseen = unseen
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

// ListActive devuelve todo lo activo, de todas las usuarias.
func (s *WatchlistOrgStore) ListActive(ctx context.Context) ([]WatchlistOrg, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+orgColumns+`
		FROM watchlist_orgs wo
		WHERE wo.active = true
		ORDER BY (wo.kind = 'marca') DESC, wo.name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("watchlist orgs list active: %w", err)
	}
	return collectOrgs(rows)
}

// GetForUser devuelve una fila si es de esa usuaria.
func (s *WatchlistOrgStore) GetForUser(ctx context.Context, id, userID uuid.UUID) (*WatchlistOrg, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+orgColumns+` FROM watchlist_orgs wo WHERE wo.id = $1 AND wo.user_id = $2`, id, userID)
	o, err := scanOrg(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("watchlist org get: %w", err)
	}
	return &o, nil
}

// checkBrandLimit falla con ErrBrandLimit si activar esta marca pasaría el tope.
// exclude es la fila que se está editando (uuid.Nil al crear).
func checkBrandLimit(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID, exclude uuid.UUID) error {
	var n int
	if err := q.QueryRow(ctx, `
		SELECT count(*) FROM watchlist_orgs
		WHERE user_id = $1 AND kind = 'marca' AND active = true AND id <> $2
	`, userID, exclude).Scan(&n); err != nil {
		return fmt.Errorf("watchlist brand count: %w", err)
	}
	if n >= MaxActiveBrands {
		return ErrBrandLimit
	}
	return nil
}

// brandLock serializa los cambios que cuentan marcas activas de una usuaria.
func brandLock(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7461638, hashtext($1::text))`, userID.String())
	return err
}

func (s *WatchlistOrgStore) Create(ctx context.Context, org *WatchlistOrg) error {
	if org.ID == uuid.Nil {
		org.ID = uuid.New()
	}
	j, err := marshalOrg(org)
	if err != nil {
		return fmt.Errorf("watchlist org create: marshal: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("watchlist org create: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op tras Commit
	if org.IsBrand() && org.Active {
		if err := brandLock(ctx, tx, org.UserID); err != nil {
			return fmt.Errorf("watchlist org create: lock: %w", err)
		}
		if err := checkBrandLimit(ctx, tx, org.UserID, uuid.Nil); err != nil {
			return err
		}
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO watchlist_orgs (id, user_id, name, kind, website, keywords, youtube_channels,
			aliases, exclude_terms, hashtags, socials, to_confirm, sources_evidence, active, alert_feeds, context_aliases, people)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		RETURNING created_at, updated_at
	`, org.ID, org.UserID, org.Name, org.Kind, org.Website, j.kw, j.yt, j.al, j.ex, j.ht, j.so,
		org.ToConfirm, j.ev, org.Active, j.af, j.ca, j.pe).Scan(&org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return fmt.Errorf("watchlist org create: %w", err)
	}
	return tx.Commit(ctx)
}

// Update guarda lo editable (no toca sources_evidence) si la fila es de
// org.UserID. Respeta el tope de marcas activas.
func (s *WatchlistOrgStore) Update(ctx context.Context, org *WatchlistOrg) error {
	j, err := marshalOrg(org)
	if err != nil {
		return fmt.Errorf("watchlist org update: marshal: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("watchlist org update: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op tras Commit
	if org.IsBrand() && org.Active {
		if err := brandLock(ctx, tx, org.UserID); err != nil {
			return fmt.Errorf("watchlist org update: lock: %w", err)
		}
		if err := checkBrandLimit(ctx, tx, org.UserID, org.ID); err != nil {
			return err
		}
	}
	err = tx.QueryRow(ctx, `
		UPDATE watchlist_orgs
		SET name = $3, kind = $4, website = $5, keywords = $6, youtube_channels = $7,
		    aliases = $8, exclude_terms = $9, hashtags = $10, socials = $11, to_confirm = $12,
		    active = $13, alert_feeds = $14, context_aliases = $15, people = $16, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING created_at, updated_at
	`, org.ID, org.UserID, org.Name, org.Kind, org.Website, j.kw, j.yt, j.al, j.ex, j.ht, j.so,
		org.ToConfirm, org.Active, j.af, j.ca, j.pe).Scan(&org.CreatedAt, &org.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOrgNotFound
	}
	if err != nil {
		return fmt.Errorf("watchlist org update: %w", err)
	}
	return tx.Commit(ctx)
}

// Delete borra la fila si es de userID.
func (s *WatchlistOrgStore) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM watchlist_orgs WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("watchlist org delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOrgNotFound
	}
	return nil
}

// ToggleActive pausa o activa la fila si es de userID (con el tope de marcas).
func (s *WatchlistOrgStore) ToggleActive(ctx context.Context, id, userID uuid.UUID, active bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("watchlist org toggle: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op tras Commit
	var kind string
	if err := tx.QueryRow(ctx, `SELECT kind FROM watchlist_orgs WHERE id = $1 AND user_id = $2`, id, userID).Scan(&kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrgNotFound
		}
		return fmt.Errorf("watchlist org toggle: %w", err)
	}
	if active && kind == KindMarca {
		if err := brandLock(ctx, tx, userID); err != nil {
			return fmt.Errorf("watchlist org toggle: lock: %w", err)
		}
		if err := checkBrandLimit(ctx, tx, userID, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE watchlist_orgs SET active = $3, updated_at = NOW() WHERE id = $1 AND user_id = $2`, id, userID, active); err != nil {
		return fmt.Errorf("watchlist org toggle: %w", err)
	}
	return tx.Commit(ctx)
}

// seededKey guarda en settings los nombres de plantilla ya sembrados, para
// que una marca que la usuaria borró no vuelva a aparecer.
const seededKey = "marcas_plantilla_sembradas"

// SeedTemplates siembra las plantillas de marca para la primera usuaria
// admin. Idempotente por nombre: no toca filas que ya existen (ediciones de
// la usuaria) ni recrea las que borró. Sin admin todavía no hace nada.
// Devuelve cuántas creó.
func (s *WatchlistOrgStore) SeedTemplates(ctx context.Context, templates []WatchlistOrg) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("seed marcas: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op tras Commit
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7461639)`); err != nil {
		return 0, fmt.Errorf("seed marcas: lock: %w", err)
	}
	var adminID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE role = 'admin' AND password_hash NOT LIKE '!%' ORDER BY created_at LIMIT 1`).Scan(&adminID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("seed marcas: admin: %w", err)
	}

	done := map[string]bool{}
	var raw string
	if err := tx.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, seededKey).Scan(&raw); err == nil {
		var names []string
		_ = json.Unmarshal([]byte(raw), &names)
		for _, n := range names {
			done[strings.ToLower(n)] = true
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("seed marcas: settings: %w", err)
	}

	created := 0
	for _, t := range templates {
		key := strings.ToLower(t.Name)
		if done[key] {
			continue
		}
		done[key] = true
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM watchlist_orgs WHERE user_id = $1 AND lower(name) = $2)`, adminID, key).Scan(&exists); err != nil {
			return 0, fmt.Errorf("seed marcas: exists: %w", err)
		}
		if exists {
			continue
		}
		o := t
		o.ID = uuid.New()
		o.UserID = adminID
		o.Kind = KindMarca
		o.Active = true
		if err := checkBrandLimit(ctx, tx, adminID, uuid.Nil); err != nil {
			o.Active = false // con el tope lleno, entra en pausa
		}
		j, err := marshalOrg(&o)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO watchlist_orgs (id, user_id, name, kind, website, keywords, youtube_channels,
				aliases, exclude_terms, hashtags, socials, to_confirm, sources_evidence, active,
				alert_feeds, context_aliases, people)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		`, o.ID, o.UserID, o.Name, o.Kind, o.Website, j.kw, j.yt, j.al, j.ex, j.ht, j.so, o.ToConfirm, j.ev, o.Active,
			j.af, j.ca, j.pe); err != nil {
			return 0, fmt.Errorf("seed marcas: insert %s: %w", o.Name, err)
		}
		created++
	}

	names := make([]string, 0, len(done))
	for n := range done {
		names = append(names, n)
	}
	b, _ := json.Marshal(names)
	if _, err := tx.Exec(ctx, `
		INSERT INTO settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
	`, seededKey, string(b)); err != nil {
		return 0, fmt.Errorf("seed marcas: guardar: %w", err)
	}
	return created, tx.Commit(ctx)
}

// ── WatchlistHitStore ────────────────────────────────────────────

type WatchlistHitStore struct {
	pool *pgxpool.Pool
}

func NewWatchlistHitStore(pool *pgxpool.Pool) *WatchlistHitStore {
	return &WatchlistHitStore{pool: pool}
}

func (s *WatchlistHitStore) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]WatchlistHit, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+hitColumns+`
		FROM watchlist_hits wh
		JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wo.user_id = $1
		ORDER BY wh.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("watchlist hits list: %w", err)
	}
	defer rows.Close()
	return scanHitRows(rows)
}

// ListByOrg devuelve las menciones de un org de esa usuaria.
func (s *WatchlistHitStore) ListByOrg(ctx context.Context, orgID, userID uuid.UUID, limit, offset int) ([]WatchlistHit, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+hitColumns+`
		FROM watchlist_hits wh
		JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wh.org_id = $1 AND wo.user_id = $4
		ORDER BY COALESCE(wh.published_at, wh.created_at) DESC
		LIMIT $2 OFFSET $3
	`, orgID, limit, offset, userID)
	if err != nil {
		return nil, fmt.Errorf("watchlist hits list by org: %w", err)
	}
	defer rows.Close()
	return scanHitRows(rows)
}

func (s *WatchlistHitStore) CountUnseenByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM watchlist_hits wh
		JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wo.user_id = $1 AND wh.seen = false
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("watchlist hits count unseen: %w", err)
	}
	return count, nil
}

func (s *WatchlistHitStore) Create(ctx context.Context, hit *WatchlistHit) error {
	if hit.ID == uuid.Nil {
		hit.ID = uuid.New()
	}
	if hit.Network == "" {
		hit.Network = NetworkFor(hit.SourceType)
	}
	if hit.Sentiment == "" {
		hit.Sentiment = "unknown"
	}
	// Deduplicado: por url_hash (en todas las marcas) y, dentro de la marca,
	// por título normalizado (la misma noticia con otra URL).
	err := s.pool.QueryRow(ctx, `
		INSERT INTO watchlist_hits (id, org_id, source_type, network, matched_hashtag, title, url, url_hash,
			snippet, sentiment, published_at, title_key, matched_person)
		SELECT $1::uuid, $2::uuid, $3::text, $4::text, $5::text, $6::text, $7::text, $8::text,
			$9::text, $10::text, $11::timestamptz, $12::text, $13::text
		WHERE $12::text = '' OR NOT EXISTS (
			SELECT 1 FROM watchlist_hits WHERE org_id = $2::uuid AND title_key = $12::text
		)
		ON CONFLICT (url_hash) DO NOTHING
		RETURNING created_at
	`, hit.ID, hit.OrgID, hit.SourceType, hit.Network, hit.Hashtag, hit.Title, hit.URL, hit.URLHash,
		hit.Snippet, hit.Sentiment, hit.PublishedAt, hit.TitleKey, hit.Person).Scan(&hit.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		hit.ID = uuid.Nil // duplicado
		return nil
	}
	if err != nil {
		hit.ID = uuid.Nil
		return fmt.Errorf("watchlist hit create: %w", err)
	}
	return nil
}

// MarkSeen marca una mención de esa usuaria como vista.
func (s *WatchlistHitStore) MarkSeen(ctx context.Context, hitID, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE watchlist_hits wh SET seen = true
		FROM watchlist_orgs wo
		WHERE wh.id = $1 AND wh.org_id = wo.id AND wo.user_id = $2
	`, hitID, userID)
	if err != nil {
		return fmt.Errorf("watchlist hit mark seen: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("watchlist hit not found: %s", hitID)
	}
	return nil
}

func (s *WatchlistHitStore) MarkAllSeenByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE watchlist_hits wh SET seen = true
		FROM watchlist_orgs wo
		WHERE wh.org_id = wo.id AND wo.user_id = $1 AND wh.seen = false
	`, userID)
	if err != nil {
		return 0, fmt.Errorf("watchlist hits mark all seen: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// MarkAllSeenByOrg marca vistas todas las menciones de un org de esa usuaria.
func (s *WatchlistHitStore) MarkAllSeenByOrg(ctx context.Context, orgID, userID uuid.UUID) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE watchlist_hits wh SET seen = true
		FROM watchlist_orgs wo
		WHERE wh.org_id = wo.id AND wo.id = $1 AND wo.user_id = $2 AND wh.seen = false
	`, orgID, userID)
	if err != nil {
		return 0, fmt.Errorf("watchlist hits mark org seen: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// Delete borra una mención de esa usuaria.
func (s *WatchlistHitStore) Delete(ctx context.Context, hitID, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM watchlist_hits wh USING watchlist_orgs wo
		WHERE wh.id = $1 AND wh.org_id = wo.id AND wo.user_id = $2
	`, hitID, userID)
	if err != nil {
		return fmt.Errorf("watchlist hit delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("watchlist hit not found: %s", hitID)
	}
	return nil
}

func (s *WatchlistHitStore) ListRecentByUser(ctx context.Context, userID uuid.UUID, limit int) ([]WatchlistHit, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+hitColumns+`
		FROM watchlist_hits wh
		JOIN watchlist_orgs wo ON wo.id = wh.org_id
		WHERE wo.user_id = $1
		ORDER BY wh.created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("watchlist hits list recent: %w", err)
	}
	defer rows.Close()
	return scanHitRows(rows)
}

const hitColumns = `wh.id, wh.org_id, wo.name, wh.source_type, wh.network, wh.matched_hashtag, wh.matched_person, wh.title, wh.url, wh.url_hash,
		       wh.snippet, wh.sentiment, wh.ai_draft, wh.seen, wh.published_at, wh.created_at`

// ── Helpers ──────────────────────────────────────────────────────

func scanHitRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]WatchlistHit, error) {
	var hits []WatchlistHit
	for rows.Next() {
		var h WatchlistHit
		if err := rows.Scan(
			&h.ID, &h.OrgID, &h.OrgName, &h.SourceType, &h.Network, &h.Hashtag, &h.Person, &h.Title, &h.URL, &h.URLHash,
			&h.Snippet, &h.Sentiment, &h.AIDraft, &h.Seen, &h.PublishedAt, &h.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("watchlist hit scan: %w", err)
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// scanJSONStringSlice unmarshals a JSONB column into a []string.
func scanJSONStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var s []string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	return s
}
