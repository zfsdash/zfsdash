'use client';

import { useState } from 'react';
import { Logo } from '@/components/Logo';
import { Step1Welcome } from '@/components/setup/Step1Welcome';
import { Step2Account } from '@/components/setup/Step2Account';
import { Step3Host } from '@/components/setup/Step3Host';
import { Step4Done } from '@/components/setup/Step4Done';
import { cn } from '@/lib/utils';

const STEPS = [
  { label: 'Welcome' },
  { label: 'Admin Account' },
  { label: 'Add Host' },
  { label: 'Done' },
];

export default function SetupPage() {
  const [step, setStep] = useState(0);
  const [username, setUsername] = useState('admin');
  const [hostName, setHostName] = useState<string | undefined>(undefined);

  function handleAccountCreated() {
    // Capture username from form state — in real usage this is echoed from API
    setStep(2);
  }

  function handleHostAdded(name?: string) {
    setHostName(name);
    setStep(3);
  }

  return (
    <div className="min-h-screen bg-gray-950 flex flex-col">
      {/* Top bar */}
      <header className="border-b border-gray-800 px-6 py-4 flex items-center justify-between">
        <Logo size="sm" />
        <span className="text-xs text-gray-600 font-mono">Setup Wizard</span>
      </header>

      {/* Step progress */}
      <div className="max-w-2xl mx-auto w-full px-6 pt-8">
        <div className="flex items-center gap-0">
          {STEPS.map((s, i) => (
            <div key={s.label} className="flex items-center flex-1 last:flex-none">
              {/* Circle */}
              <div className="flex flex-col items-center gap-1">
                <div
                  className={cn(
                    'w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-colors',
                    i < step
                      ? 'bg-green-500 text-white'
                      : i === step
                      ? 'bg-blue-600 text-white ring-2 ring-blue-400 ring-offset-2 ring-offset-gray-950'
                      : 'bg-gray-800 text-gray-600'
                  )}
                >
                  {i < step ? '✓' : i + 1}
                </div>
                <span
                  className={cn(
                    'text-xs whitespace-nowrap',
                    i === step ? 'text-white' : i < step ? 'text-green-400' : 'text-gray-600'
                  )}
                >
                  {s.label}
                </span>
              </div>
              {/* Connector line */}
              {i < STEPS.length - 1 && (
                <div
                  className={cn(
                    'flex-1 h-px mx-2 mb-5 transition-colors',
                    i < step ? 'bg-green-500' : 'bg-gray-800'
                  )}
                />
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Step content */}
      <main className="flex-1 flex items-start justify-center px-6 py-12">
        <div className="w-full max-w-2xl">
          {step === 0 && <Step1Welcome onNext={() => setStep(1)} />}
          {step === 1 && (
            <Step2Account
              onNext={() => {
                handleAccountCreated();
              }}
            />
          )}
          {step === 2 && <Step3Host onNext={handleHostAdded} />}
          {step === 3 && <Step4Done username={username} hostName={hostName} />}
        </div>
      </main>
    </div>
  );
}
