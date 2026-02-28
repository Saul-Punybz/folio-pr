/**
 * Returns a human-readable relative time string.
 * e.g. "2 hours ago", "3 days ago", "just now"
 */
export function timeAgo(date: string): string {
  const now = Date.now();
  const then = new Date(date).getTime();
  const seconds = Math.floor((now - then) / 1000);

  if (seconds < 60) return 'just now';

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  const days = Math.floor(hours / 24);
  if (days < 7) return `${days}d ago`;

  const weeks = Math.floor(days / 7);
  if (weeks < 4) return `${weeks}w ago`;

  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;

  const years = Math.floor(days / 365);
  return `${years}y ago`;
}

/**
 * Formats an ISO date string into a readable format.
 * e.g. "Feb 26, 2026"
 */
export function formatDate(date: string): string {
  return new Date(date).toLocaleDateString('en-US', {
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
      return { bg: 'bg-blue-500/10 dark:bg-blue-400/10', text: 'text-blue-600 dark:text-blue-400' };
    case 'grants':
      return { bg: 'bg-emerald-500/10 dark:bg-emerald-400/10', text: 'text-emerald-600 dark:text-emerald-400' };
    case 'federal':
      return { bg: 'bg-purple-500/10 dark:bg-purple-400/10', text: 'text-purple-600 dark:text-purple-400' };
    case 'local':
      return { bg: 'bg-amber-500/10 dark:bg-amber-400/10', text: 'text-amber-600 dark:text-amber-400' };
    default:
      return { bg: 'bg-zinc-500/10 dark:bg-zinc-400/10', text: 'text-zinc-600 dark:text-zinc-400' };
  }
}

/**
 * Returns Tailwind color classes for a status badge.
 */
export function statusColor(status: string): { bg: string; text: string } {
  switch (status.toLowerCase()) {
    case 'inbox':
      return { bg: 'bg-indigo-500/10 dark:bg-indigo-400/10', text: 'text-indigo-600 dark:text-indigo-400' };
    case 'saved':
      return { bg: 'bg-emerald-500/10 dark:bg-emerald-400/10', text: 'text-emerald-600 dark:text-emerald-400' };
    case 'trashed':
      return { bg: 'bg-red-500/10 dark:bg-red-400/10', text: 'text-red-600 dark:text-red-400' };
    case 'archived':
      return { bg: 'bg-zinc-500/10 dark:bg-zinc-400/10', text: 'text-zinc-600 dark:text-zinc-400' };
    default:
      return { bg: 'bg-zinc-500/10 dark:bg-zinc-400/10', text: 'text-zinc-600 dark:text-zinc-400' };
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
