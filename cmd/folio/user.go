package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/Saul-Punybz/folio/internal/config"
	"github.com/Saul-Punybz/folio/internal/handlers"
	"github.com/Saul-Punybz/folio/internal/models"
)

const minPasswordLen = handlers.MinPasswordLen

// userCommand atiende `folio user add|list`.
func userCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch args[0] {
	case "add":
		if len(args) < 2 || len(args) > 3 {
			return errors.New("uso: folio user add <email> [admin|member]")
		}
		email := models.NormalizeEmail(args[1])
		if !strings.Contains(email, "@") {
			return fmt.Errorf("email inválido: %q", args[1])
		}
		role := "member"
		if len(args) == 3 {
			role = strings.ToLower(args[2])
		}
		if role != "admin" && role != "member" {
			return fmt.Errorf("rol inválido %q (admin o member)", role)
		}

		password, err := readPassword()
		if err != nil {
			return err
		}
		if len([]rune(password)) < minPasswordLen {
			return fmt.Errorf("la contraseña debe tener al menos %d caracteres", minPasswordLen)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash: %w", err)
		}

		pool, err := connect(ctx, config.Load().DB)
		if err != nil {
			return err
		}
		defer pool.Close()
		users := models.NewUserStore(pool)

		if _, err := users.GetByEmail(ctx, email); err == nil {
			if err := users.SetPassword(ctx, email, string(hash), role); err != nil {
				return err
			}
			fmt.Printf("usuario actualizado: %s (%s)\n", email, role)
			return nil
		}
		u := &models.User{Email: email, PasswordHash: string(hash), Role: role}
		if err := users.Create(ctx, u); err != nil {
			return err
		}
		fmt.Printf("usuario creado: %s (%s)\n", email, role)
		return nil

	case "list":
		pool, err := connect(ctx, config.Load().DB)
		if err != nil {
			return err
		}
		defer pool.Close()
		list, err := models.NewUserStore(pool).List(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "EMAIL\tROL\tCREADO")
		for _, u := range list {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", u.Email, u.Role, u.CreatedAt.Format("2006-01-02"))
		}
		return tw.Flush()

	default:
		return errors.New(usage)
	}
}

// readPassword lee FOLIO_PASSWORD; si no está, stdin (sin eco en terminal).
func readPassword() (string, error) {
	if p := os.Getenv("FOLIO_PASSWORD"); p != "" {
		return p, nil
	}
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Contraseña: ")
		p1, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("leer contraseña: %w", err)
		}
		fmt.Fprint(os.Stderr, "Repite la contraseña: ")
		p2, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("leer contraseña: %w", err)
		}
		if string(p1) != string(p2) {
			return "", errors.New("las contraseñas no coinciden")
		}
		return string(p1), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("no hay contraseña: usa FOLIO_PASSWORD o pásala por stdin")
	}
	return strings.TrimRight(line, "\r\n"), nil
}
