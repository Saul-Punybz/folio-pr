import { useState, useEffect } from 'react';
import { api } from '../lib/api';

// Types
interface TagTrend { tag: string; day: string; count: number; }
interface EntityCount { name: string; type: string; count: number; }
interface SentimentCount { sentiment: string; count: number; }
interface SourceHealth { source: string; article_count: number; last_ingested: string; enriched_count: number; }
interface VolumePoint { day: string; count: number; }

const DAY_OPTIONS = [7, 14, 30, 90];

function Skeleton({ className = '' }: { className?: string }) {
  return <div className={`animate-pulse bg-zinc-200 dark:bg-zinc-800 rounded ${className}`} />;
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 p-5">
      <h2 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100 mb-4">{title}</h2>
      {children}
    </div>
  );
}

function entityTypeBadge(type: string) {
  const colors: Record<string, string> = {
    person: 'bg-blue-100 dark:bg-blue-500/20 text-blue-700 dark:text-blue-400',
    organization: 'bg-green-100 dark:bg-green-500/20 text-green-700 dark:text-green-400',
    place: 'bg-amber-100 dark:bg-amber-500/20 text-amber-700 dark:text-amber-400',
  };
  const cls = colors[type.toLowerCase()] || 'bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400';
  return (
    <span className={`inline-flex items-center px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded-full ${cls}`}>
      {type}
    </span>
  );
}

