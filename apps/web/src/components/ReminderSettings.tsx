import React, { useEffect, useState } from 'react'
import { X, Trash2, Loader2 } from 'lucide-react'
import { reminders as remindersApi } from '../api/reminders'
import { Reminder, CreateReminderRequest } from '../types/reminder'
import { useStore } from '../store/useStore'

const DAY_LABELS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

interface Props {
  habitId: string
  onClose: () => void
}

function defaultForm(): CreateReminderRequest {
  return { remindAt: '08:00', daysOfWeek: [1, 2, 3, 4, 5], enabled: true }
}

export function ReminderSettings({ habitId, onClose }: Props) {
  const [reminderList, setReminderList] = useState<Reminder[]>([])
  const [loadingFetch, setLoadingFetch] = useState(true)
  const [saving, setSaving] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [error, setError] = useState('')

  // Form state — one form for adding / editing the first reminder
  const [form, setForm] = useState<CreateReminderRequest>(defaultForm())
  const [editingId, setEditingId] = useState<string | null>(null)

  const addToast = useStore((s) => s.addToast)

  useEffect(() => {
    setLoadingFetch(true)
    remindersApi.list(habitId)
      .then((res) => {
        const fetched = res.data.reminders ?? []
        setReminderList(fetched)
        if (fetched.length > 0) {
          // Pre-fill form with the first reminder for editing
          const first = fetched[0]
          setForm({ remindAt: first.remindAt, daysOfWeek: first.daysOfWeek, enabled: first.enabled })
          setEditingId(first.id)
        }
      })
      .catch(() => {})
      .finally(() => setLoadingFetch(false))
  }, [habitId])

  const toggleDay = (day: number) => {
    setForm((prev) => ({
      ...prev,
      daysOfWeek: prev.daysOfWeek.includes(day)
        ? prev.daysOfWeek.filter((d) => d !== day)
        : [...prev.daysOfWeek, day].sort((a, b) => a - b),
    }))
  }

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (form.daysOfWeek.length === 0) {
      setError('Select at least one day')
      return
    }
    setError('')
    setSaving(true)
    try {
      if (editingId) {
        const res = await remindersApi.update(habitId, editingId, form)
        setReminderList((prev) =>
          prev.map((r) => (r.id === editingId ? res.data : r))
        )
        addToast({ type: 'success', message: 'Reminder updated' })
      } else {
        const res = await remindersApi.create(habitId, form)
        setReminderList((prev) => [...prev, res.data])
        setEditingId(res.data.id)
        addToast({ type: 'success', message: 'Reminder set' })
      }
    } catch {
      setError('Could not save reminder. Try again.')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (reminderId: string) => {
    setDeletingId(reminderId)
    try {
      await remindersApi.remove(habitId, reminderId)
      setReminderList((prev) => prev.filter((r) => r.id !== reminderId))
      if (editingId === reminderId) {
        setEditingId(null)
        setForm(defaultForm())
      }
      addToast({ type: 'info', message: 'Reminder removed' })
    } catch {
      addToast({ type: 'error', message: 'Could not delete reminder.' })
    } finally {
      setDeletingId(null)
    }
  }

  const handleSelectReminder = (r: Reminder) => {
    setEditingId(r.id)
    setForm({ remindAt: r.remindAt, daysOfWeek: r.daysOfWeek, enabled: r.enabled })
    setError('')
  }

  const handleNewReminder = () => {
    setEditingId(null)
    setForm(defaultForm())
    setError('')
  }

  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/30 backdrop-blur-sm" onClick={onClose} />

      {/* Panel */}
      <div className="relative w-full max-w-md rounded-2xl bg-white shadow-2xl animate-bounce-in">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-slate-100 px-6 py-4">
          <h2 className="text-lg font-bold text-slate-800">Reminders</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-600 transition-colors">
            <X size={20} />
          </button>
        </div>

        <div className="p-6 space-y-5">
          {loadingFetch ? (
            <div className="flex justify-center py-8">
              <Loader2 className="animate-spin text-brand-500" size={24} />
            </div>
          ) : (
            <>
              {/* Existing reminders list */}
              {reminderList.length > 0 && (
                <div className="space-y-2">
                  <p className="text-xs font-medium text-slate-400 uppercase tracking-wide">
                    Existing reminders
                  </p>
                  {reminderList.map((r) => (
                    <div
                      key={r.id}
                      className={`flex items-center justify-between rounded-xl px-3 py-2.5 border transition-colors cursor-pointer ${
                        editingId === r.id
                          ? 'border-brand-400 bg-brand-50'
                          : 'border-slate-200 hover:border-slate-300 bg-slate-50'
                      }`}
                      onClick={() => handleSelectReminder(r)}
                    >
                      <div>
                        <p className="text-sm font-semibold text-slate-700">{r.remindAt}</p>
                        <p className="text-xs text-slate-400">
                          {r.daysOfWeek.map((d) => DAY_LABELS[d]).join(', ')}
                          {!r.enabled && ' · disabled'}
                        </p>
                      </div>
                      <button
                        onClick={(e) => { e.stopPropagation(); handleDelete(r.id) }}
                        disabled={deletingId === r.id}
                        className="ml-3 shrink-0 text-slate-300 hover:text-red-400 transition-colors"
                        title="Delete reminder"
                      >
                        {deletingId === r.id
                          ? <Loader2 size={14} className="animate-spin" />
                          : <Trash2 size={14} />
                        }
                      </button>
                    </div>
                  ))}
                  <button
                    type="button"
                    onClick={handleNewReminder}
                    className="text-xs text-brand-500 hover:text-brand-600 font-medium transition-colors"
                  >
                    + Add another reminder
                  </button>
                </div>
              )}

              {/* Form */}
              <form onSubmit={handleSave} className="space-y-4">
                <p className="text-xs font-medium text-slate-400 uppercase tracking-wide">
                  {editingId ? 'Edit reminder' : 'New reminder'}
                </p>

                {/* Time */}
                <div>
                  <label className="block text-sm font-medium text-slate-700 mb-1">Time</label>
                  <input
                    type="time"
                    value={form.remindAt}
                    onChange={(e) => setForm((prev) => ({ ...prev, remindAt: e.target.value }))}
                    className="w-full rounded-xl border border-slate-200 px-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  />
                </div>

                {/* Days of week */}
                <div>
                  <label className="block text-sm font-medium text-slate-700 mb-2">Days</label>
                  <div className="flex gap-2 flex-wrap">
                    {DAY_LABELS.map((label, idx) => (
                      <button
                        key={idx}
                        type="button"
                        onClick={() => toggleDay(idx)}
                        className={`h-9 w-9 rounded-xl text-xs font-semibold transition-all ${
                          form.daysOfWeek.includes(idx)
                            ? 'bg-brand-500 text-white shadow-sm'
                            : 'bg-slate-100 text-slate-500 hover:bg-slate-200'
                        }`}
                      >
                        {label}
                      </button>
                    ))}
                  </div>
                </div>

                {/* Enabled toggle */}
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-slate-700">Enabled</label>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={form.enabled}
                    onClick={() => setForm((prev) => ({ ...prev, enabled: !prev.enabled }))}
                    className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-1 ${
                      form.enabled ? 'bg-brand-500' : 'bg-slate-200'
                    }`}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                        form.enabled ? 'translate-x-6' : 'translate-x-1'
                      }`}
                    />
                  </button>
                </div>

                {error && <p className="text-sm text-red-500">{error}</p>}

                <div className="flex gap-3 pt-1">
                  <button
                    type="button"
                    onClick={onClose}
                    className="flex-1 rounded-xl border border-slate-200 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-50 transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={saving}
                    className="flex-1 rounded-xl bg-brand-500 py-2.5 text-sm font-medium text-white hover:bg-brand-600 disabled:opacity-60 transition-colors"
                  >
                    {saving ? 'Saving…' : editingId ? 'Update' : 'Save'}
                  </button>
                </div>
              </form>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
