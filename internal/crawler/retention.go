package crawler

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
)

// DefaultDocumentsMaxBytes: tope total de la carpeta de documentos (1 GB).
const DefaultDocumentsMaxBytes int64 = 1 << 30

// SettingsReader: lo que la limpieza necesita de la tabla settings.
type SettingsReader interface {
	Get(ctx context.Context, key string) (string, bool, error)
}

// DocumentsMaxBytes lee settings.documents_max_bytes (1 GB si falta o es inválido).
func DocumentsMaxBytes(ctx context.Context, s SettingsReader) int64 {
	if s == nil {
		return DefaultDocumentsMaxBytes
	}
	v, ok, err := s.Get(ctx, "documents_max_bytes")
	if err != nil || !ok {
		return DefaultDocumentsMaxBytes
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || n <= 0 {
		return DefaultDocumentsMaxBytes
	}
	return n
}

// newDocGrace: un PDF recién bajado no se borra antes de que la extracción lo vea.
const newDocGrace = time.Hour

// PlanRetention decide qué archivos borrar (nunca filas ni texto):
//
//  1. Los que ningún fondo vivo usa: no produjeron fondo, el fondo fue
//     descartado o quedó cerrado hace más de 90 días. Se salvan los que aún
//     esperan extracción o se bajaron hace menos de una hora.
//     Un documento guardado (Kept, o el de un fondo guardado) jamás se borra
//     ni cuenta para el tope.
//  2. Si aun así el total pasa del tope, lo más viejo sin fondo vivo, aunque
//     sea reciente. El PDF de un fondo por revisar o confirmado jamás se toca.
//
// Devuelve los ids en el orden en que se borran.
func PlanRetention(docs []models.DocState, now time.Time, capBytes int64) []uuid.UUID {
	var del []uuid.UUID
	var total int64
	var rest []models.DocState
	for _, d := range docs {
		if d.Path == "" || d.Kept || d.Saved {
			continue
		}
		if !d.HasActive && !d.Pending && now.Sub(d.CreatedAt) >= newDocGrace {
			del = append(del, d.ID)
			continue
		}
		total += d.SizeBytes
		rest = append(rest, d)
	}
	if capBytes > 0 && total > capBytes {
		sort.SliceStable(rest, func(i, j int) bool { return rest[i].CreatedAt.Before(rest[j].CreatedAt) })
		for _, d := range rest {
			if total <= capBytes {
				break
			}
			if d.HasActive {
				continue
			}
			del = append(del, d.ID)
			total -= d.SizeBytes
		}
	}
	return del
}

// CleanupStats resume una limpieza.
type CleanupStats struct {
	Deleted, Orphans int
	FreedBytes       int64
}

// RunDocumentCleanup aplica la política de retención a <datos>/documentos y
// borra los archivos huérfanos (sin fila). Idempotente.
func RunDocumentCleanup(ctx context.Context, deps Deps, settings SettingsReader) CleanupStats {
	var st CleanupStats
	if deps.Documents == nil || deps.DataDir == "" {
		return st
	}
	now := time.Now()
	docs, err := deps.Documents.ListDocStates(ctx, now)
	if err != nil {
		slog.Error("documents cleanup: list", "err", err)
		return st
	}
	bySize := make(map[uuid.UUID]models.DocState, len(docs))
	for _, d := range docs {
		bySize[d.ID] = d
	}
	for _, id := range PlanRetention(docs, now, DocumentsMaxBytes(ctx, settings)) {
		if ctx.Err() != nil {
			return st
		}
		d := bySize[id]
		abs := filepath.Join(deps.DataDir, d.Path)
		if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
			slog.Warn("documents cleanup: remove", "path", d.Path, "err", err)
			continue
		}
		if err := deps.Documents.ReleaseFile(ctx, id); err != nil {
			slog.Warn("documents cleanup: release", "id", id, "err", err)
			continue
		}
		st.Deleted++
		st.FreedBytes += d.SizeBytes
	}

	// Archivos sueltos (sin fila, p. ej. de un fallo a medias).
	if refs, err := deps.Documents.ReferencedPaths(ctx); err == nil {
		entries, _ := os.ReadDir(DocumentsDir(deps.DataDir))
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			rel := filepath.Join("documentos", e.Name())
			if refs[rel] {
				continue
			}
			info, err := e.Info()
			if err != nil || now.Sub(info.ModTime()) < 24*time.Hour {
				continue
			}
			if os.Remove(filepath.Join(deps.DataDir, rel)) == nil {
				st.Orphans++
				st.FreedBytes += info.Size()
			}
		}
	}
	slog.Info("documents cleanup: done", "deleted", st.Deleted, "orphans", st.Orphans, "freed_mb", st.FreedBytes>>20)
	return st
}
