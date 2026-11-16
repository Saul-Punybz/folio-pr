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

// Niveles y tipos de fuente de un fondo (contrato con el frontend).
var (
	Levels      = []string{"federal", "estatal", "municipal", "local"}
	SourceKinds = []string{"pagina", "pdf", "api"}
)

// ValidLevel dice si l es uno de los cuatro niveles.
func ValidLevel(l string) bool {
	for _, v := range Levels {
		if v == l {
			return true
		}
	}
	return false
}

// ErrDismissed: la huella de este fondo está en la lista de descartados.
var ErrDismissed = errors.New("opportunity dismissed")

// Opportunity es un fondo (subasta, RFP, grant…). reviewed=false es un
// «posible fondo» que la usuaria todavía no confirmó.
type Opportunity struct {
	ID            uuid.UUID         `json:"id"`
	PageID        *uuid.UUID        `json:"page_id,omitempty"`
	DomainID      *uuid.UUID        `json:"domain_id"`
	Title         string            `json:"title"`
	Agency        string            `json:"agency"`
	Summary       string            `json:"summary"`
	Amount        string            `json:"amount"`
	AmountCents   int64             `json:"amount_cents"`
	Deadline      *time.Time        `json:"deadline,omitempty"`
	ContactName   string            `json:"contact_name"`
	ContactEmail  string            `json:"contact_email"`
	ContactPhone  string            `json:"contact_phone"`
	Category      string            `json:"category"`
	Status        string            `json:"status"`
	SourceURL     string            `json:"source_url"`
	Confidence    int               `json:"confidence"`
	RawExtraction json.RawMessage   `json:"raw_extraction"`
	Reviewed      bool              `json:"reviewed"`
	Reference     string            `json:"reference"`
	Level         string            `json:"level"`
	SourceKind    string            `json:"source_kind"`
	DocumentURL   string            `json:"document_url"`
	DocScanned    bool              `json:"document_scanned"`
	DocLocal      bool              `json:"document_local"` // el PDF está en disco
	DocKept       bool              `json:"document_kept"`  // el PDF está guardado (la retención no lo borra)
	FundType      string            `json:"fund_type"`      // grant | subasta | rfp | otros (ver FundType)
	Evidence      map[string]string `json:"evidence"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`

	// Internos (no van al JSON).
	DedupeKey         string     `json:"-"`
	DocumentID        *uuid.UUID `json:"-"`
	DocumentRemoteURL string     `json:"-"`
	EditedFields      []string   `json:"-"`
}

// prAST: zona de Puerto Rico (UTC-4 todo el año).
var prAST = time.FixedZone("AST", -4*3600)

