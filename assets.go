// Package folio provides embedded assets for the self-contained macOS app.
package folio

import "embed"

// FrontendDist contains the built Astro/React frontend.
//
//go:embed all:frontend/dist
var FrontendDist embed.FS

// Migrations contains all SQL migration files.
//
//go:embed migrations/*.sql
var Migrations embed.FS
