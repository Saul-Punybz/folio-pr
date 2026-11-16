package crawler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/pdftext"
)

// ErrTooLarge: el PDF pasa del tope de tamaño.
var ErrTooLarge = errors.New("PDF demasiado grande")

var pdfClient = &http.Client{Timeout: fetchTimeout}

// PDFProbe es lo que dice el servidor de un PDF sin bajarlo (HEAD).
type PDFProbe struct {
	Status        int
	ContentLength int64 // -1 si no lo dice
	ETag          string
	LastModified  string
}

// ProbePDF pide solo los encabezados. Un servidor que no soporta HEAD (405,
// 501…) o falla devuelve error: el que llama baja de todos modos con tope.
func ProbePDF(ctx context.Context, rawURL string) (*PDFProbe, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := pdfClient.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HEAD %d", resp.StatusCode)
	}
	p := &PDFProbe{Status: resp.StatusCode, ContentLength: -1,
		ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")}
	if n, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil {
		p.ContentLength = n
	}
	return p, nil
}

// Unchanged: el servidor dice que es el mismo archivo que ya se bajó
// (mismo ETag, o mismo Last-Modified y mismo tamaño, o —sin validadores—
// el mismo tamaño exacto).
func (p *PDFProbe) Unchanged(etag, lastModified string, size int64) bool {
	switch {
	case p.ETag != "" && etag != "":
		return p.ETag == etag
	case p.LastModified != "" && lastModified != "":
		return p.LastModified == lastModified && (p.ContentLength < 0 || size <= 0 || p.ContentLength == size)
	default:
		return size > 0 && p.ContentLength == size
	}
}

// FetchPDF baja un PDF con tope: si el servidor anuncia más de maxBytes no
// baja nada; si no lo anuncia, corta al pasarse. Devuelve ErrTooLarge.
func FetchPDF(ctx context.Context, rawURL string, maxBytes int64) (*FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/pdf,*/*;q=0.5")
	resp, err := pdfClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	res := &FetchResult{
		URL: rawURL, StatusCode: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"),
		FinalURL: resp.Request.URL.String(), ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified"),
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return res, nil
	}
	if resp.ContentLength > maxBytes {
		return res, fmt.Errorf("%w: %d MB (tope %d MB)", ErrTooLarge, resp.ContentLength>>20, maxBytes>>20)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return res, err
	}
	if int64(len(body)) > maxBytes {
		return res, fmt.Errorf("%w: más de %d MB", ErrTooLarge, maxBytes>>20)
	}
	res.ContentLength = len(body)
	if pdftext.IsPDF(res.ContentType, body) {
		res.IsPDF = true
		res.Body = body
	}
	return res, nil
}

// DownloadDocument baja un PDF (con los mismos topes del crawler), lo guarda
// en <datos>/documentos/<sha256>.pdf y registra su fila. Sirve para guardar a
// pedido un PDF que solo se conocía por su enlace.
func DownloadDocument(ctx context.Context, docs *models.DocumentStore, dataDir, rawURL string) (*models.Document, error) {
	return downloadDocument(ctx, docs, dataDir, rawURL, FetchPDF)
}

// DownloadDocumentWith es DownloadDocument con otra función de descarga (pruebas).
func DownloadDocumentWith(ctx context.Context, docs *models.DocumentStore, dataDir, rawURL string,
	fetch func(context.Context, string, int64) (*FetchResult, error)) (*models.Document, error) {
	return downloadDocument(ctx, docs, dataDir, rawURL, fetch)
}

func downloadDocument(ctx context.Context, docs *models.DocumentStore, dataDir, rawURL string,
	fetch func(context.Context, string, int64) (*FetchResult, error)) (*models.Document, error) {
	if docs == nil || dataDir == "" {
		return nil, errors.New("sin carpeta de documentos")
	}
	res, err := fetch(ctx, rawURL, MaxDocumentBytes)
	if err != nil {
		return nil, err
	}
	if !res.IsPDF || len(res.Body) == 0 {
		return nil, errors.New("el enlace no devolvió un PDF")
	}
	text, err := pdftext.Extract(res.Body)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(res.Body)
	sha := hex.EncodeToString(sum[:])
	rel := filepath.Join("documentos", sha+".pdf")
	abs := filepath.Join(dataDir, rel)
	if _, statErr := os.Stat(abs); statErr != nil {
		if err := os.MkdirAll(DocumentsDir(dataDir), 0o700); err != nil {
			return nil, err
		}
		tmp := abs + ".tmp"
		if err := os.WriteFile(tmp, res.Body, 0o600); err != nil {
			return nil, err
		}
		if err := os.Rename(tmp, abs); err != nil {
			return nil, err
		}
	}
	doc := &models.Document{SHA256: sha, Path: rel, SizeBytes: int64(len(res.Body)), Pages: text.Pages,
		Scanned: text.Scanned, SourceURL: rawURL}
	if err := docs.Upsert(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}
