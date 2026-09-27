import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Events } from '@wailsio/runtime'
import * as DDNS from '../bindings/github.com/ryansonshine/rytrix-ddns-client/desktop/ddns'
import type { Status } from '../bindings/github.com/ryansonshine/rytrix-ddns-client/desktop/models'

function LogoMark({ className = 'h-9 w-9' }: { className?: string }) {
  return (
    <svg viewBox="0 0 64 64" className={className} aria-hidden="true">
      <defs>
        <linearGradient id="logo-bg" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#6366f1" />
          <stop offset="1" stopColor="#06b6d4" />
        </linearGradient>
      </defs>
      <rect width="64" height="64" rx="16" fill="url(#logo-bg)" />
      <g fill="none" stroke="#fff" strokeLinecap="round">
        <circle cx="30" cy="34" r="17" strokeWidth="3.5" />
        <ellipse cx="30" cy="34" rx="7" ry="17" strokeWidth="3" />
        <path d="M13.5 34h33" strokeWidth="3" />
      </g>
      <circle cx="46" cy="18" r="7.5" fill="#4f46e5" />
      <circle cx="46" cy="18" r="5" fill="#fff" />
    </svg>
  )
}

const primary =
  'w-full rounded-xl bg-gradient-to-r from-indigo-500 to-cyan-500 px-4 py-2.5 text-sm font-semibold text-white shadow-lg shadow-indigo-900/40 transition hover:brightness-110 disabled:opacity-50'
const secondary =
  'flex-1 rounded-xl border border-ink-700 px-3 py-2 text-sm font-medium text-slate-200 transition hover:border-slate-500 disabled:opacity-50'
const input =
  'w-full rounded-xl border border-ink-700 bg-ink-950 px-3.5 py-2.5 font-mono text-sm text-slate-100 placeholder:text-slate-600 outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-500/30'

function message(e: unknown) {
  return e instanceof Error ? e.message : String(e)
}

function timeAgo(iso: string) {
  if (!iso) return 'never'
  const s = Math.round((Date.now() - new Date(iso).getTime()) / 1000)
  if (s < 60) return 'just now'
  if (s < 3600) return `${Math.floor(s / 60)} min ago`
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

function Setup({ status, onDone, onCancel }: { status: Status; onDone: (s: Status) => void; onCancel?: () => void }) {
  const [hostname, setHostname] = useState(status.hostname)
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      onDone(await DDNS.Save(hostname, token))
    } catch (err) {
      setError(message(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className="card space-y-4 p-5">
      <div>
        <h2 className="font-semibold">Connect a host</h2>
        <p className="mt-1 text-sm text-slate-400">
          Register a host on the{' '}
          <button type="button" onClick={() => DDNS.OpenDashboard()} className="text-indigo-300 hover:text-indigo-200">
            dashboard
          </button>
          , then paste its name and token.
        </p>
      </div>
      <label className="block">
        <span className="text-xs font-medium uppercase tracking-wider text-slate-500">Hostname</span>
        <div className="mt-1.5 flex items-center overflow-hidden rounded-xl border border-ink-700 bg-ink-950 focus-within:border-indigo-400 focus-within:ring-2 focus-within:ring-indigo-500/30">
          <input value={hostname} onChange={(e) => setHostname(e.target.value)} placeholder="home" required autoFocus className="min-w-0 flex-1 bg-transparent px-3.5 py-2.5 font-mono text-sm outline-none placeholder:text-slate-600" />
          <span className="pr-3.5 font-mono text-sm text-slate-500">.{status.domain}</span>
        </div>
      </label>
      <label className="block">
        <span className="text-xs font-medium uppercase tracking-wider text-slate-500">Token</span>
        <input type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder="paste the host's token" required className={`${input} mt-1.5`} />
      </label>
      {error && <p className="rounded-lg bg-rose-500/10 px-3 py-2 text-sm text-rose-200">{error}</p>}
      <button type="submit" disabled={busy} className={primary}>
        {busy ? 'Checking…' : 'Save and connect'}
      </button>
      {onCancel && (
        <button type="button" onClick={onCancel} className="w-full text-sm text-slate-400 hover:text-slate-200">
          Cancel
        </button>
      )}
    </form>
  )
}

function CopyButton() {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    if (await DDNS.CopyHostname()) {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    }
  }
  return (
    <button
      type="button"
      onClick={copy}
      title="Copy hostname"
      aria-label="Copy hostname"
      className={`flex shrink-0 items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-medium transition ${
        copied ? 'border-emerald-500/40 text-emerald-300' : 'border-ink-700 text-slate-300 hover:border-slate-500 hover:text-white'
      }`}
    >
      <svg viewBox="0 0 24 24" className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d={copied ? 'm5 12 5 5 9-10' : 'M9 9h11v11H9zM5 15H4V4h11v1'} />
      </svg>
      {copied ? 'Copied' : 'Copy'}
    </button>
  )
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between py-2.5">
      <span className="text-sm text-slate-400">{label}</span>
      <span className="text-sm text-slate-100">{children}</span>
    </div>
  )
}