// OpportunityStatus calcula el estado por la fecha límite, por días en hora
// de Puerto Rico: vencida → closed; ≤7 días → closing_soon; si no, open.
// Sin fecha → unknown. Es la única regla: la usan el crawler, el PUT y
// (en SQL, con la misma ventana) RefreshStatuses.
func OpportunityStatus(deadline *time.Time, now time.Time) string {
	if deadline == nil {
		return "unknown"
	}
	day := func(t time.Time) time.Time {
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	// Una DATE de Postgres llega como medianoche UTC: su día es el de la fecha.
	dl := day(*deadline)
	if deadline.Location() != time.UTC {
		dl = day(deadline.In(prAST))
	}
	today := day(now.In(prAST))
	switch {
	case dl.Before(today):
		return "closed"
	case !dl.After(today.AddDate(0, 0, 7)):
		return "closing_soon"
	default:
		return "open"
	}
}

type OpportunityStore struct {
	pool *pgxpool.Pool
}

func NewOpportunityStore(pool *pgxpool.Pool) *OpportunityStore {
	return &OpportunityStore{pool: pool}
}

const oppColumns = `o.id, o.page_id, o.domain_id, o.title, o.agency, o.summary, o.amount, o.amount_cents,
	o.deadline, o.contact_name, o.contact_email, o.contact_phone, o.category, o.status, o.source_url,
	o.confidence, o.raw_extraction, o.reviewed, o.reference, o.level, o.source_kind,
	o.document_id, o.document_remote_url, o.document_scanned, o.evidence, o.dedupe_key,
	o.edited_fields, o.created_at, o.updated_at,
	COALESCE((SELECT d.kept FROM documents d WHERE d.id = o.document_id), false)`

func scanOpportunity(row pgx.Row) (*Opportunity, error) {
	var o Opportunity
	var ev []byte
	if err := row.Scan(
		&o.ID, &o.PageID, &o.DomainID, &o.Title, &o.Agency, &o.Summary, &o.Amount, &o.AmountCents,
		&o.Deadline, &o.ContactName, &o.ContactEmail, &o.ContactPhone, &o.Category, &o.Status, &o.SourceURL,
		&o.Confidence, &o.RawExtraction, &o.Reviewed, &o.Reference, &o.Level, &o.SourceKind,
		&o.DocumentID, &o.DocumentRemoteURL, &o.DocScanned, &ev, &o.DedupeKey,
		&o.EditedFields, &o.CreatedAt, &o.UpdatedAt, &o.DocKept,
	); err != nil {
		return nil, err
	}
	o.Evidence = map[string]string{}
	if len(ev) > 0 {
		_ = json.Unmarshal(ev, &o.Evidence)
	}
	o.FundType = FundType(o.Category, o.Reference, o.Title)
	o.DocLocal = o.DocumentID != nil
	switch {
	case o.DocumentID != nil:
		o.DocumentURL = "/api/opportunities/" + o.ID.String() + "/document"
	default:
		o.DocumentURL = o.DocumentRemoteURL
	}
	return &o, nil
}

func scanOpportunityRows(rows pgx.Rows) ([]Opportunity, error) {
	defer rows.Close()
	var opps []Opportunity
	for rows.Next() {
		o, err := scanOpportunity(rows)
		if err != nil {
			return nil, fmt.Errorf("opportunity scan: %w", err)
		}
		opps = append(opps, *o)
	}
	return opps, rows.Err()
}

// IsDismissed dice si una huella fue descartada.
func (s *OpportunityStore) IsDismissed(ctx context.Context, key string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM opportunity_dismissed WHERE dedupe_key = $1)`, key).Scan(&ok)
	return ok, err
}

// mergeText: un campo de texto en el upsert. Se conserva el valor viejo si
// se corrigió a mano, o si el nuevo viene vacío, o si el viejo vino de una
// fuente con más confianza.
func mergeText(col string) string {
	return fmt.Sprintf(`%[1]s = CASE
		WHEN '%[1]s' = ANY(opportunities.edited_fields) THEN opportunities.%[1]s
		WHEN EXCLUDED.%[1]s <> '' AND (opportunities.%[1]s = '' OR EXCLUDED.confidence >= opportunities.confidence) THEN EXCLUDED.%[1]s
		ELSE opportunities.%[1]s END`, col)
}

// Upsert inserta o actualiza por dedupe_key. Si la huella fue descartada no
// hace nada y devuelve ErrDismissed. created=true si la fila es nueva.
// Nunca toca reviewed, ni los campos corregidos a mano, ni borra un PDF ya
// adjunto: la misma convocatoria vista en dos URL queda en una sola fila
// con la mejor fuente.
func (s *OpportunityStore) Upsert(ctx context.Context, o *Opportunity) (created bool, err error) {
	if o.DedupeKey == "" {
		return false, errors.New("opportunity upsert: empty dedupe_key")
	}
	if dismissed, err := s.IsDismissed(ctx, o.DedupeKey); err != nil {
		return false, fmt.Errorf("opportunity upsert: %w", err)
	} else if dismissed {
		return false, ErrDismissed
	}
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	if o.RawExtraction == nil {
		o.RawExtraction = json.RawMessage("{}")
	}
	if o.Evidence == nil {
		o.Evidence = map[string]string{}
	}
	ev, _ := json.Marshal(o.Evidence)
	if o.Level == "" {
		o.Level = "estatal"
	}
	if o.SourceKind == "" {
		o.SourceKind = "pagina"
	}
	if o.Category == "" {
		o.Category = "other"
	}
	if o.Status == "" {
		o.Status = OpportunityStatus(o.Deadline, time.Now())
	}

	sets := []string{
		mergeText("title"), mergeText("agency"), mergeText("summary"), mergeText("reference"),
		mergeText("contact_name"), mergeText("contact_email"), mergeText("contact_phone"),
		mergeText("source_url"), mergeText("document_remote_url"),
		`amount = CASE WHEN 'amount' = ANY(opportunities.edited_fields) THEN opportunities.amount
			WHEN EXCLUDED.amount <> '' AND (opportunities.amount = '' OR EXCLUDED.confidence >= opportunities.confidence) THEN EXCLUDED.amount
			ELSE opportunities.amount END`,
		`amount_cents = CASE WHEN 'amount' = ANY(opportunities.edited_fields) THEN opportunities.amount_cents
			WHEN EXCLUDED.amount <> '' AND (opportunities.amount = '' OR EXCLUDED.confidence >= opportunities.confidence) THEN EXCLUDED.amount_cents
			ELSE opportunities.amount_cents END`,
		`deadline = CASE WHEN 'deadline' = ANY(opportunities.edited_fields) THEN opportunities.deadline
			ELSE COALESCE(EXCLUDED.deadline, opportunities.deadline) END`,
		`category = CASE WHEN 'category' = ANY(opportunities.edited_fields) THEN opportunities.category
			WHEN opportunities.category = 'other' OR EXCLUDED.confidence >= opportunities.confidence THEN EXCLUDED.category
			ELSE opportunities.category END`,
		`status = CASE WHEN 'status' = ANY(opportunities.edited_fields) THEN opportunities.status ELSE EXCLUDED.status END`,
		`page_id = COALESCE(EXCLUDED.page_id, opportunities.page_id)`,
		`domain_id = COALESCE(opportunities.domain_id, EXCLUDED.domain_id)`,
		`level = opportunities.level`,
		// La fuente "mejor": un PDF con texto gana a una página; se conserva el documento.
		`source_kind = CASE WHEN opportunities.source_kind = 'pdf' THEN 'pdf' ELSE EXCLUDED.source_kind END`,
		`document_id = COALESCE(EXCLUDED.document_id, opportunities.document_id)`,
		`document_scanned = CASE WHEN EXCLUDED.document_id IS NOT NULL THEN EXCLUDED.document_scanned ELSE opportunities.document_scanned END`,
		`evidence = opportunities.evidence || EXCLUDED.evidence`,
		`raw_extraction = EXCLUDED.raw_extraction`,
		`confidence = GREATEST(opportunities.confidence, EXCLUDED.confidence)`,
		`updated_at = NOW()`,
	}

	err = s.pool.QueryRow(ctx, `
		INSERT INTO opportunities (id, page_id, domain_id, title, agency, summary, amount,
		    amount_cents, deadline, contact_name, contact_email, contact_phone, category, status,
		    source_url, confidence, raw_extraction, reviewed, reference, level, source_kind,
		    document_id, document_remote_url, document_scanned, evidence, dedupe_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)
		ON CONFLICT (dedupe_key) DO UPDATE SET `+strings.Join(sets, ",\n")+`
		RETURNING id, (xmax = 0)
	`, o.ID, o.PageID, o.DomainID, o.Title, o.Agency, o.Summary, o.Amount,
		o.AmountCents, o.Deadline, o.ContactName, o.ContactEmail, o.ContactPhone, o.Category, o.Status,
		o.SourceURL, o.Confidence, o.RawExtraction, o.Reviewed, o.Reference, o.Level, o.SourceKind,
		o.DocumentID, o.DocumentRemoteURL, o.DocScanned, ev, o.DedupeKey,
	).Scan(&o.ID, &created)
	if err != nil {
		return false, fmt.Errorf("opportunity upsert: %w", err)
	}
	return created, nil
}

func (s *OpportunityStore) GetByID(ctx context.Context, id uuid.UUID) (*Opportunity, error) {
	o, err := scanOpportunity(s.pool.QueryRow(ctx, `SELECT `+oppColumns+` FROM opportunities o WHERE o.id = $1`, id))
	if err != nil {
		return nil, fmt.Errorf("opportunity get: %w", err)
	}
	return o, nil
}

// ListFilter: filtros de la lista (vacío = sin filtro).
type ListFilter struct {
	Status   string
	Category string
	Level    string
	DomainID *uuid.UUID
	Reviewed *bool
	Sort     string
	Limit    int
	Offset   int
}

func (f ListFilter) where(args *[]any) string {
	var conds []string
	add := func(cond string, v any) {
		*args = append(*args, v)
		conds = append(conds, fmt.Sprintf(cond, len(*args)))
	}
	if f.Status != "" {
		add("o.status = $%d", f.Status)
	}
	if f.Category != "" {
		add("o.category = $%d", f.Category)
	}
	if f.Level != "" {
		add("o.level = $%d", f.Level)
	}
	if f.DomainID != nil {
		add("o.domain_id = $%d", *f.DomainID)
	}
	if f.Reviewed != nil {
		add("o.reviewed = $%d", *f.Reviewed)
	}
	if len(conds) == 0 {
		return ""
	}
	return " AND " + strings.Join(conds, " AND ")
}

func (s *OpportunityStore) List(ctx context.Context, f ListFilter) ([]Opportunity, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	args := []any{}
	query := `SELECT ` + oppColumns + ` FROM opportunities o WHERE 1=1` + f.where(&args)
	switch f.Sort {
	case "deadline":
		query += " ORDER BY o.deadline ASC NULLS LAST, o.confidence DESC"
	case "amount":
		query += " ORDER BY o.amount_cents DESC"
	case "confidence":
		query += " ORDER BY o.confidence DESC, o.deadline ASC NULLS LAST"
	default:
		query += " ORDER BY o.created_at DESC"
	}
	args = append(args, f.Limit, f.Offset)
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("opportunities list: %w", err)
	}
	return scanOpportunityRows(rows)
}

// Count cuenta con los mismos filtros que List.
func (s *OpportunityStore) Count(ctx context.Context, f ListFilter) (int, error) {
	args := []any{}
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM opportunities o WHERE 1=1`+f.where(&args), args...).Scan(&n)
	return n, err
}

