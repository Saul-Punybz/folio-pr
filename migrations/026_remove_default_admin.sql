-- Migration 026: quitar el usuario por defecto admin@folio.local / admin.
-- Los usuarios se crean ahora con `folio user add`.
--
-- Todas las FK hacia users son ON DELETE CASCADE (sessions, notes,
-- watchlist_orgs, chat_sessions, telegram_users, bot_notifications,
-- research_projects, escritos). En una instalación vieja, la app de escritorio
-- usaba AutoAuth como ese admin, así que TODO lo que se creó está a su nombre:
-- borrarlo sin más borraría en cascada notas, vigilancia y escritos.
--
-- Por eso:
--   1. Siempre se invalida su contraseña (ningún hash bcrypt empieza con '!'),
--      y se cierran sus sesiones: ya no se puede entrar con admin/admin.
--   2. Solo se borra la fila si no tiene datos propios colgando.
-- En una base nueva el admin no tiene nada, así que se borra.

UPDATE users
SET password_hash = '!disabled-default-admin'
WHERE email = 'admin@folio.local';

DELETE FROM sessions
WHERE user_id IN (SELECT id FROM users WHERE email = 'admin@folio.local');

DELETE FROM users u
WHERE u.email = 'admin@folio.local'
  AND NOT EXISTS (SELECT 1 FROM notes n WHERE n.user_id = u.id)
  AND NOT EXISTS (SELECT 1 FROM watchlist_orgs w WHERE w.user_id = u.id)
  AND NOT EXISTS (SELECT 1 FROM escritos e WHERE e.user_id = u.id)
  AND NOT EXISTS (SELECT 1 FROM research_projects r WHERE r.user_id = u.id)
  AND NOT EXISTS (SELECT 1 FROM chat_sessions c WHERE c.user_id = u.id);
