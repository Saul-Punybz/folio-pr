package watch

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/archivo"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/robots"
)

// Archivo de menciones: para cada mención con página, Folio baja la página
// (con robots.txt, ≥1 s por host, timeout y tope de tamaño), guarda el texto
// y los metadatos, y busca un enlace de respaldo en Internet Archive.

const (
	// ArchiveRunCap: menciones que procesa una corrida (arranque, cron, botón).
	ArchiveRunCap = 150
	archiveBatch  = 25
)

// WaybackAPI: «available» de Internet Archive; responde sin cuenta ni llave.
var WaybackAPI = "https://archive.org/wayback/available"

var robotsCheck = robots.New(userAgent, nil)

var errRobots = errors.New("robots.txt no permite bajar esta página")

// pageFetch es lo que devuelve bajar una página.
type pageFetch struct {
	Body        []byte
	FinalURL    string
	Status      int
	ContentType string
}

// fetchPage baja una página de forma cortés: robots.txt (también en cada
// redirección), una petición por segundo por host, timeout y tope de tamaño.
func fetchPage(ctx context.Context, rawURL string) (*pageFetch, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("URL no válida")
	}
	if !robotsCheck.Allowed(ctx, rawURL) {
		return nil, errRobots
	}
	if err := clock.wait(ctx, rawURL); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, agentTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
	req.Header.Set("Accept-Language", "es-PR,es;q=0.9,en;q=0.7")
	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("demasiadas redirecciones")
			}
			if !robotsCheck.Allowed(r.Context(), r.URL.String()) {
				return errRobots
			}
			return clock.wait(r.Context(), r.URL.String())
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	pf := &pageFetch{Body: body, FinalURL: resp.Request.URL.String(), Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type")}
	if resp.StatusCode != http.StatusOK {
		return pf, fmt.Errorf("la página respondió %d", resp.StatusCode)
	}
	return pf, nil
}

var reURLinBytes = regexp.MustCompile(`https?://[\x21-\x7e]+`)

// decodeGoogleNewsURL saca la URL real de un enlace de Google News cuando el
// identificador la lleva dentro (formato viejo). Los identificadores nuevos
// (AU_yqL…) no la traen: devuelve "".
func decodeGoogleNewsURL(link string) string {
	u, err := url.Parse(link)
	if err != nil || u.Hostname() != "news.google.com" {
		return ""
	}
	id := u.Path[strings.LastIndex(u.Path, "/")+1:]
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(id, "="))
	if err != nil {
		return ""
	}
	if m := reURLinBytes.Find(b); m != nil {
		return strings.TrimRight(string(m), "\x00")
	}
	return ""
}

func isGoogleHost(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "google.com" || strings.HasSuffix(h, ".google.com")
}

var bloqueos = []string{"just a moment", "attention required", "access denied", "verify you are human",
	"are you a robot", "acceso denegado", "checking your browser", "enable javascript", "before you continue"}

// pareceBloqueo: la «página» es un muro anti-bots o de consentimiento.
func pareceBloqueo(p archivo.Page) bool {
	t := strings.ToLower(p.Title)
	for _, b := range bloqueos {
		if strings.Contains(t, b) {
			return true
		}
	}
	return false
}

