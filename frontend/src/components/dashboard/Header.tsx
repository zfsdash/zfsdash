'use client';

import { Logo } from '@/components/Logo';
import { useQuery } from '@tanstack/react-query';
import { api, type Host } from '@/lib/api';
import { useState, useRef, useEffect } from 'react';
import { ChevronDown, LogOut, User, RefreshCw, AlertCircle } from 'lucide-react';
import { cn } from '@/lib/utils';

interface Props {
  selectedHost: Host | null;
  hosts: Host[];
  onSelectHost: (host: Host) => void;
  onRefresh: () => void;
  isRefreshing: boolean;
}

export function DashboardHeader({
  selectedHost,
  hosts,
  onSelectHost,
  onRefresh,
  isRefreshing,
}: Props) {
  const { data: user } = useQuery({
    queryKey: ['me'],
    queryFn: api.me,
    retry: false,
  });

  const [hostOpen, setHostOpen] = useState(false);
  const [userOpen, setUserOpen] = useState(false);
  const hostRef = useRef<HTMLDivElement>(null);
  const userRef = useRef<HTMLDivElement>(null);

  // Close dropdowns on outside click
  useEffect(() => {
    function handle(e: MouseEvent) {
      if (hostRef.current && !hostRef.current.contains(e.target as Node)) setHostOpen(false);
      if (userRef.current && !userRef.current.contains(e.target as Node)) setUserOpen(false);
    }
    document.addEventListener('mousedown', handle);
    return () => document.removeEventListener('mousedown', handle);
  }, []);

  async function logout() {
    try { await api.logout(); } catch {}
    window.location.href = '/login';
  }

  return (
    <header className="h-14 border-b border-gray-800 bg-gray-900 px-4 flex items-center justify-between gap-4 shrink-0">
      {/* Left: Logo */}
      <div className="flex items-center gap-4">
        <Logo size="sm" />
        <span className="text-xs text-gray-600 font-mono hidden sm:block">v0.1.0</span>
      </div>

      {/* Center: Host selector */}
      <div className="flex-1 flex justify-center" ref={hostRef}>
        {hosts.length > 0 ? (
          <div className="relative">
            <button
              onClick={() => setHostOpen((v) => !v)}
              className="flex items-center gap-2 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded-lg px-4 py-1.5 text-sm transition-colors"
            >
              {/* Health dot */}
              <span
                className={cn(
                  'w-2 h-2 rounded-full',
                  selectedHost?.online ? 'bg-green-400' : 'bg-red-500'
                )}
              />
              <span className="text-white font-medium">
                {selectedHost?.name ?? 'Select host'}
              </span>
              <ChevronDown className={cn('w-4 h-4 text-gray-500 transition-transform', hostOpen && 'rotate-180')} />
            </button>

            {hostOpen && (
              <div className="absolute top-full mt-1 left-1/2 -translate-x-1/2 w-56 bg-gray-800 border border-gray-700 rounded-xl shadow-2xl overflow-hidden z-50">
                <div className="px-3 py-2 border-b border-gray-700">
                  <p className="text-xs text-gray-500 uppercase tracking-wider font-semibold">Hosts</p>
                </div>
                {hosts.map((h) => (
                  <button
                    key={h.id}
                    onClick={() => { onSelectHost(h); setHostOpen(false); }}
                    className={cn(
                      'w-full flex items-center gap-3 px-4 py-2.5 text-sm hover:bg-gray-700 transition-colors text-left',
                      selectedHost?.id === h.id && 'bg-blue-600/20 text-blue-300'
                    )}
                  >
                    <span className={cn('w-2 h-2 rounded-full shrink-0', h.online ? 'bg-green-400' : 'bg-red-500')} />
                    <span className="text-white">{h.name}</span>
                    <span className="text-xs text-gray-500 ml-auto">{h.mode}</span>
                  </button>
                ))}
                <div className="border-t border-gray-700 px-4 py-2">
                  <a
                    href="/hosts/new"
                    className="text-xs text-blue-400 hover:text-blue-300 transition-colors"
                  >
                    + Add host
                  </a>
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className="flex items-center gap-2 text-yellow-400 text-sm">
            <AlertCircle className="w-4 h-4" />
            No hosts configured
          </div>
        )}
      </div>

      {/* Right: Actions */}
      <div className="flex items-center gap-2">
        <button
          onClick={onRefresh}
          disabled={isRefreshing}
          title="Refresh"
          className="p-2 rounded-lg text-gray-400 hover:text-white hover:bg-gray-800 transition-colors"
        >
          <RefreshCw className={cn('w-4 h-4', isRefreshing && 'animate-spin')} />
        </button>

        {/* User menu */}
        <div className="relative" ref={userRef}>
          <button
            onClick={() => setUserOpen((v) => !v)}
            className="flex items-center gap-2 p-1.5 rounded-lg hover:bg-gray-800 transition-colors"
          >
            <div className="w-7 h-7 rounded-full bg-blue-600 flex items-center justify-center text-xs font-bold text-white uppercase">
              {user?.username?.[0] ?? 'A'}
            </div>
            <ChevronDown className="w-3.5 h-3.5 text-gray-500" />
          </button>

          {userOpen && (
            <div className="absolute top-full mt-1 right-0 w-48 bg-gray-800 border border-gray-700 rounded-xl shadow-2xl overflow-hidden z-50">
              <div className="px-4 py-3 border-b border-gray-700">
                <p className="text-sm font-medium text-white">{user?.username ?? '—'}</p>
                <p className="text-xs text-gray-500 truncate">{user?.email ?? ''}</p>
              </div>
              <button
                onClick={logout}
                className="w-full flex items-center gap-3 px-4 py-2.5 text-sm text-red-400 hover:bg-gray-700 hover:text-red-300 transition-colors"
              >
                <LogOut className="w-4 h-4" />
                Sign out
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
