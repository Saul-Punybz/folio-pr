import { useEffect, type ReactNode } from 'react';

/* Piezas compartidas de presentación (Material 3 Expressive). Sin lógica de datos. */

const PATHS: Record<string, string> = {
  save: 'M17.593 3.322c1.1.128 1.907 1.077 1.907 2.185V21L12 17.25 4.5 21V5.507c0-1.108.806-2.057 1.907-2.185a48.507 48.507 0 0111.186 0z',
  trash: 'M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0',
  pin: 'M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.563.563 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.563.563 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z',
  open: 'M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25',
  close: 'M6 18L18 6M6 6l12 12',
  plus: 'M12 4.5v15m7.5-7.5h-15',
  search: 'M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z',
  refresh: 'M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182',
  link: 'M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m9.86-3.07a4.5 4.5 0 00-6.364-6.364L4.5 8.25',
  check: 'M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
  alert: 'M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z',
  inbox: 'M2.25 13.5h3.86a2.25 2.25 0 012.012 1.244l.256.512a2.25 2.25 0 002.013 1.244h3.218a2.25 2.25 0 002.013-1.244l.256-.512a2.25 2.25 0 012.013-1.244h3.859m-17.399 0V5.25A2.25 2.25 0 014.5 3h15a2.25 2.25 0 012.25 2.25v8.25m-17.399 0l-.97 4.843A1.5 1.5 0 004.873 21h14.254a1.5 1.5 0 001.49-1.707l-.97-4.843',
  eye: 'M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178zM15 12a3 3 0 11-6 0 3 3 0 016 0z',
  globe: 'M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418',
  note: 'M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125',
  undo: 'M9 15L3 9m0 0l6-6M3 9h12a6 6 0 010 12h-3',
};

export function Icon({ name, className = 'w-6 h-6', fill = false }: { name: keyof typeof PATHS | string; className?: string; fill?: boolean }) {
  return (
    <svg className={className} fill={fill ? 'currentColor' : 'none'} viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75} aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" d={PATHS[name] || ''} />
    </svg>
  );
}

export function Spinner({ className = 'w-5 h-5' }: { className?: string }) {
  return (
    <svg className={`${className} animate-spin`} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
  );
}

/** Estado vacío: ícono grande y una frase que dice qué hacer. */
export function EmptyState({ icon, text, children }: { icon: string; text: string; children?: ReactNode }) {
  return (
    <div className="empty">
      <div className="empty-icon">
        <Icon name={icon} className="w-11 h-11" />
      </div>
      <p>{text}</p>
      {children && <div className="flex flex-wrap items-center justify-center gap-3 mt-1">{children}</div>}
    </div>
  );
}

/** Diálogo: capa tenue + hoja con esquinas grandes que entra con resorte. */
export function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);
  return (
    <div className="fixed inset-0 z-[70] flex items-end sm:items-center justify-center p-3 sm:p-4" role="dialog" aria-modal="true" aria-label={title}>
      <div className="absolute inset-0 scrim" onClick={onClose} />
      <div className="relative w-full max-w-md max-h-[90vh] overflow-y-auto bg-surface-container-low rounded-[32px] p-6 sm:p-7 shadow-2xl animate-fade-in">
        <div className="flex items-center justify-between gap-3 mb-4">
          <h2 className="title-lg">{title}</h2>
          <button type="button" onClick={onClose} className="icon-btn -mr-2" aria-label="Cerrar"><Icon name="close" /></button>
        </div>
        {children}
      </div>
    </div>
  );
}

/** Mensaje de error o éxito dentro de una pantalla. */
export function Notice({ kind = 'bad', children }: { kind?: 'bad' | 'ok' | 'accent'; children: ReactNode }) {
  const cls = kind === 'ok' ? 'bg-success-container text-on-success-container' : kind === 'accent' ? 'bg-tertiary-container text-on-tertiary-container' : 'bg-error-container text-on-error-container';
  return <div role={kind === 'bad' ? 'alert' : 'status'} className={`p-4 text-sm font-semibold rounded-2xl ${cls}`}>{children}</div>;
}

/** Etiquetas de tema en español (solo para mostrar; el valor que viaja a la API no cambia). */
export const TAG_LABELS: Record<string, string> = {
  politics: 'Política', economy: 'Economía', health: 'Salud', education: 'Educación', infrastructure: 'Infraestructura',
  environment: 'Ambiente', crime: 'Seguridad', grants: 'Fondos', federal: 'Federal', legislation: 'Legislación',
  government: 'Gobierno', technology: 'Tecnología', culture: 'Cultura', sports: 'Deportes', business: 'Negocios',
  'small-business': 'Pequeños negocios', nonprofit: 'Sin fines de lucro', entrepreneurship: 'Emprendimiento', tourism: 'Turismo',
  energy: 'Energía', agriculture: 'Agricultura', housing: 'Vivienda',
};
export const tagLabel = (t: string) => TAG_LABELS[t] || t;
