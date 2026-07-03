// All API calls go to same-origin /api/*

export interface Host {
  id: string;
  name: string;
  mode: 'local' | 'ssh' | 'truenas';
  hostname?: string;
  online: boolean;
}

export interface Pool {
  name: string;
  health: 'ONLINE' | 'DEGRADED' | 'FAULTED' | 'UNAVAIL' | 'REMOVED' | 'OFFLINE';
  size: number;
  allocated: number;
  free: number;
  fragmentation: number;
  last_scrub?: {
    date: string;
    status: string;
    errors: number;
  };
}

export interface ZFSEvent {
  id: string;
  timestamp: string;
  pool: string;
  type: string;
  message: string;
  severity: 'info' | 'warning' | 'error';
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(body || `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  // Setup
  setupInit: (data: { username: string; email: string; password: string }) =>
    apiFetch('/api/setup/init', { method: 'POST', body: JSON.stringify(data) }),

  setupStatus: () =>
    apiFetch<{ initialized: boolean }>('/api/setup/status'),

  // Hosts
  listHosts: () => apiFetch<Host[]>('/api/hosts'),

  addHost: (data: {
    name: string;
    mode: 'local' | 'ssh' | 'truenas';
    hostname?: string;
    port?: number;
    username?: string;
    password?: string;
    private_key?: string;
    api_key?: string;
  }) => apiFetch<Host>('/api/hosts', { method: 'POST', body: JSON.stringify(data) }),

  // Pools
  listPools: (hostId: string) =>
    apiFetch<Pool[]>(`/api/hosts/${hostId}/pools`),

  startScrub: (hostId: string, pool: string) =>
    apiFetch(`/api/hosts/${hostId}/pools/${pool}/scrub`, { method: 'POST' }),

  // Events
  listEvents: (hostId: string) =>
    apiFetch<ZFSEvent[]>(`/api/hosts/${hostId}/events`),

  // Auth
  me: () => apiFetch<{ username: string; email: string }>('/api/auth/me'),

  logout: () => apiFetch('/api/auth/logout', { method: 'POST' }),
};

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i] ?? 'B'}`;
}
