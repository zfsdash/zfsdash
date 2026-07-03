'use client';

import { type Pool } from '@/lib/api';
import { formatBytes } from '@/lib/utils';
import { Server, Database, AlertTriangle, HardDrive } from 'lucide-react';

interface Props {
  pools: Pool[];
}

export function QuickStats({ pools }: Props) {
  const totalSize = pools.reduce((s, p) => s + p.size, 0);
  const totalUsed = pools.reduce((s, p) => s + p.allocated, 0);
  const degraded = pools.filter((p) => p.health !== 'ONLINE').length;

  const stats = [
    {
      icon: <Database className="w-5 h-5" />,
      label: 'Total Pools',
      value: pools.length.toString(),
      color: 'text-blue-400',
      bg: 'bg-blue-400/10',
    },
    {
      icon: <HardDrive className="w-5 h-5" />,
      label: 'Total Capacity',
      value: formatBytes(totalSize),
      color: 'text-purple-400',
      bg: 'bg-purple-400/10',
    },
    {
      icon: <Server className="w-5 h-5" />,
      label: 'Space Used',
      value: formatBytes(totalUsed),
      color: 'text-cyan-400',
      bg: 'bg-cyan-400/10',
    },
    {
      icon: <AlertTriangle className="w-5 h-5" />,
      label: 'Degraded Pools',
      value: degraded.toString(),
      color: degraded > 0 ? 'text-red-400' : 'text-green-400',
      bg: degraded > 0 ? 'bg-red-400/10' : 'bg-green-400/10',
    },
  ];

  return (
    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
      {stats.map((s) => (
        <div
          key={s.label}
          className="bg-gray-900 border border-gray-800 rounded-xl p-4 flex items-center gap-3"
        >
          <div className={`${s.bg} ${s.color} p-2 rounded-lg`}>{s.icon}</div>
          <div>
            <p className="text-xs text-gray-500 font-medium">{s.label}</p>
            <p className="text-lg font-bold text-white leading-tight">{s.value}</p>
          </div>
        </div>
      ))}
    </div>
  );
}
