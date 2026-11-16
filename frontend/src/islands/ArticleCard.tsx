import type { Article } from '../lib/api';
import { timeAgo } from '../lib/utils';
import { Icon, tagLabel } from './ui';

interface ArticleCardProps {
  article: Article;
  selected: boolean;
  onSave?: (id: string) => void;
  onTrash?: (id: string) => void;
  onPin?: (id: string) => void;
  onClick?: () => void;
  showActions?: boolean;
  /** Modo selección: casilla en la tarjeta. */
  selecting?: boolean;
  checked?: boolean;
  onToggle?: (id: string) => void;
}

export default function ArticleCard({
  article,
  selected,
  onSave,
  onTrash,
  onPin,
  onClick,
  showActions = true,
  selecting = false,
  checked = false,
  onToggle,
}: ArticleCardProps) {
  const description =
    article.summary ||
    (article.clean_text
      ? article.clean_text.slice(0, 160) + (article.clean_text.length > 160 ? '...' : '')
      : '');

  return (
    <article
      onClick={selecting && onToggle ? () => onToggle(article.id) : onClick}
      className={`group relative flex flex-col h-full overflow-hidden cursor-pointer
        bg-surface-container-low hover:bg-surface-container
        rounded-[28px] hover:rounded-[36px]
        transition-[border-radius,background-color,transform,box-shadow] duration-500 ease-[var(--spring)]
        active:scale-[0.985]
        ${selected || checked ? 'ring-[3px] ring-primary shadow-lg' : ''}`}
    >
      <div className="px-5 pt-5 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs min-w-0 text-on-surface-variant">
          {selecting && (
            <input
              type="checkbox"
              checked={checked}
              onChange={() => onToggle?.(article.id)}
              onClick={(e) => e.stopPropagation()}
              aria-label={`Seleccionar: ${article.title}`}
              className="shrink-0"
            />
          )}
          {article.pinned && (
            <span className="text-tertiary shrink-0" title="Fijado">
              <Icon name="pin" className="w-4 h-4" fill />
            </span>
          )}
          <span className="font-semibold truncate">{article.source}</span>
          <span aria-hidden="true">&middot;</span>
          <span className="shrink-0">{timeAgo(article.published_at || article.created_at)}</span>
        </div>
        <span className="tag tag-primary shrink-0 uppercase tracking-wider">{article.region}</span>
      </div>

      <h3 className="px-5 pt-3 text-[1.1875rem] leading-[1.2] font-extrabold text-on-surface line-clamp-3 tracking-tight">
        {article.title}
      </h3>

      {description && (
        <p className="px-5 pt-2 text-sm text-on-surface-variant leading-relaxed line-clamp-3 flex-1">{description}</p>
      )}
      {!description && <div className="flex-1" />}

      {article.tags && article.tags.length > 0 && (
        <div className="px-5 pt-3 flex flex-wrap gap-1.5">
          {article.tags.slice(0, 3).map((tag) => (
            <span key={tag} className="tag">{tagLabel(tag)}</span>
          ))}
          {article.tags.length > 3 && <span className="tag">+{article.tags.length - 3}</span>}
        </div>
      )}

      {article.image_url && (
        <div className="mx-3 mt-4 h-36 overflow-hidden rounded-[20px] bg-surface-container-high">
          <img
            src={article.image_url}
            alt=""
            className="w-full h-full object-cover transition-transform duration-700 group-hover:scale-105"
            loading="lazy"
            onError={(e) => {
              (e.target as HTMLImageElement).parentElement!.style.display = 'none';
            }}
          />
        </div>
      )}

      {showActions && !selecting && (
        <div className="card-actions">
          {onSave && (
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); onSave(article.id); }}
              className="btn btn-filled btn-sm card-save"
            >
              <Icon name="save" className="w-5 h-5 shrink-0" />
              <span>Guardar</span>
            </button>
          )}
          {onTrash && (
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); onTrash(article.id); }}
              className="icon-btn"
              title="Borrar"
              aria-label="Borrar"
            >
              <Icon name="trash" className="w-6 h-6" />
            </button>
          )}
          {onPin && (
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); onPin(article.id); }}
              className={`icon-btn ${article.pinned ? 'on' : ''}`}
              title={article.pinned ? 'Quitar de fijados' : 'Fijar'}
              aria-label={article.pinned ? 'Quitar de fijados' : 'Fijar'}
              aria-pressed={!!article.pinned}
            >
              <Icon name="pin" className="w-6 h-6" fill={!!article.pinned} />
            </button>
          )}
          {article.url && (
            <a
              href={article.url}
              target="_blank"
              rel="noopener noreferrer"
              className="icon-btn ml-auto"
              title="Abrir original"
              aria-label="Abrir original"
              onClick={(e) => e.stopPropagation()}
            >
              <Icon name="open" className="w-6 h-6" />
            </a>
          )}
        </div>
      )}
    </article>
  );
}
