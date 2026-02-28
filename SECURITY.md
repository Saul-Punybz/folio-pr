# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Folio, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email the maintainer directly or use GitHub's private vulnerability reporting feature.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest (main branch) | Yes |

## Security Considerations

- **Authentication**: Session-based with bcrypt password hashing
- **Database**: Parameterized queries via pgx (no SQL injection)
- **CORS**: Configured per deployment environment
- **Secrets**: All credentials stored in environment variables, never committed
- **HTTPS**: Caddy provides automatic TLS certificates in production

## Best Practices for Deployment

1. Change the default admin password immediately after setup
2. Use strong `DB_PASS` values
3. Keep `.env` files out of version control (already in `.gitignore`)
4. Run behind Caddy or another reverse proxy with HTTPS
5. Restrict database access to localhost or private network

---

**Maintainer:** [Saul A. Gonzalez Alonso](https://github.com/Saul-Punybz) -- saul@puny.bz
