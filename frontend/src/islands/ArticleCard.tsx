import type { Article } from '../lib/api';
import { timeAgo, regionColor } from '../lib/utils';

interface ArticleCardProps {
  article: Article;
  selected: boolean;
  onSave?: (id: string) => void;
  onTrash?: (id: string) => void;
  onPin?: (id: string) => void;
  onClick?: () => void;
  showActions?: boolean;
}

export default function ArticleCard({
  article,
  selected,
  onSave,
  onTrash,
  onPin,
  onClick,
  showActions = true,
}: ArticleCardProps) {
  const region = regionColor(article.region);

  const description =
    article.summary ||
    (article.clean_text
      ? article.clean_text.slice(0, 160) + (article.clean_text.length > 160 ? '...' : '')
      : '');

  return (
    <div
      onClick={onClick}
      className={`
        group relative flex flex-col h-full overflow-hidden cursor-pointer transition-all duration-150
        border rounded-sm
        ${
          selected
            ? 'ring-2 ring-indigo-500 border-indigo-400 shadow-lg'
            : 'border-zinc-300 dark:border-zinc-700 hover:shadow-md hover:border-zinc-400 dark:hover:border-zinc-500'
        }
      `}
    >
      {/* Dark header bar — newspaper style */}
      <div className="bg-zinc-800 dark:bg-zinc-950 px-3 py-2 flex items-center justify-between">
        <div className="flex items-center gap-1.5 text-[10px] text-zinc-400 min-w-0">
          {article.pinned && (
            <svg className="w-3 h-3 text-yellow-400 shrink-0" fill="currentColor" viewBox="0 0 24 24">
              <path d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.563.563 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.563.563 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z" />
            </svg>
          )}
          <span className="font-medium text-zinc-300 truncate">{article.source}</span>
          <span className="text-zinc-600 shrink-0">&middot;</span>
          <span className="text-zinc-500 shrink-0">{timeAgo(article.published_at || article.created_at)}</span>
        </div>
        <span
          className="inline-flex items-center px-2 py-0.5 text-[9px] font-bold uppercase tracking-widest bg-zinc-100 text-zinc-800 rounded-sm shrink-0"
        >
          {article.region}
        </span>
      </div>

      {/* Title area — large, bold, newspaper-like */}
      <div className="bg-zinc-50 dark:bg-zinc-900 px-3 py-3 border-b border-zinc-200 dark:border-zinc-800">
        <h3 className="text-[15px] font-extrabold text-zinc-900 dark:text-zinc-50 leading-tight line-clamp-3 uppercase tracking-tight">
          {article.title}
        </h3>
      </div>

      {/* Body: summary + image */}
      <div className="flex-1 flex flex-col bg-white dark:bg-zinc-900">
        {/* Summary */}
        {description && (
          <div className="px-3 pt-2.5 pb-2 flex-1">
            <p className="text-[11px] text-zinc-600 dark:text-zinc-400 leading-relaxed line-clamp-3">
              {description}
            </p>
          </div>
        )}

        {/* Tags */}
        {article.tags && article.tags.length > 0 && (
          <div className="px-3 pb-2 flex flex-wrap gap-1">
            {article.tags.slice(0, 3).map((tag) => (
              <span
                key={tag}
                className="inline-flex items-center px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wider rounded-sm bg-zinc-100 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-500"
              >
                {tag}
              </span>
            ))}
            {article.tags.length > 3 && (
              <span className="text-[9px] text-zinc-400">+{article.tags.length - 3}</span>
            )}
          </div>
        )}

        {/* Photo at bottom */}
        {article.image_url && (
          <div className="w-full h-32 overflow-hidden bg-zinc-200 dark:bg-zinc-800">
            <img
              src={article.image_url}
              alt=""
              className="w-full h-full object-cover"
              loading="lazy"
              onError={(e) => {
                (e.target as HTMLImageElement).parentElement!.style.display = 'none';
              }}
            />
          </div>
        )}
      </div>

      {/* Action buttons — bottom bar */}
      {showActions && (
        <div className="flex items-center gap-1 px-2 py-1.5 bg-zinc-50 dark:bg-zinc-950 border-t border-zinc-200 dark:border-zinc-800">
          {onSave && (
            <button
              onClick={(e) => { e.stopPropagation(); onSave(article.id); }}
              className="inline-flex items-center gap-1 px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-white bg-zinc-800 hover:bg-indigo-600 rounded-sm transition-colors"
            >
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M17.593 3.322c1.1.128 1.907 1.077 1.907 2.185V21L12 17.25 4.5 21V5.507c0-1.108.806-2.057 1.907-2.185a48.507 48.507 0 0111.186 0z" />
              </svg>
              Save
            </button>
          )}
          {onTrash && (
            <button
              onClick={(e) => { e.stopPropagation(); onTrash(article.id); }}
              className="inline-flex items-center gap-1 px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-zinc-500 hover:text-red-500 bg-zinc-200 dark:bg-zinc-800 hover:bg-red-50 dark:hover:bg-red-500/10 rounded-sm transition-colors"
            >
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
              </svg>
              Trash
            </button>
          )}
          {onPin && (
            <button
              onClick={(e) => { e.stopPropagation(); onPin(article.id); }}
              className={`inline-flex items-center p-1 rounded-sm transition-colors ${
                article.pinned ? 'text-yellow-500' : 'text-zinc-400 hover:text-yellow-500'
              }`}
              title={article.pinned ? 'Unpin' : 'Pin'}
            >
              <svg className="w-3 h-3" fill={article.pinned ? 'currentColor' : 'none'} viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.563.563 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.563.563 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z" />
              </svg>
            </button>
          )}
          {article.url && (
            <a
              href={article.url}
              target="_blank"
              rel="noopener noreferrer"
              className="ml-auto p-1 text-zinc-400 hover:text-indigo-500 transition-colors"
              title="Open original"
              onClick={(e) => e.stopPropagation()}
            >
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
              </svg>
            </a>
          )}
        </div>
      )}
    </div>
  );
}
