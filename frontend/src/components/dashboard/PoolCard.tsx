'use client';

import { type Pool } from '@/lib/api';
import { formatBytes, formatDate, capacityColor, healthBg } from '@/lib/utils';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { RefreshCw, Layers, CheckCircle2, AlertCircle, Clock } from 'lucide-react';
import { cn } from '@/lib/utils';

interface Props {
  pool: Pool;
  hostId: string;
}

export function PoolCard({ pool, hostId }: Props) {
  const queryClient = useQueryClient();
  const usedPct = pool.size > 0 ? (pool.allocated / pool.size) * 100 : 0;

  const scrubMutation = useMutation({
    mutationFn: () => api.startScrub(hostId, pool.name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['pools', hostId] });
    },
  });

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden hover:border-gray-700 transition-colors">
      {/* Card header */}
      <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <div className="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center shrink-0">
            <Layers className="w-4 h-4 text-blue-400" />
          </div>
          <div className="min-w-0">
            <h3 className="text-base font-bold text-white truncate">{pool.name}</h3>
            <p className="text-xs text-gray-500">{pool.size > 0 ? formatBytes(pool.size) + ' total' : 'Unknown size'}</p>
          </div>
        </div>

        {/* Health badge */}
        <span
          className={cn(
            'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold ring-1 shrink-0',
            healthBg(pool.health)
          )}
        >
          {pool.health === 'ONLINE' ? (
            <CheckCircle2 className="w-3 h-3" />
          ) : (
            <AlertCircle className="w-3 h-3" />
          )}
          {pool.health}
        </span>
      </div>

      {/* Capacity */}
      <div className="px-5 pt-4 pb-2 space-y-2">
        <div className="flex items-center justify-between text-xs">
          <span className="text-gray-500">Capacity</span>
          <span className="font-medium text-white">{usedPct.toFixed(1)}% used</span>
        </div>
        {/* Progress bar */}
        <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
          <div
            className={cn('h-full rounded-full transition-all', capacityColor(usedPct))}
            style={{ width: `${Math.min(usedPct, 100)}%` }}
          />
        </div>
      </div>

      {/* Stats grid */}
      <div className="px-5 pb-4 grid grid-cols-3 gap-3 text-center">
        {[
          { label: 'Used', value: formatBytes(pool.allocated) },
          { label: 'Free', value: formatBytes(pool.free) },
          { label: 'Frag', value: `${pool.fragmentation}%` },
        ].map((s) => (
          <div key={s.label} className="bg-gray-800/50 rounded-lg py-2">
            <p className="text-xs text-gray-500">{s.label}</p>
            <p className="text-sm font-semibold text-white mt-0.5">{s.value}</p>
          </div>
        ))}
      </div>

      {/* Scrub info */}
      <div className="px-5 pb-4">
        <div className="bg-gray-800/40 border border-gray-800 rounded-lg px-3 py-2.5 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 min-w-0">
            <Clock className="w-3.5 h-3.5 text-gray-500 shrink-0" />
            <div className="min-w-0">
              <p className="text-xs text-gray-500">Last scrub</p>
              {pool.last_scrub ? (
                <p className="text-xs text-white truncate">
                  {formatDate(pool.last_scrub.date)}{' '}
                  <span
                    className={cn(
                      'font-medium',
                      pool.last_scrub.status === 'completed'
                        ? pool.last_scrub.errors === 0
                          ? 'text-green-400'
                          : 'text-yellow-400'
                        : 'text-gray-500'
                    )}
                  >
                    {pool.last_scrub.errors === 0
                      ? '✓ clean'
                      : `${pool.last_scrub.errors} errors`}
                  </span>
                </p>
              ) : (
                <p className="text-xs text-gray-600">Never</p>
              )}
            </div>
          </div>

          <button
            onClick={() => scrubMutation.mutate()}
            disabled={scrubMutation.isPending}
            className="flex items-center gap-1.5 text-xs text-blue-400 hover:text-blue-300 border border-blue-500/30 hover:border-blue-500/60 rounded-lg px-2.5 py-1 transition-colors shrink-0 disabled:opacity-50"
          >
            <RefreshCw className={cn('w-3 h-3', scrubMutation.isPending && 'animate-spin')} />
            {scrubMutation.isPending ? 'Starting…' : 'Scrub'}
          </button>
        </div>
      </div>

      {/* Footer */}
      <div className="px-5 pb-4">
        <a
          href={`/hosts/${hostId}/pools/${pool.name}/datasets`}
          className="w-full flex items-center justify-center gap-1.5 text-xs text-gray-500 hover:text-white border border-gray-800 hover:border-gray-700 rounded-lg py-2 transition-colors"
        >
          View Datasets →
        </a>
      </div>
    </div>
  );
}
