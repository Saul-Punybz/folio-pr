// Package pdftext extrae texto de PDF sin IA, sin cgo y sin binarios externos.
// Usa github.com/ledongthuc/pdf (Go puro, licencia BSD-3).
package pdftext

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/ledongthuc/pdf"
)

const (
	maxBytes          = 40 << 20
	extractTimeout    = 90 * time.Second
	minAlphaPerPage   = 40
	maxPagesToExtract = 2000
)

// Result es el texto extraído de un PDF.
type Result struct {
	Text    string // texto en orden de lectura, páginas separadas por "\n\n"
	Pages   int
	Scanned bool // true si el PDF no trae texto extraíble (es imagen escaneada)
}

// IsPDF dice si el contenido es un PDF, por Content-Type o por la firma "%PDF-".
func IsPDF(contentType string, data []byte) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if ct == "application/pdf" || ct == "application/x-pdf" {
		return true
	}
	head := data
	if len(head) > 1024 {
		head = head[:1024]
	}
	return bytes.Contains(head, []byte("%PDF-"))
}

// Extract saca el texto de un PDF. Nunca entra en pánico: un PDF corrupto
// devuelve error.
func Extract(data []byte) (Result, error) {
	if len(data) > maxBytes {
		return Result{}, fmt.Errorf("pdftext: PDF demasiado grande (%d MB, máximo %d MB)", len(data)>>20, maxBytes>>20)
	}
	if len(data) == 0 {
		return Result{}, errors.New("pdftext: archivo vacío")
	}
	if !bytes.Contains(data[:min(len(data), 1024)], []byte("%PDF-")) {
		return Result{}, errors.New("pdftext: el archivo no es un PDF (falta la firma %PDF-)")
	}

	type out struct {
		res Result
		err error
	}
	ch := make(chan out, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- out{err: fmt.Errorf("pdftext: PDF corrupto o no soportado: %v", r)}
			}
		}()
		res, err := extract(data)
		ch <- out{res, err}
	}()
	select {
	case o := <-ch:
		return o.res, o.err
	case <-time.After(extractTimeout):
		return Result{}, errors.New("pdftext: tiempo agotado extrayendo el PDF")
	}
}

func extract(data []byte) (Result, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Result{}, fmt.Errorf("pdftext: no se pudo abrir el PDF: %w", err)
	}
	n := r.NumPage()
	if n <= 0 {
		return Result{}, errors.New("pdftext: el PDF no tiene páginas")
	}
	var pages []string
	alpha := 0
	failed := 0
	for i := 1; i <= n && i <= maxPagesToExtract; i++ {
		txt, ok := pageText(r, i)
		if !ok {
			failed++
		}
		if txt != "" {
			pages = append(pages, txt)
			for _, c := range txt {
				if unicode.IsLetter(c) {
					alpha++
				}
			}
		}
	}
	if failed == n {
		return Result{}, errors.New("pdftext: no se pudo leer ninguna página (PDF corrupto o cifrado)")
	}
	text := strings.TrimSpace(strings.Join(pages, "\n\n"))
	return Result{Text: text, Pages: n, Scanned: alpha < minAlphaPerPage*n}, nil
}

// pageText extrae una página; un pánico en una página no tumba las demás.
func pageText(r *pdf.Reader, i int) (txt string, ok bool) {
	defer func() {
		if rec := recover(); rec != nil {
			txt, ok = "", false
		}
	}()
	p := r.Page(i)
	if p.V.IsNull() {
		return "", true
	}
	return layout(p.Content().Text), true
}

// layout respeta el orden del content stream (que casi siempre es el orden de
// lectura) y parte líneas cuando cambia la coordenada Y. No reordena por X:
// muchas fuentes reportan ancho 0 y eso revuelve las letras.
func layout(ts []pdf.Text) string {
	var raw []string
	var sb strings.Builder
	var curY, prevEnd, prevW float64
	var lastByte byte
	started := false
	flush := func() {
		raw = append(raw, sb.String())
		sb.Reset()
		started = false
	}
	for _, t := range ts {
		if t.S == "\n" || t.S == "\r" {
			// el "\n" que emite la librería aparece también a mitad de línea;
			// los saltos reales se detectan por la coordenada Y.
			continue
		}
		s := strings.ReplaceAll(t.S, "\ufffd", "")
		if s == "" {
			continue
		}
		tol := math.Max(t.FontSize*0.6, 2)
		if started && math.Abs(t.Y-curY) > tol {
			flush()
		}
		if started && prevW > 0 && t.W > 0 {
			gap := t.X - prevEnd
			if gap > t.FontSize*0.3 && lastByte != ' ' && !strings.HasPrefix(s, " ") {
				sb.WriteByte(' ')
			}
		}
		if !started {
			curY = t.Y
			started = true
		}
		sb.WriteString(s)
		lastByte = s[len(s)-1]
		prevEnd, prevW = t.X+t.W, t.W
	}
	if started {
		flush()
	}
	return normalize(raw)
}

var ligatures = strings.NewReplacer(
	"ﬀ", "ff", "ﬁ", "fi", "ﬂ", "fl", "ﬃ", "ffi", "ﬄ", "ffl",
	" ", " ", " ", " ", "​", "", "­", "­",
)

var composeMap = map[string]rune{
	"á": 'á', "é": 'é', "í": 'í', "ó": 'ó', "ú": 'ú',
	"Á": 'Á', "É": 'É', "Í": 'Í', "Ó": 'Ó', "Ú": 'Ú',
	"ñ": 'ñ', "Ñ": 'Ñ', "ü": 'ü', "Ü": 'Ü',
	"ı́": 'í',
}

func compose(s string) string {
	if !strings.ContainsAny(s, "́̃̈") {
		return s
	}
	rs := []rune(s)
	var out []rune
	for i := 0; i < len(rs); i++ {
		if i+1 < len(rs) && (rs[i+1] == 0x301 || rs[i+1] == 0x303 || rs[i+1] == 0x308) {
			if c, ok := composeMap[string(rs[i:i+2])]; ok {
				out = append(out, c)
				i++
				continue
			}
		}
		out = append(out, rs[i])
	}
	return string(out)
}

// normalize une palabras partidas por guion al final de línea y limpia espacios.
func normalize(raw []string) string {
	var lines []string
	for _, l := range raw {
		l = compose(ligatures.Replace(l))
		l = strings.Join(strings.Fields(l), " ")
		l = strings.ReplaceAll(l, "­", "")
		if l != "" {
			lines = append(lines, l)
		}
	}
	var sb strings.Builder
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if i+1 < len(lines) && joinsHyphen(l, lines[i+1]) {
			// "contra-" + "tación" -> "contratación"
			l = l[:len(l)-1] + lines[i+1]
			lines[i+1] = l
			continue
		}
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

func joinsHyphen(cur, next string) bool {
	if !strings.HasSuffix(cur, "-") || len(cur) < 2 {
		return false
	}
	rs := []rune(strings.TrimSuffix(cur, "-"))
	last := rs[len(rs)-1]
	if !unicode.IsLetter(last) {
		return false
	}
	first := []rune(next)[0]
	return unicode.IsLower(first)
}
