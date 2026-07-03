'use client';

import { useQuery } from '@tanstack/react-query';
import { api, type ZFSEvent } from '@/lib/api';
import { formatDate } from '@/lib/utils';
import { Activity, AlertCircle, AlertTriangle, Info } from 'lucide-react';
import { cn } from '@/lib/utils';

function severityIcon(sev: ZFSEvent['severity']) {
  switch (sev) {
    case 'error': return <AlertCircle className="w-4 h-4 text-red-500 shrink-0" />;
    case 'warning': return <AlertTriangle className="w-4 h-4 text-yellow-400 shrink-0" />;
    default: return <Info className="w-4 h-4 text-blue-400 shrink-0" />;
  }
}

function severityBg(sev: ZFSEvent['severity']) {
  switch (sev) {
    case 'error': return 'border-l-red-500';
    case 'warning': return 'border-l-yellow-400';
    default: return 'border-l-blue-500';
  }
}

interface Props {
  hostId: string;
}

export function EventFeed({ hostId }: Props) {
  const { data: events, isLoading } = useQuery({
    queryKey: ['events', hostId],
    queryFn: () => api.listEvents(hostId),
    refetchInterval: 30_000,
  });

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
      {/* Header */}
      <div className="px-5 py-4 border-b border-gray-800 flex items-center gap-3">
        <Activity className="w-4 h-4 text-gray-400" />
        <h2 className="text-sm font-semibold text-white">Recent Events</h2>
        {events && events.length > 0 && (
          <span className="ml-auto text-xs text-gray-600">{events.length} events</span>
        )}
      </div>

      {/* List */}
      <div className="divide-y divide-gray-800">
        {isLoading && (
          <div className="flex flex-col gap-3 px-5 py-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-10 bg-gray-800 rounded animate-pulse" />
            ))}
          </div>
        )}

        {!isLoading && (!events || events.length === 0) && (
          <div className="px-5 py-8 text-center">
            <Activity className="w-8 h-8 text-gray-700 mx-auto mb-2" />
            <p className="text-sm text-gray-600">No recent events</p>
          </div>
        )}

        {events?.slice(0, 10).map((ev) => (
          <div
            key={ev.id}
            className={cn(
              'flex items-start gap-3 px-5 py-3.5 border-l-2',
              severityBg(ev.severity)
            )}
          >
            <div className="pt-0.5">{severityIcon(ev.severity)}</div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-xs font-semibold text-gray-400 font-mono">
                  {ev.pool}
                </span>
                <span className="text-xs text-gray-600">•</span>
                <span className="text-xs text-gray-500">{ev.type}</span>
              </div>
              <p className="text-sm text-white mt-0.5 truncate">{ev.message}</p>
            </div>
            <span className="text-xs text-gray-600 whitespace-nowrap shrink-0 pt-0.5">
              {formatDate(ev.timestamp)}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
