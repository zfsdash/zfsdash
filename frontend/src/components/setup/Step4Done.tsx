'use client';

import { CheckCircle2, ExternalLink, Server, User } from 'lucide-react';
import { useRouter } from 'next/navigation';

interface Props {
  username: string;
  hostName?: string;
}

export function Step4Done({ username, hostName }: Props) {
  const router = useRouter();

  const configured = [
    { icon: <User className="w-4 h-4" />, label: 'Admin account', value: username },
    ...(hostName
      ? [{ icon: <Server className="w-4 h-4" />, label: 'First host', value: hostName }]
      : []),
  ];

  return (
    <div className="flex flex-col items-center text-center gap-8 max-w-md mx-auto">
      {/* Success icon */}
      <div className="relative">
        <div className="w-24 h-24 rounded-full bg-green-400/10 flex items-center justify-center">
          <CheckCircle2 className="w-14 h-14 text-green-400" />
        </div>
        <div className="absolute -inset-2 rounded-full bg-green-400/5 animate-ping" />
      </div>

      <div className="space-y-2">
        <h2 className="text-3xl font-bold text-white">You&apos;re all set!</h2>
        <p className="text-gray-400">
          ZFSdash is ready. Start monitoring your pools and get notified when
          things go wrong.
        </p>
      </div>

      {/* Summary */}
      <div className="w-full bg-gray-900 border border-gray-700 rounded-xl p-5 space-y-3">
        <p className="text-xs uppercase tracking-widest text-gray-600 font-semibold">Configured</p>
        {configured.map((item) => (
          <div key={item.label} className="flex items-center justify-between gap-4">
            <span className="flex items-center gap-2 text-sm text-gray-400">
              <span className="text-green-400">{item.icon}</span>
              {item.label}
            </span>
            <span className="text-sm font-medium text-white">{item.value}</span>
          </div>
        ))}
        {!hostName && (
          <p className="text-xs text-gray-600 pt-1">
            No host configured — you can add one from the dashboard.
          </p>
        )}
      </div>

      <button
        onClick={() => router.push('/')}
        className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold px-8 py-3 rounded-xl transition-colors"
      >
        Open Dashboard
        <ExternalLink className="w-4 h-4" />
      </button>
    </div>
  );
}