function Connected({ status, onChangeHost }: { status: Status; onChangeHost: () => void }) {
  const [error, setError] = useState('')
  const state = status.busy ? 'busy' : status.error ? 'error' : 'ok'
  const dot = { ok: 'bg-emerald-400', busy: 'bg-amber-400 animate-pulse', error: 'bg-rose-400' }[state]
  const label = { ok: 'Up to date', busy: 'Checking…', error: 'Update failed' }[state]

  const act = async (fn: () => Promise<unknown>) => {
    setError('')
    try {
      await fn()
    } catch (err) {
      setError(message(err))
    }
  }

  return (
    <div className="space-y-4">
      <div className="card p-5">
        <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-slate-400">
          <span className={`h-2 w-2 rounded-full ${dot}`} /> {label}
        </div>
        <div className="mt-2 flex items-start justify-between gap-2">
          <p className="break-all font-mono text-lg text-cyan-200">
            {status.hostname}
            <span className="text-slate-500">.{status.domain}</span>
          </p>
          <CopyButton />
        </div>
        <div className="mt-3 divide-y divide-ink-800 border-t border-ink-800">
          <Row label="Points to">
            <span className="font-mono">{status.ip || '–'}</span>
          </Row>
          <Row label="Last checked">{timeAgo(status.lastChecked)}</Row>
        </div>
        {status.error && <p className="mt-3 rounded-lg bg-rose-500/10 px-3 py-2 text-sm text-rose-200">{status.error}</p>}
        <div className="mt-4 flex gap-2">
          <button type="button" disabled={status.busy} onClick={() => act(() => DDNS.UpdateNow())} className={secondary}>
            Update now
          </button>
          <button type="button" onClick={() => DDNS.OpenDashboard()} className={secondary}>
            Dashboard
          </button>
        </div>
      </div>

      <div className="card divide-y divide-ink-800 px-5">
        <label className="flex cursor-pointer items-center justify-between py-3.5">
          <span className="text-sm">Start at login</span>
          <input
            type="checkbox"
            checked={status.startAtLogin}
            onChange={(e) => act(() => DDNS.SetStartAtLogin(e.target.checked))}
            className="h-4 w-4 accent-indigo-500"
          />
        </label>
        <div className="flex items-center justify-between py-3.5">
          <button type="button" onClick={onChangeHost} className="text-sm text-indigo-300 hover:text-indigo-200">
            Change host
          </button>
          <button
            type="button"
            onClick={() => confirm('Remove this host from the app? It stays registered on the server.') && act(() => DDNS.Forget())}
            className="text-sm text-rose-300 hover:text-rose-200"
          >
            Disconnect
          </button>
        </div>
      </div>
      {error && <p className="rounded-lg bg-rose-500/10 px-3 py-2 text-sm text-rose-200">{error}</p>}
    </div>
  )
}

export default function App() {
  const [status, setStatus] = useState<Status | null>(null)
  const [editing, setEditing] = useState(false)
  const [, tick] = useState(0)

  useEffect(() => {
    DDNS.GetStatus().then(setStatus)
    const off = Events.On('status', (ev) => setStatus(ev.data as Status))
    const timer = setInterval(() => tick((n) => n + 1), 30_000)
    return () => {
      off()
      clearInterval(timer)
    }
  }, [])

  return (
    <div className="relative flex h-full flex-col overflow-hidden">
      <div aria-hidden="true" className="pointer-events-none absolute -top-32 left-1/2 -z-10 h-72 w-[36rem] -translate-x-1/2 rounded-full bg-gradient-to-r from-indigo-600/30 via-sky-500/15 to-cyan-400/25 blur-3xl" />
      <header className="flex items-center gap-3 px-6 pb-4 pt-10" style={{ '--wails-draggable': 'drag' } as React.CSSProperties}>
        <LogoMark />
        <div>
          <h1 className="text-base font-semibold leading-tight">
            rytrix <span className="bg-gradient-to-r from-indigo-400 to-cyan-400 bg-clip-text text-transparent">DDNS</span>
          </h1>
          <p className="text-xs text-slate-500">Keeps your hostname pointed at this computer</p>
        </div>
      </header>

      <main className="flex-1 overflow-y-auto px-6 pb-6">
        {!status ? null : !status.configured || editing ? (
          <Setup
            status={status}
            onDone={(s) => {
              setStatus(s)
              setEditing(false)
            }}
            onCancel={status.configured ? () => setEditing(false) : undefined}
          />
        ) : (
          <Connected status={status} onChangeHost={() => setEditing(true)} />
        )}
      </main>

      <footer className="px-6 pb-4 text-center text-xs text-slate-600">
        {status && `v${status.version} · runs in the ${navigator.platform.startsWith('Mac') ? 'menu bar' : 'system tray'}`}
      </footer>
    </div>
  )
}
