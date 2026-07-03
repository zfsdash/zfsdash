'use client';

import { Logo } from '@/components/Logo';
import { ArrowRight, Shield, Zap, Server } from 'lucide-react';

interface Props {
  onNext: () => void;
}

export function Step1Welcome({ onNext }: Props) {
  return (
    <div className="flex flex-col items-center text-center gap-8">
      <Logo size="lg" />

      <div className="space-y-3">
        <h1 className="text-4xl font-bold text-white">
          Welcome to ZFSdash
        </h1>
        <p className="text-lg text-gray-400 max-w-md">
          Let&apos;s get you set up in 2 minutes. Monitor your ZFS pools, get
          alerts, and manage snapshots — all from one place.
        </p>
      </div>

      {/* Feature highlights */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 w-full max-w-xl mt-2">
        {[
          { icon: <Zap className="w-5 h-5" />, title: 'Real-time health', desc: 'Pool status, scrub progress, and SMART data' },
          { icon: <Server className="w-5 h-5" />, title: 'Multi-host', desc: 'Local, SSH remote, and TrueNAS hosts' },
          { icon: <Shield className="w-5 h-5" />, title: 'Alerting', desc: 'Get notified before disaster strikes' },
        ].map((f) => (
          <div
            key={f.title}
            className="bg-gray-900 border border-gray-700 rounded-xl p-4 flex flex-col gap-2 text-left"
          >
            <div className="text-blue-400">{f.icon}</div>
            <p className="text-sm font-semibold text-white">{f.title}</p>
            <p className="text-xs text-gray-500">{f.desc}</p>
          </div>
        ))}
      </div>

      <button
        onClick={onNext}
        className="flex items-center gap-2 bg-blue-600 hover:bg-blue-500 text-white font-semibold px-8 py-3 rounded-xl transition-colors text-base"
      >
        Get Started
        <ArrowRight className="w-5 h-5" />
      </button>
    </div>
  );
}
