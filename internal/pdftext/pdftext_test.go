package pdftext

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestIsPDF(t *testing.T) {
	pdf := []byte("%PDF-1.4\n...")
	cases := []struct {
		ct   string
		data []byte
		want bool
	}{
		{"application/pdf", nil, true},
		{"Application/PDF; charset=binary", []byte("x"), true},
		{"application/octet-stream", pdf, true},
		{"", pdf, true},
		{"text/html", []byte("<html>"), false},
		{"", nil, false},
	}
	for _, c := range cases {
		if got := IsPDF(c.ct, c.data); got != c.want {
			t.Errorf("IsPDF(%q, %.10q) = %v, quería %v", c.ct, c.data, got, c.want)
		}
	}
}

func TestExtractAvisoReal(t *testing.T) {
	res, err := Extract(read(t, "dtop_aviso_publico_reglamento.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Pages != 1 || res.Scanned {
		t.Fatalf("páginas=%d scanned=%v", res.Pages, res.Scanned)
	}
	for _, want := range []string{"AVISO PÚBLICO", "Transportación y Obras Públicas", "Sánchez Vilella", "14 de agosto de 2026"} {
		if !strings.Contains(res.Text, want) {
			t.Errorf("falta %q en:\n%s", want, res.Text)
		}
	}
}

func TestExtractFormularioReal(t *testing.T) {
	res, err := Extract(read(t, "vivienda_solicitud_empleo.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Pages != 3 || res.Scanned {
		t.Fatalf("páginas=%d scanned=%v", res.Pages, res.Scanned)
	}
	if !strings.Contains(res.Text, "INSTRUCCIONES PARA LLENAR LA SOLICITUD DE EXAMEN Y EMPLEO") {
		t.Errorf("título no legible:\n%.400s", res.Text)
	}
	if !strings.Contains(res.Text, "número de") || !strings.Contains(res.Text, "convocatoria") {
		t.Errorf("acentos/palabras perdidas:\n%.800s", res.Text)
	}
}

// minimalPDF arma un PDF válido de una página con el contenido dado.
func minimalPDF(content string) []byte {
	var b bytes.Buffer
	var offs []int
	obj := func(s string) {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(offs), s)
	}
	b.WriteString("%PDF-1.4\n")
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	obj("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>")
	obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, x)
	return b.Bytes()
}

func TestSinTextoEsEscaneado(t *testing.T) {
	// página que solo dibuja un rectángulo (equivale a una imagen sin texto)
	res, err := Extract(minimalPDF("0 0 100 100 re f"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Scanned || res.Pages != 1 || res.Text != "" {
		t.Fatalf("esperaba escaneado, obtuve %+v", res)
	}
}

func TestTextoSintetico(t *testing.T) {
	res, err := Extract(minimalPDF("BT /F1 12 Tf 10 100 Td (Subasta formal numero 123 para la adquisicion de equipo y materiales) Tj ET"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Scanned || !strings.Contains(res.Text, "Subasta formal") {
		t.Fatalf("%+v", res)
	}
}

func TestNoEsPDF(t *testing.T) {
	for _, d := range [][]byte{nil, []byte("<html><body>hola</body></html>"), []byte("PK\x03\x04zip")} {
		if _, err := Extract(d); err == nil {
			t.Errorf("esperaba error para %.20q", d)
		}
	}
}

func TestCorruptoNoHacePanic(t *testing.T) {
	real := read(t, "dtop_aviso_publico_reglamento.pdf")
	casos := map[string][]byte{
		"solo cabecera":  []byte("%PDF-1.7\n"),
		"basura":         append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte{0xff, 0x00, 0x41}, 5000)...),
		"truncado mitad": real[:len(real)/2],
		"truncado 100":   real[:100],
		"cola cortada":   real[:len(real)-300],
	}
	// bytes alterados a lo largo del archivo
	mut := append([]byte(nil), real...)
	for i := 200; i < len(mut); i += 997 {
		mut[i] ^= 0x5a
	}
	casos["mutado"] = mut
	for name, d := range casos {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: pánico %v", name, r)
				}
			}()
			res, err := Extract(d)
			t.Logf("%s: err=%v pages=%d chars=%d", name, err, res.Pages, len(res.Text))
		}()
	}
}

func TestDemasiadoGrande(t *testing.T) {
	big := append([]byte("%PDF-1.4\n"), make([]byte, 41<<20)...)
	_, err := Extract(big)
	if err == nil || !strings.Contains(err.Error(), "demasiado grande") {
		t.Fatalf("err = %v", err)
	}
}

func TestNormalize(t *testing.T) {
	got := normalize([]string{"la contra-", "tación  de   servicios", "Puerto Rico-", "Norte", "fiﬁ cación", "mañana"})
	want := "la contratación de servicios\nPuerto Rico-\nNorte\nffi cación\nmañana"
	// "fiﬁ" -> "fifi"
	want = strings.Replace(want, "ffi", "fifi", 1)
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}
