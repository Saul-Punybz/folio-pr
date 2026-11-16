package archivo

import (
	"bytes"
	"encoding/csv"
	"strings"
)

// Fila es una mención lista para exportar.
type Fila struct {
	Titulo    string
	Medio     string
	Autores   string
	Fecha     string // «2025-10-19» o vacío
	URL       string
	Resumen   string
	Cita      Cita
	Copia     string // URL de Internet Archive, si hay
	Archivada string
}

// CSV exporta: título, medio, autor, fecha, URL, resumen, cita APA. Lleva BOM
// UTF-8 para que Excel respete los acentos. Una celda que empieza con = + - @
// se escapa para que no se ejecute como fórmula.
func CSV(filas []Fila) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"Título", "Medio", "Autor", "Fecha", "URL", "Resumen", "Cita APA", "Copia en Internet Archive"}); err != nil {
		return nil, err
	}
	for _, f := range filas {
		rec := []string{f.Titulo, f.Medio, f.Autores, f.Fecha, f.URL, f.Resumen, f.Cita.Plain, f.Copia}
		for i, c := range rec {
			rec[i] = neutralizar(c)
		}
		if err := w.Write(rec); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func neutralizar(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// Markdown exporta las menciones con su cita APA (cursiva con *…*).
func Markdown(titulo string, filas []Fila) string {
	var b strings.Builder
	b.WriteString("# " + titulo + "\n\n")
	for _, f := range filas {
		b.WriteString("## " + oneLine(f.Titulo) + "\n\n")
		meta := []string{}
		if f.Medio != "" {
			meta = append(meta, "**Medio:** "+f.Medio)
		}
		if f.Autores != "" {
			meta = append(meta, "**Autor:** "+f.Autores)
		}
		if f.Fecha != "" {
			meta = append(meta, "**Fecha:** "+f.Fecha)
		}
		if len(meta) > 0 {
			b.WriteString(strings.Join(meta, " · ") + "\n\n")
		}
		if f.Resumen != "" {
			b.WriteString(f.Resumen + "\n\n")
		}
		b.WriteString("- Original: " + f.URL + "\n")
		if f.Copia != "" {
			b.WriteString("- Copia en Internet Archive: " + f.Copia + "\n")
		}
		b.WriteString("- Cita APA: " + f.Cita.Markdown + "\n\n")
	}
	return b.String()
}

// Bibliografia une las citas, una por párrafo, en el orden en que vienen.
func Bibliografia(es []Entrada, markdown bool) string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		if markdown {
			out = append(out, e.Cita.Markdown)
		} else {
			out = append(out, e.Cita.Plain)
		}
	}
	return strings.Join(out, "\n\n")
}

func oneLine(s string) string { return collapse(s) }