// waybackLookup busca la captura de Internet Archive más cercana a la fecha de
// publicación (o a hoy). Sin cuenta ni llave. "" si no hay captura.
func waybackLookup(ctx context.Context, pageURL string, near *time.Time) (string, error) {
	q := "?url=" + url.QueryEscape(pageURL)
	if near != nil {
		q += "&timestamp=" + near.UTC().Format("20060102")
	}
	body, _, err := fetch(ctx, WaybackAPI+q, "application/json")
	if err != nil {
		return "", err
	}
	var r struct {
		Archived struct {
			Closest struct {
				Available bool   `json:"available"`
				URL       string `json:"url"`
				Status    string `json:"status"`
			} `json:"closest"`
		} `json:"archived_snapshots"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	c := r.Archived.Closest
	if !c.Available || c.URL == "" || !strings.HasPrefix(c.Status, "2") {
		return "", nil
	}
	return strings.Replace(c.URL, "http://", "https://", 1), nil
}

// titleAuthorPrefix separa «Autor: texto» (como arman el título los posts).
func titleAuthorPrefix(title string) (author, text string) {
	if i := strings.Index(title, ": "); i > 0 && i <= 80 {
		return strings.TrimSpace(title[:i]), strings.TrimSpace(title[i+2:])
	}
	return "", title
}

func nonEmpty(ss ...string) string {
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// archiveOne archiva una mención. Devuelve lo que se guardará.
func archiveOne(ctx context.Context, it models.ArchiveItem) models.ArchiveResult {
	kind := archivo.KindFor(it.SourceType)
	res := models.ArchiveResult{}

	title, suffix := it.Title, ""
	if it.SourceType == "google_news" {
		title, suffix = archivo.SplitTitleSite(it.Title)
	}

	if it.NeedFetch {
		res.Fetched = true
		switch {
		case kind == archivo.KindPost:
			// El texto del post ya está guardado (viene de la API de la red).
			author, text := titleAuthorPrefix(it.Title)
			text = nonEmpty(it.Snippet, text)
			res.Status = models.ArchiveCopia
			res.ContentText = text
			res.Summary = archivo.CortarPorOracion(text, archivo.ResumenMaxRunes)
			res.SiteName = archivo.PlatformFor(it.SourceType)
			if len(it.Authors) == 0 && author != "" {
				res.Authors = []string{author}
			}
		default:
			archivePage(ctx, it, title, suffix, &res)
		}
	}

	// Enlace de respaldo en Internet Archive (si funciona sin cuenta).
	target := nonEmpty(res.CanonicalURL, res.ResolvedURL, it.URL)
	if it.WaybackURL == "" && !isGoogleHost(target) && ctx.Err() == nil {
		res.WaybackTried = true
		if w, err := waybackLookup(ctx, target, it.PublishedAt); err != nil {
			slog.Debug("archivo: wayback", "url", target, "err", err)
			res.WaybackTried = false // no se pudo preguntar: se reintenta luego
		} else {
			res.WaybackURL = w
		}
	}
	return res
}

// archivePage baja la página de una mención y llena res.
func archivePage(ctx context.Context, it models.ArchiveItem, title, suffix string, res *models.ArchiveResult) {
	fallback := func(why string) {
		res.Status = models.ArchiveSinCopia
		res.Err = truncateStr(why, 200)
		// Se guarda lo que se tenga: título limpio, medio y descripción del buscador.
		res.Title = title
		if suffix != "" {
			res.SiteName = suffix
		} else if it.SiteName == "" {
			res.SiteName = archivo.SiteFromURL(it.URL)
		}
		if !isGoogleHost(it.URL) || suffix != "" {
			res.Summary = archivo.Resumen(it.Snippet, "", title)
		}
	}

	target := it.URL
	if real := decodeGoogleNewsURL(it.URL); real != "" {
		target = real
		res.ResolvedURL = real
	}
	pf, err := fetchPage(ctx, target)
	if err != nil {
		fallback(err.Error())
		return
	}
	if pf.FinalURL != it.URL {
		res.ResolvedURL = pf.FinalURL
	}
	if isGoogleHost(pf.FinalURL) {
		res.ResolvedURL = ""
		fallback("Google News no entregó la página original")
		return
	}
	if ct := strings.ToLower(pf.ContentType); ct != "" && !strings.Contains(ct, "html") && !strings.Contains(ct, "xml") {
		fallback("no es una página web (" + ct + ")")
		return
	}
	p := archivo.ParsePage(pf.Body, pf.FinalURL)
	if pareceBloqueo(p) {
		res.ResolvedURL = ""
		fallback("la página pide verificación anti-bots")
		return
	}
	res.Title = nonEmpty(p.Title, title)
	res.SiteName = nonEmpty(p.Site, suffix)
	res.Authors = p.Authors
	res.PublishedAt = p.Published
	res.Lang = p.Lang
	res.CanonicalURL = p.Canonical
	res.ImageURL = p.Image
	res.Summary = archivo.Resumen(p.Description, p.Text, res.Title)
	if utf8.RuneCountInString(p.Text) >= archivo.MinCopyRunes {
		res.Status = models.ArchiveCopia
		res.ContentText = p.Text
	} else {
		// La página cargó pero el cuerpo no venía en el HTML (se arma con
		// JavaScript): se guarda lo que haya y no se reintenta.
		res.Status = models.ArchiveParcial
		res.ContentText = nonEmpty(p.Text, p.Description, it.Snippet)
	}
}

// RunArchive archiva hasta maxItems menciones pendientes, en tandas. Una
// mención «sin copia» se reintenta en la corrida siguiente (máx. 3 intentos).
// Devuelve cuántas procesó. Nunca borra nada.
func RunArchive(ctx context.Context, hits *models.WatchlistHitStore, maxItems int) int {
	if hits == nil {
		return 0
	}
	if maxItems <= 0 {
		maxItems = ArchiveRunCap
	}
	start := time.Now()
	var seen []uuid.UUID
	var copia, parcial, sin int
	done := 0
	for done < maxItems && ctx.Err() == nil {
		n := archiveBatch
		if maxItems-done < n {
			n = maxItems - done
		}
		items, err := hits.PendingArchive(ctx, n, seen)
		if err != nil {
			slog.Error("archivo: pendientes", "err", err)
			break
		}
		if len(items) == 0 {
			break
		}
		for _, it := range items {
			if ctx.Err() != nil {
				break
			}
			seen = append(seen, it.ID)
			res := archiveOne(ctx, it)
			if err := hits.SaveArchive(ctx, it.ID, res); err != nil {
				slog.Error("archivo: guardar", "id", it.ID, "err", err)
				continue
			}
			done++
			switch res.Status {
			case models.ArchiveCopia:
				copia++
			case models.ArchiveParcial:
				parcial++
			case models.ArchiveSinCopia:
				sin++
			}
		}
	}
	if done > 0 {
		slog.Info("archivo: corrida lista", "procesadas", done, "copia", copia, "parcial", parcial, "sin_copia", sin,
			"duracion", time.Since(start).Round(time.Second))
	}
	return done
}
