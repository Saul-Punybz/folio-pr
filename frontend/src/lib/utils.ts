/**
 * Returns a human-readable relative time string.
 * p. ej. "hace 2 h", "hace 3 d", "ahora"
 */
export function timeAgo(date: string): string {
  const now = Date.now();
  const then = new Date(date).getTime();
  const seconds = Math.floor((now - then) / 1000);

  if (seconds < 60) return 'ahora';

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `hace ${minutes} min`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `hace ${hours} h`;

  const days = Math.floor(hours / 24);
  if (days < 7) return `hace ${days} d`;

  const weeks = Math.floor(days / 7);
  if (weeks < 4) return `hace ${weeks} sem`;

  const months = Math.floor(days / 30);
  if (months < 12) return `hace ${months} m`;

  const years = Math.floor(days / 365);
  return `hace ${years} a`;
}

/**
 * Formats an ISO date string into a readable format.
 * p. ej. "26 feb 2026"
 */
export function formatDate(date: string): string {
  return new Date(date).toLocaleDateString('es-PR', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

/**
 * Returns Tailwind color classes for a region badge.
 */
export function regionColor(region: string): { bg: string; text: string } {
  switch (region.toLowerCase()) {
    case 'pr':
      return { bg: 'bg-primary/10 ', text: 'text-primary ' };
    case 'grants':
      return { bg: 'bg-success/10 ', text: 'text-success ' };
    case 'federal':
      return { bg: 'bg-primary/10 ', text: 'text-primary ' };
    case 'local':
      return { bg: 'bg-tertiary/10 ', text: 'text-tertiary ' };
    default:
      return { bg: 'bg-surface-container-highest/10 ', text: 'text-on-surface-variant ' };
  }
}

/**
 * Returns Tailwind color classes for a status badge.
 */
export function statusColor(status: string): { bg: string; text: string } {
  switch (status.toLowerCase()) {
    case 'inbox':
      return { bg: 'bg-primary/10 ', text: 'text-primary ' };
    case 'saved':
      return { bg: 'bg-success/10 ', text: 'text-success ' };
    case 'trashed':
      return { bg: 'bg-error/10 ', text: 'text-error ' };
    case 'archived':
      return { bg: 'bg-surface-container-highest/10 ', text: 'text-on-surface-variant ' };
    default:
      return { bg: 'bg-surface-container-highest/10 ', text: 'text-on-surface-variant ' };
  }
}

/**
 * Checks if an evidence expiry date has passed.
 */
export function isExpired(expiresAt: string): boolean {
  if (!expiresAt) return false;
  return new Date(expiresAt).getTime() < Date.now();
}

/**
 * Debounce utility.
 */
export function debounce<T extends (...args: any[]) => any>(
  fn: T,
  ms: number
): (...args: Parameters<T>) => void {
  let timer: ReturnType<typeof setTimeout>;
  return (...args: Parameters<T>) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), ms);
  };
}
