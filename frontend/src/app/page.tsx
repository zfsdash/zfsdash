'use client';

import { useState, useCallback } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, type Host } from '@/lib/api';
import { DashboardHeader } from '@/components/dashboard/Header';
import { QuickStats } from '@/components/dashboard/QuickStats';
import { PoolCard } from '@/components/dashboard/PoolCard';
import { EventFeed } from '@/components/dashboard/EventFeed';
import { EmptyState } from '@/components/dashboard/EmptyState';
import { Database, Loader2 } from 'lucide-react';

export default function DashboardPage() {
  const queryClient = useQueryClient();
  const [selectedHost, setSelectedHost] = useState<Host | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Fetch hosts
  const { data: hosts = [], isLoading: hostsLoading } = useQuery({
    queryKey: ['hosts'],
    queryFn: api.listHosts,
    onSuccess: (data: Host[]) => {
      if (!selectedHost && data.length > 0 && data[0]) {
        setSelectedHost(data[0]);
      }
    },
  } as Parameters<typeof useQuery>[0]);

  // Fetch pools for selected host
  const {
    data: pools = [],
    isLoading: poolsLoading,
    error: poolsError,
  } = useQuery({
    queryKey: ['pools', selectedHost?.id],
    queryFn: () => api.listPools(selectedHost!.id),
    enabled: !!selectedHost,
    refetchInterval: 30_000,
  });

  const refresh = useCallback(async () => {
    setIsRefreshing(true);
    await queryClient.invalidateQueries();
    setTimeout(() => setIsRefreshing(false), 800);
  }, [queryClient]);

  // Full-page loading state
  if (hostsLoading) {
    return (
      <div className="min-h-screen bg-gray-950 flex items-center justify-center">
        <div className="flex flex-col items-center gap-3 text-gray-600">
          <Loader2 className="w-8 h-8 animate-spin" />
          <p className="text-sm">Loading ZFSdash…</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-950 flex flex-col">
      <DashboardHeader
        selectedHost={selectedHost}
        hosts={hosts}
        onSelectHost={setSelectedHost}
        onRefresh={refresh}
        isRefreshing={isRefreshing}
      />

      <main className="flex-1 px-4 sm:px-6 py-6 max-w-screen-xl mx-auto w-full">
        {/* No hosts */}
        {hosts.length === 0 && <EmptyState type="no-hosts" />}

        {selectedHost && (
          <div className="flex flex-col gap-6">
            {/* Quick Stats */}
            {pools.length > 0 && <QuickStats pools={pools} />}

            {/* Pool Grid */}
            <section>
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-sm font-semibold text-gray-400 uppercase tracking-wider flex items-center gap-2">
                  <Database className="w-4 h-4" />
                  Pools
                  {pools.length > 0 && (
                    <span className="ml-1 text-gray-600 font-normal normal-case tracking-normal">
                      — {selectedHost.name}
                    </span>
                  )}
                </h2>
                {poolsLoading && (
                  <Loader2 className="w-4 h-4 animate-spin text-gray-600" />
                )}
              </div>

              {poolsError ? (
                <div className="bg-red-500/10 border border-red-500/30 rounded-xl px-5 py-4 text-red-400 text-sm">
                  Failed to load pools: {(poolsError as Error).message}
                </div>
              ) : poolsLoading ? (
                <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
                  {[1, 2, 3].map((i) => (
                    <div key={i} className="bg-gray-900 border border-gray-800 rounded-xl h-64 animate-pulse" />
                  ))}
                </div>
              ) : pools.length === 0 ? (
                <EmptyState type="no-pools" hostId={selectedHost.id} />
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
                  {pools.map((pool) => (
                    <PoolCard key={pool.name} pool={pool} hostId={selectedHost.id} />
                  ))}
                </div>
              )}
            </section>

            {/* Events feed */}
            {selectedHost && (
              <EventFeed hostId={selectedHost.id} />
            )}
          </div>
        )}
      </main>
    </div>
  );
}
