package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/Saul-Punybz/folio/internal/config"
	"github.com/Saul-Punybz/folio/internal/embeddb"
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

		cfg, cleanup, err := userDB()
		if err != nil {
			return err
		}
		defer cleanup()
		pool, err := connect(ctx, cfg)
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
		cfg, cleanup, err := userDB()
		if err != nil {
			return err
		}
		defer cleanup()
		pool, err := connect(ctx, cfg)
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

// userDB elige la base para `folio user`:
//   - con alguna variable DB_* → esa base (servidor);
//   - sin DB_* → el Postgres embebido de la app: si Folio está abierto, su
//     conexión (puerto de pgdata/postmaster.pid y contraseña de db.secret);
//     si no, lo arranca un momento y lo para al terminar.
func userDB() (config.DBConfig, func(), error) {
	noop := func() {}
	for _, k := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME", "DB_SSLMODE"} {
		if os.Getenv(k) != "" {
			return config.Load().DB, noop, nil
		}
	}
	root, err := appRoot()
	if err != nil {
		return config.DBConfig{}, noop, err
	}
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0o700); err != nil {
		return config.DBConfig{}, noop, err
	}
	lock, ok, err := lockInstance(root)
	if err != nil {
		return config.DBConfig{}, noop, fmt.Errorf("candado de instancia: %w", err)
	}
	if !ok {
		// Folio está abierto: usar su Postgres.
		cfg, err := runningDB(root)
		return cfg, noop, err
	}
	pg, err := embeddb.Start(embeddb.Options{Root: root, BinariesPath: pgBinaries()})
	if err != nil {
		lock.Close()
		return config.DBConfig{}, noop, fmt.Errorf("no arrancó la base de datos: %w", err)
	}
	return pg.DB(), func() {
		if err := pg.Stop(); err != nil {
			fmt.Fprintln(os.Stderr, "aviso: parar postgres:", err)
		}
		lock.Close()
	}, nil
}

// runningDB: conexión al Postgres del Folio que está abierto.
func runningDB(root string) (config.DBConfig, error) {
	pid, err := os.ReadFile(filepath.Join(root, "pgdata", "postmaster.pid"))
	if err != nil {
		return config.DBConfig{}, fmt.Errorf("Folio está abierto pero su base no responde todavía: %w", err)
	}
	lines := strings.Split(string(pid), "\n")
	if len(lines) < 4 {
		return config.DBConfig{}, errors.New("postmaster.pid incompleto")
	}
	port, err := strconv.Atoi(strings.TrimSpace(lines[3]))
	if err != nil {
		return config.DBConfig{}, fmt.Errorf("puerto en postmaster.pid: %w", err)
	}
	secret, err := os.ReadFile(filepath.Join(root, "db.secret"))
	if err != nil {
		return config.DBConfig{}, fmt.Errorf("db.secret: %w", err)
	}
	return config.DBConfig{Host: "127.0.0.1", Port: port, User: "folio", Pass: strings.TrimSpace(string(secret)),
		DBName: "folio", SSLMode: "disable"}, nil
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
