'use client';

import { Database, Plus } from 'lucide-react';

interface Props {
  type: 'no-hosts' | 'no-pools';
  hostId?: string;
}

export function EmptyState({ type, hostId }: Props) {
  if (type === 'no-hosts') {
    return (
      <div className="flex flex-col items-center justify-center py-24 gap-6 text-center">
        <div className="w-20 h-20 rounded-2xl bg-gray-900 border border-gray-800 flex items-center justify-center">
          <Database className="w-10 h-10 text-gray-700" />
        </div>
        <div className="space-y-2">
          <h2 className="text-xl font-bold text-white">No hosts configured</h2>
          <p className="text-gray-500 max-w-sm">
            Add a host to start monitoring your ZFS pools. You can connect to this
            machine, a remote server via SSH, or a TrueNAS instance.
          </p>
        </div>
        <a
          href="/setup"
          className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold px-6 py-2.5 rounded-xl transition-colors"
        >
          <Plus className="w-4 h-4" />
          Add your first host
        </a>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center justify-center py-16 gap-4 text-center">
      <div className="w-16 h-16 rounded-xl bg-gray-900 border border-gray-800 flex items-center justify-center">
        <Database className="w-8 h-8 text-gray-700" />
      </div>
      <div className="space-y-1.5">
        <h3 className="text-lg font-semibold text-white">No pools found</h3>
        <p className="text-sm text-gray-500">
          ZFSdash couldn&apos;t find any ZFS pools on this host. Make sure ZFS is
          installed and pools are imported.
        </p>
      </div>
    </div>
  );
}
