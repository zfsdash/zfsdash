'use client';

import { useState } from 'react';
import { api } from '@/lib/api';
import { Monitor, Globe, Database, Key, ChevronRight } from 'lucide-react';

interface Props {
  onNext: (hostName?: string) => void;
}

type Mode = 'local' | 'ssh' | 'truenas';
type AuthMethod = 'password' | 'key';

const MODE_CARDS = [
  {
    mode: 'local' as Mode,
    icon: <Monitor className="w-6 h-6" />,
    title: 'Local',
    desc: 'This machine. No credentials needed.',
  },
  {
    mode: 'ssh' as Mode,
    icon: <Globe className="w-6 h-6" />,
    title: 'SSH Remote',
    desc: 'Connect to a remote Linux/FreeBSD server.',
  },
  {
    mode: 'truenas' as Mode,
    icon: <Database className="w-6 h-6" />,
    title: 'TrueNAS',
    desc: 'Connect via the TrueNAS REST API.',
  },
];

export function Step3Host({ onNext }: Props) {
  const [mode, setMode] = useState<Mode | null>(null);
  const [name, setName] = useState('');
  const [hostname, setHostname] = useState('');
  const [port, setPort] = useState('22');
  const [username, setUsername] = useState('root');
  const [authMethod, setAuthMethod] = useState<AuthMethod>('password');
  const [password, setPassword] = useState('');
  const [privateKey, setPrivateKey] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      await api.addHost({
        name,
        mode: mode!,
        ...(mode === 'ssh' && {
          hostname,
          port: parseInt(port, 10),
          username,
          ...(authMethod === 'password' ? { password } : { private_key: privateKey }),
        }),
        ...(mode === 'truenas' && { hostname, api_key: apiKey }),
      });
      onNext(name);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add host');
    } finally {
      setLoading(false);
    }
  }

  const inputCls =
    'w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-2.5 text-white placeholder-gray-600 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition';
  const labelCls = 'block text-sm font-medium text-gray-300 mb-1.5';

  return (
    <div className="w-full max-w-lg mx-auto space-y-6">
      <div className="text-center space-y-1">
        <h2 className="text-2xl font-bold text-white">Add Your First Host</h2>
        <p className="text-gray-400 text-sm">Choose how ZFSdash should connect to your storage.</p>
      </div>

      {/* Mode selector */}
      <div className="grid grid-cols-3 gap-3">
        {MODE_CARDS.map((card) => (
          <button
            key={card.mode}
            type="button"
            onClick={() => setMode(card.mode)}
            className={`flex flex-col gap-2 p-4 rounded-xl border text-left transition-all ${
              mode === card.mode
                ? 'border-blue-500 bg-blue-500/10 ring-1 ring-blue-500'
                : 'border-gray-700 bg-gray-900 hover:border-gray-600'
            }`}
          >
            <div className={mode === card.mode ? 'text-blue-400' : 'text-gray-500'}>
              {card.icon}
            </div>
            <p className="text-sm font-semibold text-white">{card.title}</p>
            <p className="text-xs text-gray-500">{card.desc}</p>
          </button>
        ))}
      </div>

      {mode && (
        <form onSubmit={submit} className="space-y-4 bg-gray-900 border border-gray-700 rounded-xl p-5">
          {/* Name */}
          <div>
            <label className={labelCls}>Host Label</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder={mode === 'local' ? 'My NAS' : 'nas-01'}
              className={inputCls}
            />
          </div>

          {/* SSH fields */}
          {mode === 'ssh' && (
            <>
              <div className="grid grid-cols-3 gap-3">
                <div className="col-span-2">
                  <label className={labelCls}>Hostname / IP</label>
                  <input
                    type="text"
                    value={hostname}
                    onChange={(e) => setHostname(e.target.value)}
                    required
                    placeholder="192.168.1.100"
                    className={inputCls}
                  />
                </div>
                <div>
                  <label className={labelCls}>Port</label>
                  <input
                    type="number"
                    value={port}
                    onChange={(e) => setPort(e.target.value)}
                    required
                    className={inputCls}
                  />
                </div>
              </div>

              <div>
                <label className={labelCls}>SSH Username</label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  className={inputCls}
                />
              </div>

              {/* Auth method toggle */}
              <div>
                <label className={labelCls}>Authentication</label>
                <div className="flex rounded-lg border border-gray-700 overflow-hidden">
                  {(['password', 'key'] as AuthMethod[]).map((m) => (
                    <button
                      key={m}
                      type="button"
                      onClick={() => setAuthMethod(m)}
                      className={`flex-1 py-2 text-sm font-medium transition-colors ${
                        authMethod === m
                          ? 'bg-blue-600 text-white'
                          : 'bg-gray-800 text-gray-400 hover:text-white'
                      }`}
                    >
                      {m === 'password' ? 'Password' : 'Private Key'}
                    </button>
                  ))}
                </div>
              </div>

              {authMethod === 'password' ? (
                <div>
                  <label className={labelCls}>Password</label>
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className={inputCls}
                  />
                </div>
              ) : (
                <div>
                  <label className={labelCls}>
                    <Key className="inline w-3.5 h-3.5 mr-1" />
                    Private Key (PEM)
                  </label>
                  <textarea
                    value={privateKey}
                    onChange={(e) => setPrivateKey(e.target.value)}
                    rows={5}
                    placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                    className={`${inputCls} font-mono text-xs resize-y`}
                  />
                </div>
              )}
            </>
          )}

          {/* TrueNAS fields */}
          {mode === 'truenas' && (
            <>
              <div>
                <label className={labelCls}>TrueNAS Hostname / IP</label>
                <input
                  type="text"
                  value={hostname}
                  onChange={(e) => setHostname(e.target.value)}
                  required
                  placeholder="truenas.local or 192.168.1.50"
                  className={inputCls}
                />
              </div>
              <div>
                <label className={labelCls}>API Key</label>
                <input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  required
                  placeholder="TrueNAS API key"
                  className={inputCls}
                />
                <p className="text-xs text-gray-500 mt-1">
                  Generate in TrueNAS → Credentials → API Keys
                </p>
              </div>
            </>
          )}

          {error && (
            <div className="bg-red-500/10 border border-red-500/30 rounded-lg px-4 py-3 text-red-400 text-sm">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full flex items-center justify-center gap-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white font-semibold py-2.5 rounded-xl transition-colors"
          >
            {loading ? 'Connecting…' : 'Add Host'}
            {!loading && <ChevronRight className="w-4 h-4" />}
          </button>
        </form>
      )}

      <div className="text-center">
        <button
          type="button"
          onClick={() => onNext()}
          className="text-sm text-gray-500 hover:text-gray-300 transition-colors underline underline-offset-4"
        >
          Skip for now
        </button>
      </div>
    </div>
  );
}
