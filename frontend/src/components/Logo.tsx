import { cn } from '@/lib/utils';

export function Logo({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  const dims = { sm: 32, md: 48, lg: 72 }[size];
  const textSize = { sm: 'text-base', md: 'text-xl', lg: 'text-3xl' }[size];

  return (
    <div className="flex items-center gap-3">
      <svg
        width={dims}
        height={dims}
        viewBox="0 0 48 48"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        {/* Outer hexagon */}
        <polygon
          points="24,2 44,13 44,35 24,46 4,35 4,13"
          fill="#1d4ed8"
          stroke="#3b82f6"
          strokeWidth="1.5"
        />
        {/* Z letter */}
        <text
          x="24"
          y="32"
          textAnchor="middle"
          fontSize="22"
          fontWeight="700"
          fill="white"
          fontFamily="-apple-system, BlinkMacSystemFont, monospace"
        >
          Z
        </text>
        {/* Small data stripes */}
        <line x1="8" y1="22" x2="14" y2="22" stroke="#93c5fd" strokeWidth="1.5" strokeLinecap="round" />
        <line x1="8" y1="26" x2="12" y2="26" stroke="#93c5fd" strokeWidth="1.5" strokeLinecap="round" />
        <line x1="34" y1="22" x2="40" y2="22" stroke="#93c5fd" strokeWidth="1.5" strokeLinecap="round" />
        <line x1="36" y1="26" x2="40" y2="26" stroke="#93c5fd" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
      <span className={cn('font-bold tracking-tight text-white', textSize)}>
        ZFS<span className="text-blue-400">dash</span>
      </span>
    </div>
  );
}