function formatShortDate(dateStr: string) {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

export default function AnalyticsDashboard() {
  const [days, setDays] = useState(30);
  const [loading, setLoading] = useState(true);

  const [volume, setVolume] = useState<VolumePoint[]>([]);
  const [entities, setEntities] = useState<EntityCount[]>([]);
  const [tagTrends, setTagTrends] = useState<TagTrend[]>([]);
  const [sentiment, setSentiment] = useState<SentimentCount[]>([]);
  const [sourceHealth, setSourceHealth] = useState<SourceHealth[]>([]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    Promise.all([
      api.getArticleVolume(days),
      api.getTopEntities(days),
      api.getTagTrends(days),
      api.getSentiment(days),
      api.getSourceHealth(),
    ])
      .then(([volData, entData, tagData, sentData, srcData]) => {
        if (cancelled) return;
        setVolume(volData.volume || []);
        setEntities(entData.entities || []);
        setTagTrends(tagData.trends || []);
        setSentiment(sentData.sentiment || []);
        setSourceHealth(srcData.sources || []);
      })
      .catch((err) => {
        if (!cancelled) console.error('Analytics fetch error:', err);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => { cancelled = true; };
  }, [days]);

  // Aggregate tags
  const aggregatedTags = tagTrends.reduce<Record<string, number>>((acc, t) => {
    acc[t.tag] = (acc[t.tag] || 0) + t.count;
    return acc;
  }, {});
  const sortedTags = Object.entries(aggregatedTags)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 20);
  const maxTagCount = sortedTags.length > 0 ? sortedTags[0][1] : 1;

  // Volume max
  const maxVolume = volume.length > 0 ? Math.max(...volume.map((v) => v.count)) : 1;

  // Sentiment totals
  const sentimentTotal = sentiment.reduce((acc, s) => acc + s.count, 0) || 1;

  const sentimentColors: Record<string, string> = {
    positive: 'bg-emerald-500',
    neutral: 'bg-zinc-400 dark:bg-zinc-500',
    negative: 'bg-red-500',
  };

  const sentimentTextColors: Record<string, string> = {
    positive: 'text-emerald-600 dark:text-emerald-400',
    neutral: 'text-zinc-500 dark:text-zinc-400',
    negative: 'text-red-600 dark:text-red-400',
  };

  return (
    <div className="space-y-6">
      {/* Days selector */}
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium text-zinc-500 dark:text-zinc-400 mr-1">Period:</span>
        {DAY_OPTIONS.map((d) => (
          <button
            key={d}
            onClick={() => setDays(d)}
            className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
              days === d
                ? 'bg-indigo-500 text-white'
                : 'bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400 hover:bg-zinc-200 dark:hover:bg-zinc-700'
            }`}
          >
            {d}d
          </button>
        ))}
      </div>

      {loading ? (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="rounded-lg border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 p-5">
              <Skeleton className="h-4 w-32 mb-4" />
              <div className="space-y-2">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-4 w-1/2" />
                <Skeleton className="h-4 w-5/6" />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* 1. Article Volume */}
          <Panel title="Article Volume">
            {volume.length === 0 ? (
              <p className="text-xs text-zinc-400 dark:text-zinc-500">No data available.</p>
            ) : (
              <div className="space-y-1.5 max-h-64 overflow-y-auto">
                {volume.map((v) => (
                  <div key={v.day} className="flex items-center gap-2">
                    <span className="text-[10px] font-mono text-zinc-500 dark:text-zinc-400 w-14 shrink-0">
                      {formatShortDate(v.day)}
                    </span>
                    <div className="flex-1 h-4 bg-zinc-100 dark:bg-zinc-800 rounded overflow-hidden">
                      <div
                        className="h-4 bg-indigo-500 rounded transition-all"
                        style={{ width: `${(v.count / maxVolume) * 100}%` }}
                      />
                    </div>
                    <span className="text-[10px] font-mono text-zinc-500 dark:text-zinc-400 w-8 text-right shrink-0">
                      {v.count}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </Panel>

          {/* 2. Top Entities */}
          <Panel title="Top Entities">
            {entities.length === 0 ? (
              <p className="text-xs text-zinc-400 dark:text-zinc-500">No entities found.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-zinc-200 dark:border-zinc-800">
                      <th className="text-left text-[10px] font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400 pb-2">Name</th>
                      <th className="text-left text-[10px] font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400 pb-2">Type</th>
                      <th className="text-right text-[10px] font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400 pb-2">Count</th>
                    </tr>
                  </thead>
                  <tbody>
                    {entities.slice(0, 15).map((e, i) => (
                      <tr key={i} className="border-b border-zinc-100 dark:border-zinc-800/50">
                        <td className="py-1.5 text-xs text-zinc-900 dark:text-zinc-100">{e.name}</td>
                        <td className="py-1.5">{entityTypeBadge(e.type)}</td>
                        <td className="py-1.5 text-xs text-zinc-500 dark:text-zinc-400 text-right font-mono">{e.count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Panel>

          {/* 3. Tag Frequency */}
          <Panel title="Tag Frequency">
            {sortedTags.length === 0 ? (
              <p className="text-xs text-zinc-400 dark:text-zinc-500">No tags found.</p>
            ) : (
              <div className="space-y-1.5 max-h-64 overflow-y-auto">
                {sortedTags.map(([tag, count]) => (
                  <div key={tag} className="flex items-center gap-2">
                    <span className="text-xs text-zinc-700 dark:text-zinc-300 w-28 truncate shrink-0" title={tag}>
                      {tag}
                    </span>
                    <div className="flex-1 h-3.5 bg-zinc-100 dark:bg-zinc-800 rounded overflow-hidden">
                      <div
                        className="h-3.5 bg-indigo-400 dark:bg-indigo-500 rounded transition-all"
                        style={{ width: `${(count / maxTagCount) * 100}%` }}
                      />
                    </div>
                    <span className="text-[10px] font-mono text-zinc-500 dark:text-zinc-400 w-8 text-right shrink-0">
                      {count}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </Panel>

          {/* 4. Sentiment Breakdown */}
          <Panel title="Sentiment Breakdown">
            {sentiment.length === 0 ? (
              <p className="text-xs text-zinc-400 dark:text-zinc-500">No sentiment data.</p>
            ) : (
              <div className="space-y-4">
                {/* Percentage bar */}
                <div className="flex h-6 rounded-lg overflow-hidden">
                  {sentiment.map((s) => {
                    const pct = (s.count / sentimentTotal) * 100;
                    if (pct === 0) return null;
                    return (
                      <div
                        key={s.sentiment}
                        className={`${sentimentColors[s.sentiment] || 'bg-zinc-300'} transition-all`}
                        style={{ width: `${pct}%` }}
                        title={`${s.sentiment}: ${s.count} (${pct.toFixed(1)}%)`}
                      />
                    );
                  })}
                </div>

                {/* Legend */}
                <div className="flex items-center gap-4">
                  {sentiment.map((s) => {
                    const pct = ((s.count / sentimentTotal) * 100).toFixed(1);
                    return (
                      <div key={s.sentiment} className="flex items-center gap-1.5">
                        <div className={`w-2.5 h-2.5 rounded-full ${sentimentColors[s.sentiment] || 'bg-zinc-300'}`} />
                        <span className={`text-xs font-medium capitalize ${sentimentTextColors[s.sentiment] || 'text-zinc-500'}`}>
                          {s.sentiment}
                        </span>
                        <span className="text-[10px] text-zinc-400 dark:text-zinc-500">
                          {s.count} ({pct}%)
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </Panel>

          {/* 5. Source Health */}
          <Panel title="Source Health">
            {sourceHealth.length === 0 ? (
              <p className="text-xs text-zinc-400 dark:text-zinc-500">No sources configured.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-zinc-200 dark:border-zinc-800">
                      <th className="text-left text-[10px] font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400 pb-2">Source</th>
                      <th className="text-right text-[10px] font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400 pb-2">Articles</th>
                      <th className="text-right text-[10px] font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400 pb-2">Last Ingested</th>
                      <th className="text-right text-[10px] font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400 pb-2">Enriched %</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sourceHealth.map((s, i) => {
                      const enrichedPct = s.article_count > 0
                        ? ((s.enriched_count / s.article_count) * 100).toFixed(0)
                        : '0';
                      return (
                        <tr key={i} className="border-b border-zinc-100 dark:border-zinc-800/50">
                          <td className="py-1.5 text-xs text-zinc-900 dark:text-zinc-100">{s.source}</td>
                          <td className="py-1.5 text-xs text-zinc-500 dark:text-zinc-400 text-right font-mono">{s.article_count}</td>
                          <td className="py-1.5 text-xs text-zinc-500 dark:text-zinc-400 text-right">
                            {s.last_ingested ? formatShortDate(s.last_ingested) : 'never'}
                          </td>
                          <td className="py-1.5 text-right">
                            <span className={`text-xs font-mono ${
                              Number(enrichedPct) >= 80
                                ? 'text-emerald-600 dark:text-emerald-400'
                                : Number(enrichedPct) >= 50
                                  ? 'text-amber-600 dark:text-amber-400'
                                  : 'text-red-600 dark:text-red-400'
                            }`}>
                              {enrichedPct}%
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </Panel>
        </div>
      )}
    </div>
  );
}
