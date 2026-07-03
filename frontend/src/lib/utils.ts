export function cn(...classes: (string | undefined | false | null)[]): string {
  return classes.filter(Boolean).join(' ');
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i] ?? 'B'}`;
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function capacityColor(pct: number): string {
  if (pct >= 85) return 'bg-red-500';
  if (pct >= 70) return 'bg-yellow-400';
  return 'bg-green-400';
}

export function healthColor(health: string): string {
  switch (health) {
    case 'ONLINE': return 'text-green-400';
    case 'DEGRADED': return 'text-yellow-400';
    default: return 'text-red-500';
  }
}

export function healthBg(health: string): string {
  switch (health) {
    case 'ONLINE': return 'bg-green-400/10 text-green-400 ring-green-400/20';
    case 'DEGRADED': return 'bg-yellow-400/10 text-yellow-400 ring-yellow-400/20';
    default: return 'bg-red-500/10 text-red-500 ring-red-500/20';
  }
}
