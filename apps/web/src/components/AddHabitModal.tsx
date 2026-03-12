import React, { useState } from 'react'
import { X } from 'lucide-react'
import { habits as habitApi, Habit } from '../api/client'
import { useStore } from '../store/useStore'

const COLORS = [
  '#6366f1', '#8b5cf6', '#ec4899', '#ef4444',
  '#f97316', '#eab308', '#10b981', '#06b6d4',
]

const ICONS = [
  { key: 'check', label: '✓' },
  { key: 'running', label: '🏃' },
  { key: 'book', label: '📚' },
  { key: 'water', label: '💧' },
  { key: 'meditation', label: '🧘' },
  { key: 'gym', label: '💪' },
  { key: 'sleep', label: '😴' },
  { key: 'food', label: '🥗' },
  { key: 'code', label: '💻' },
  { key: 'music', label: '🎵' },
]

interface Props {
  onClose: () => void
  onCreated: (habit: Habit) => void
}

export function AddHabitModal({ onClose, onCreated }: Props) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [color, setColor] = useState(COLORS[0])
  const [icon, setIcon] = useState('check')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const addToast = useStore((s) => s.addToast)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError('Name is required'); return }

    setLoading(true)
    setError('')
    try {
      const res = await habitApi.create({ name: name.trim(), description, color, icon })
      addToast({ type: 'success', message: `"${res.data.name}" added!` })
      onCreated(res.data)
    } catch {
      setError('Could not create habit. Try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/30 backdrop-blur-sm" onClick={onClose} />

      {/* Modal */}
      <div className="relative w-full max-w-md rounded-2xl bg-white shadow-2xl animate-bounce-in">
        <div className="flex items-center justify-between border-b border-slate-100 px-6 py-4">
          <h2 className="text-lg font-bold text-slate-800">New Habit</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-600">
            <X size={20} />
          </button>
        </div>

        <form onSubmit={submit} className="p-6 space-y-5">
          {/* Name */}
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Name *</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Morning run, Read 20 pages…"
              className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              autoFocus
            />
          </div>

          {/* Description */}
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Description</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional goal or notes"
              className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
            />
          </div>

          {/* Color */}
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-2">Color</label>
            <div className="flex gap-2 flex-wrap">
              {COLORS.map((c) => (
                <button
                  key={c}
                  type="button"
                  onClick={() => setColor(c)}
                  className={`h-8 w-8 rounded-full transition-transform ${color === c ? 'scale-125 ring-2 ring-offset-1 ring-slate-400' : 'hover:scale-110'}`}
                  style={{ backgroundColor: c }}
                />
              ))}
            </div>
          </div>

          {/* Icon */}
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-2">Icon</label>
            <div className="flex gap-2 flex-wrap">
              {ICONS.map((ic) => (
                <button
                  key={ic.key}
                  type="button"
                  onClick={() => setIcon(ic.key)}
                  className={`h-9 w-9 rounded-xl text-lg transition-all ${icon === ic.key ? 'ring-2 ring-brand-500 bg-brand-50' : 'bg-slate-100 hover:bg-slate-200'}`}
                >
                  {ic.label}
                </button>
              ))}
            </div>
          </div>

          {error && <p className="text-sm text-red-500">{error}</p>}

          <div className="flex gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 rounded-xl border border-slate-200 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="flex-1 rounded-xl bg-brand-500 py-2.5 text-sm font-medium text-white hover:bg-brand-600 disabled:opacity-60 transition-colors"
            >
              {loading ? 'Creating…' : 'Create Habit'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