func (s *OpportunityStore) SearchFTS(ctx context.Context, query string, reviewed *bool, limit int) ([]Opportunity, error) {
	if limit <= 0 {
		limit = 50
	}
	args := []any{query, limit}
	extra := ""
	if reviewed != nil {
		args = append(args, *reviewed)
		extra = " AND o.reviewed = $3"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+oppColumns+`
		FROM opportunities o
		WHERE to_tsvector('simple', o.title || ' ' || o.agency || ' ' || o.summary || ' ' || o.reference)
		      @@ plainto_tsquery('simple', $1)`+extra+`
		ORDER BY o.confidence DESC
		LIMIT $2
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("opportunities search: %w", err)
	}
	return scanOpportunityRows(rows)
}

func (s *OpportunityStore) TotalCount(ctx context.Context) (int, error) {
	return s.Count(ctx, ListFilter{})
}

func (s *OpportunityStore) CountByStatus(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, COUNT(*) FROM opportunities GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("opportunities count by status: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

// PendingAndClosingSoon: pending = posibles fondos (reviewed=false) no
// cerrados; closing_soon = confirmados con fecha límite en ≤14 días.
func (s *OpportunityStore) PendingAndClosingSoon(ctx context.Context) (pending, closingSoon int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE NOT reviewed AND status <> 'closed'),
		  COUNT(*) FILTER (WHERE reviewed AND deadline IS NOT NULL
		                    AND deadline >= (NOW() AT TIME ZONE 'America/Puerto_Rico')::date
		                    AND deadline <= (NOW() AT TIME ZONE 'America/Puerto_Rico')::date + 14)
		FROM opportunities`).Scan(&pending, &closingSoon)
	return pending, closingSoon, err
}

