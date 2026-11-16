package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Saul-Punybz/folio/internal/models"
)

// `folio user reset`: cambia la contraseña, conserva el rol, cierra las
// sesiones de ese usuario (no las de otros) y avisa si el correo no existe.
func TestUserReset(t *testing.T) {
	pool := testPG(t)
	ctx := context.Background()
	st := newStores(pool, t.TempDir())
	a := &models.User{Email: "ana@folio.local", PasswordHash: "viejo", Role: "admin"}
	b := &models.User{Email: "beto@folio.local", PasswordHash: "viejo", Role: "member"}
	for _, u := range []*models.User{a, b} {
		if err := st.users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []*models.Session{
		{ID: "sa1", UserID: a.ID, ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "sa2", UserID: a.ID, ExpiresAt: time.Now().Add(time.Hour)},
		{ID: "sb1", UserID: b.ID, ExpiresAt: time.Now().Add(time.Hour)},
	} {
		if err := st.sessions.Create(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := resetUserPassword(ctx, st.users, "ana@folio.local", "corta"); err == nil {
		t.Fatal("debía rechazar contraseña corta")
	}
	if _, err := st.sessions.GetByToken(ctx, "sa1"); err != nil {
		t.Fatal("una contraseña rechazada no debe cerrar sesiones")
	}
	if err := resetUserPassword(ctx, st.users, "ANA@folio.local", "nueva-contraseña-1"); err != nil {
		t.Fatal(err)
	}
	got, err := st.users.GetByEmail(ctx, "ana@folio.local")
	if err != nil || got.Role != "admin" || bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("nueva-contraseña-1")) != nil {
		t.Fatalf("contraseña/rol mal: %v %+v", err, got)
	}
	for _, tok := range []string{"sa1", "sa2"} {
		if _, err := st.sessions.GetByToken(ctx, tok); err == nil {
			t.Errorf("la sesión %s debía cerrarse", tok)
		}
	}
	if _, err := st.sessions.GetByToken(ctx, "sb1"); err != nil {
		t.Error("la sesión de otro usuario no debe cerrarse")
	}
	err = resetUserPassword(ctx, st.users, "nadie@folio.local", "nueva-contraseña-1")
	if err == nil || !strings.Contains(err.Error(), "no existe") {
		t.Fatalf("usuario inexistente: %v", err)
	}
}
