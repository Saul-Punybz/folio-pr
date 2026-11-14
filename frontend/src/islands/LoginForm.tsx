import { useState, useEffect, type FormEvent } from 'react';
import { api } from '../lib/api';
import { Spinner } from './ui';

export default function LoginForm() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null);
  const [password2, setPassword2] = useState('');
  const [touched, setTouched] = useState(false);

  // Primer uso: si el backend no tiene usuarios, se crea el acceso. Si falla (backend viejo), login normal.
  useEffect(() => {
    let alive = true;
    api.setupStatus()
      .then((r) => { if (alive) setNeedsSetup(!!r.needs_setup); })
      .catch(() => { if (alive) setNeedsSetup(false); });
    return () => { alive = false; };
  }, []);

  const setup = needsSetup === true;
  const tooShort = password.length < 10;
  const mismatch = password !== password2;

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError('');
    if (setup) {
      setTouched(true);
      if (tooShort || mismatch) return;
    }
    setLoading(true);

    try {
      if (setup) await api.setup(email, password);
      await api.login(email, password);
      window.location.href = '/inbox';
    } catch (err: any) {
      setError(err.message || (setup ? 'No se pudo crear el acceso' : 'Credenciales inválidas'));
      setLoading(false);
    }
  }

  if (needsSetup === null) {
    return <div className="flex justify-center py-8 text-primary"><Spinner className="w-8 h-8" /></div>;
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      {setup && (
        <div>
          <h2 className="title-lg">Crea tu acceso</h2>
          <p className="mt-1 text-sm text-on-surface-variant">Es la primera vez. Elige tu correo y una contraseña.</p>
        </div>
      )}
      {error && (
        <div role="alert" className="p-4 text-sm font-semibold rounded-2xl bg-error-container text-on-error-container">
          {error}
        </div>
      )}

      <div>
        <label htmlFor="email" className="field-label">
          Correo
        </label>
        <input
          id="email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          autoComplete="email"
          autoFocus
          placeholder="tu@correo.org"
          className="field"
        />
      </div>

      <div>
        <label htmlFor="password" className="field-label">
          Contraseña
        </label>
        <input
          id="password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          autoComplete={setup ? 'new-password' : 'current-password'}
          placeholder={setup ? 'Mínimo 10 caracteres' : 'Tu contraseña'}
          className="field"
        />
        {setup && (
          <p className={`mt-2 text-sm font-semibold ${touched && tooShort ? 'text-error' : 'text-on-surface-variant'}`}>
            Mínimo 10 caracteres{password ? ` (llevas ${password.length})` : ''}.
          </p>
        )}
      </div>

      {setup && (
        <div>
          <label htmlFor="password2" className="field-label">
            Repite la contraseña
          </label>
          <input
            id="password2"
            type="password"
            value={password2}
            onChange={(e) => setPassword2(e.target.value)}
            required
            autoComplete="new-password"
            placeholder="Otra vez"
            className="field"
          />
          {password2 && mismatch && (
            <p className="mt-2 text-sm font-semibold text-error">Las contraseñas no coinciden.</p>
          )}
        </div>
      )}

      <button type="submit" disabled={loading} className="btn btn-filled w-full !min-h-[56px] text-base">
        {loading ? (
          <span className="flex items-center justify-center gap-2">
            <svg className="w-5 h-5 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
            {setup ? 'Creando...' : 'Entrando...'}
          </span>
        ) : (
          setup ? 'Crear acceso' : 'Entrar'
        )}
      </button>
    </form>
  );
}