// Patch es una corrección manual: cada puntero no nil se escribe (aunque sea
// vacío). ClearDeadline quita la fecha.
type Patch struct {
	Title, Agency, Summary, Amount, ContactName, ContactEmail, ContactPhone,
	Reference, Category, Status *string
	AmountCents   *int64
	Deadline      *time.Time
	ClearDeadline bool
}

// ApplyPatch escribe la corrección y anota los campos tocados en
// edited_fields para que el siguiente crawl no los pise. El estado se
// recalcula si cambió la fecha y no se mandó estado.
func (s *OpportunityStore) ApplyPatch(ctx context.Context, id uuid.UUID, p Patch) error {
	var sets []string
	var edited []string
	args := []any{id}
	set := func(col string, v any, field string) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
		if field != "" {
			edited = append(edited, field)
		}
	}
	str := func(col string, v *string) {
		if v != nil {
			set(col, strings.TrimSpace(*v), col)
		}
	}
	str("title", p.Title)
	str("agency", p.Agency)
	str("summary", p.Summary)
	str("contact_name", p.ContactName)
	str("contact_email", p.ContactEmail)
	str("contact_phone", p.ContactPhone)
	str("reference", p.Reference)
	str("category", p.Category)
	if p.Amount != nil {
		set("amount", strings.TrimSpace(*p.Amount), "amount")
		var cents int64
		if p.AmountCents != nil {
			cents = *p.AmountCents
		}
		set("amount_cents", cents, "")
	}
	deadlineChanged := false
	if p.ClearDeadline {
		set("deadline", nil, "deadline")
		deadlineChanged = true
	} else if p.Deadline != nil {
		set("deadline", *p.Deadline, "deadline")
		deadlineChanged = true
	}
	if p.Status != nil {
		set("status", *p.Status, "status")
	} else if deadlineChanged {
		var dl *time.Time
		if !p.ClearDeadline {
			dl = p.Deadline
		}
		set("status", OpportunityStatus(dl, time.Now()), "")
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, edited)
	sets = append(sets, fmt.Sprintf(`edited_fields = ARRAY(SELECT DISTINCT unnest(edited_fields || $%d::text[]))`, len(args)))
	sets = append(sets, "updated_at = NOW()")
	tag, err := s.pool.Exec(ctx, `UPDATE opportunities SET `+strings.Join(sets, ", ")+` WHERE id = $1`, args...)
	if err != nil {
		return fmt.Errorf("opportunity patch: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// Dismiss descarta un fondo: guarda su huella (para que el próximo crawl no
// lo vuelva a crear) y borra la fila.
func (s *OpportunityStore) Dismiss(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var key, title, src string
	if err := tx.QueryRow(ctx, `SELECT dedupe_key, title, source_url FROM opportunities WHERE id = $1`, id).
		Scan(&key, &title, &src); err != nil {
		return fmt.Errorf("opportunity dismiss: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO opportunity_dismissed (dedupe_key, title, source_url) VALUES ($1, $2, $3)
		ON CONFLICT (dedupe_key) DO NOTHING`, key, title, src); err != nil {
		return fmt.Errorf("opportunity dismiss: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM opportunities WHERE id = $1`, id); err != nil {
		return fmt.Errorf("opportunity dismiss: %w", err)
	}
	return tx.Commit(ctx)
}

// Unreview saca un fondo de Guardados: vuelve a Por revisar. Su PDF no se
// toca (si estaba guardado, sigue en «PDF guardados» hasta que ella lo suelte).
func (s *OpportunityStore) Unreview(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE opportunities SET reviewed = false, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("opportunity unreview: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// SetDocument enlaza un fondo con un documento recién bajado.
func (s *OpportunityStore) SetDocument(ctx context.Context, id, docID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE opportunities SET document_id = $2, updated_at = NOW() WHERE id = $1`, id, docID)
	if err != nil {
		return fmt.Errorf("opportunity set document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *OpportunityStore) MarkReviewed(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE opportunities SET reviewed = true, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("opportunity mark reviewed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DocumentFor devuelve la ruta relativa (a la carpeta de datos) del PDF
// local de un fondo.
func (s *OpportunityStore) DocumentFor(ctx context.Context, id uuid.UUID) (relPath, sha string, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT d.path, d.sha256 FROM opportunities o JOIN documents d ON d.id = o.document_id
		WHERE o.id = $1`, id).Scan(&relPath, &sha)
	return relPath, sha, err
}

// RefreshStatuses recalcula estados por fecha (misma regla que
// OpportunityStatus: vencida → closed, ≤7 días → closing_soon, si no open),
// sin tocar los estados corregidos a mano.
func (s *OpportunityStore) RefreshStatuses(ctx context.Context) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		WITH t AS (SELECT (NOW() AT TIME ZONE 'America/Puerto_Rico')::date AS today)
		UPDATE opportunities o SET
		    status = CASE
		        WHEN o.deadline < t.today THEN 'closed'
		        WHEN o.deadline <= t.today + 7 THEN 'closing_soon'
		        ELSE 'open'
		    END,
		    updated_at = NOW()
		FROM t
		WHERE o.deadline IS NOT NULL
		  AND NOT ('status' = ANY(o.edited_fields))
		  AND o.status IS DISTINCT FROM CASE
		        WHEN o.deadline < t.today THEN 'closed'
		        WHEN o.deadline <= t.today + 7 THEN 'closing_soon'
		        ELSE 'open' END
	`)
	if err != nil {
		return 0, fmt.Errorf("opportunities refresh statuses: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
