package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AnalyticsHandler provides HTTP handlers for analytics endpoints that query
// article, entity, and tag data for dashboards and trend analysis.
type AnalyticsHandler struct {
	Pool *pgxpool.Pool
}

// getDaysParam parses the "days" query parameter from the request. Returns 30
// as the default and caps the maximum at 365.
func getDaysParam(r *http.Request) int {
	s := r.URL.Query().Get("days")
	if s == "" {
		return 30
	}
	d, err := strconv.Atoi(s)
	if err != nil || d <= 0 {
		return 30
	}
	if d > 365 {
		return 365
	}
	return d
}

// TagTrends handles GET /api/analytics/tags?days=30.
// Returns daily tag counts extracted from article JSONB tags.
func (h *AnalyticsHandler) TagTrends(w http.ResponseWriter, r *http.Request) {
	days := getDaysParam(r)
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx, `
		SELECT t.tag, date_trunc('day', a.created_at)::date as day, count(*) as cnt
		FROM articles a, jsonb_array_elements_text(a.tags) AS t(tag)
		WHERE a.created_at >= NOW() - make_interval(days => $1)
		GROUP BY t.tag, day
		ORDER BY day DESC, cnt DESC
	`, days)
	if err != nil {
		slog.Error("analytics: tag trends", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query tag trends"})
		return
	}
	defer rows.Close()

	type tagTrend struct {
		Tag   string `json:"tag"`
		Day   string `json:"day"`
		Count int    `json:"count"`
	}

	var trends []tagTrend
	for rows.Next() {
		var t tagTrend
		var day time.Time
		if err := rows.Scan(&t.Tag, &day, &t.Count); err != nil {
			slog.Error("analytics: tag trends scan", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to scan tag trends"})
			return
		}
		t.Day = day.Format("2006-01-02")
		trends = append(trends, t)
	}
	if err := rows.Err(); err != nil {
		slog.Error("analytics: tag trends rows", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to iterate tag trends"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"trends": trends})
}

// TopEntities handles GET /api/analytics/entities?days=30&type=person.
// Returns the most frequently mentioned entities, optionally filtered by type.
func (h *AnalyticsHandler) TopEntities(w http.ResponseWriter, r *http.Request) {
	days := getDaysParam(r)
	entityType := r.URL.Query().Get("type")
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx, `
		SELECT e.name, e.type, count(*) as cnt
		FROM entities e
		JOIN article_entities ae ON e.id = ae.entity_id
		JOIN articles a ON a.id = ae.article_id
		WHERE a.created_at >= NOW() - make_interval(days => $1)
		AND ($2 = '' OR e.type = $2)
		GROUP BY e.name, e.type
		ORDER BY cnt DESC
		LIMIT 50
	`, days, entityType)
	if err != nil {
		slog.Error("analytics: top entities", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query top entities"})
		return
	}
	defer rows.Close()

	type entityRow struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Count int    `json:"count"`
	}

	var entities []entityRow
	for rows.Next() {
		var e entityRow
		if err := rows.Scan(&e.Name, &e.Type, &e.Count); err != nil {
			slog.Error("analytics: top entities scan", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to scan top entities"})
			return
		}
		entities = append(entities, e)
	}
	if err := rows.Err(); err != nil {
		slog.Error("analytics: top entities rows", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to iterate top entities"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"entities": entities})
}

// CoOccurrences handles GET /api/analytics/co-occurrences?entity=X.
// Returns entities that frequently appear alongside the named entity.
func (h *AnalyticsHandler) CoOccurrences(w http.ResponseWriter, r *http.Request) {
	entityName := r.URL.Query().Get("entity")
	if entityName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "entity parameter is required"})
		return
	}
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx, `
		SELECT e2.name, e2.type, count(*) as cnt
		FROM article_entities ae1
		JOIN article_entities ae2 ON ae1.article_id = ae2.article_id AND ae1.entity_id != ae2.entity_id
		JOIN entities e1 ON ae1.entity_id = e1.id
		JOIN entities e2 ON ae2.entity_id = e2.id
		WHERE lower(e1.name) = lower($1)
		GROUP BY e2.name, e2.type
		ORDER BY cnt DESC
		LIMIT 30
	`, entityName)
	if err != nil {
		slog.Error("analytics: co-occurrences", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query co-occurrences"})
		return
	}
	defer rows.Close()

	type coEntity struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Count int    `json:"count"`
	}

	var coOccurrences []coEntity
	for rows.Next() {
		var c coEntity
		if err := rows.Scan(&c.Name, &c.Type, &c.Count); err != nil {
			slog.Error("analytics: co-occurrences scan", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to scan co-occurrences"})
			return
		}
		coOccurrences = append(coOccurrences, c)
	}
	if err := rows.Err(); err != nil {
		slog.Error("analytics: co-occurrences rows", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to iterate co-occurrences"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"co_occurrences": coOccurrences})
}

// SentimentDistribution handles GET /api/analytics/sentiment?days=30.
// Returns the count of articles per sentiment label.
func (h *AnalyticsHandler) SentimentDistribution(w http.ResponseWriter, r *http.Request) {
	days := getDaysParam(r)
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx, `
		SELECT sentiment, count(*) as cnt
		FROM articles
		WHERE sentiment != '' AND created_at >= NOW() - make_interval(days => $1)
		GROUP BY sentiment
	`, days)
	if err != nil {
		slog.Error("analytics: sentiment distribution", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query sentiment distribution"})
		return
	}
	defer rows.Close()

	type sentimentRow struct {
		Sentiment string `json:"sentiment"`
		Count     int    `json:"count"`
	}

	var distribution []sentimentRow
	for rows.Next() {
		var s sentimentRow
		if err := rows.Scan(&s.Sentiment, &s.Count); err != nil {
			slog.Error("analytics: sentiment scan", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to scan sentiment"})
			return
		}
		distribution = append(distribution, s)
	}
	if err := rows.Err(); err != nil {
		slog.Error("analytics: sentiment rows", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to iterate sentiment"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"distribution": distribution})
}

// SourceHealth handles GET /api/analytics/sources.
// Returns per-source article counts, last ingestion time, and enrichment stats.
func (h *AnalyticsHandler) SourceHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx, `
		SELECT source, count(*) as article_count,
		       max(created_at) as last_ingested,
		       count(*) FILTER (WHERE summary != '' AND summary IS NOT NULL) as enriched_count
		FROM articles
		GROUP BY source
		ORDER BY article_count DESC
	`)
	if err != nil {
		slog.Error("analytics: source health", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query source health"})
		return
	}
	defer rows.Close()

	type sourceRow struct {
		Source        string `json:"source"`
		ArticleCount int    `json:"article_count"`
		LastIngested string `json:"last_ingested"`
		EnrichedCount int   `json:"enriched_count"`
	}

	var sources []sourceRow
	for rows.Next() {
		var s sourceRow
		var lastIngested time.Time
		if err := rows.Scan(&s.Source, &s.ArticleCount, &lastIngested, &s.EnrichedCount); err != nil {
			slog.Error("analytics: source health scan", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to scan source health"})
			return
		}
		s.LastIngested = lastIngested.Format(time.RFC3339)
		sources = append(sources, s)
	}
	if err := rows.Err(); err != nil {
		slog.Error("analytics: source health rows", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to iterate source health"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

// ArticleVolume handles GET /api/analytics/volume?days=30.
// Returns daily article counts for charting ingestion volume over time.
func (h *AnalyticsHandler) ArticleVolume(w http.ResponseWriter, r *http.Request) {
	days := getDaysParam(r)
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx, `
		SELECT date_trunc('day', created_at)::date as day, count(*) as cnt
		FROM articles
		WHERE created_at >= NOW() - make_interval(days => $1)
		GROUP BY day
		ORDER BY day ASC
	`, days)
	if err != nil {
		slog.Error("analytics: article volume", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query article volume"})
		return
	}
	defer rows.Close()

	type volumeRow struct {
		Day   string `json:"day"`
		Count int    `json:"count"`
	}

	var volume []volumeRow
	for rows.Next() {
		var v volumeRow
		var day time.Time
		if err := rows.Scan(&day, &v.Count); err != nil {
			slog.Error("analytics: article volume scan", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to scan article volume"})
			return
		}
		v.Day = day.Format("2006-01-02")
		volume = append(volume, v)
	}
	if err := rows.Err(); err != nil {
		slog.Error("analytics: article volume rows", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to iterate article volume"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"volume": volume})
}
