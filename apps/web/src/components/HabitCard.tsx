import React, { useState } from 'react'
import { Check, Flame, RotateCcw, Loader2, Bell } from 'lucide-react'
import { habits as habitApi, Habit } from '../api/client'
import { useStore } from '../store/useStore'
import { ReminderSettings } from './ReminderSettings'

interface Props {
  habit: Habit
}

const ICONS: Record<string, string> = {
  check: '✓',
  running: '🏃',
  book: '📚',
  water: '💧',
  meditation: '🧘',
  gym: '💪',
  sleep: '😴',
  food: '🥗',
  code: '💻',
  music: '🎵',
  heart: '❤️',
  star: '⭐',
}

export function HabitCard({ habit }: Props) {
  const [loading, setLoading] = useState(false)
  const [showReminders, setShowReminders] = useState(false)
  const updateHabit = useStore((s) => s.updateHabit)
  const addToast = useStore((s) => s.addToast)

  const toggle = async () => {
    if (loading) return
    setLoading(true)

    // Optimistic update
    updateHabit(habit.id, { completedToday: !habit.completedToday })

    try {
      if (!habit.completedToday) {
        const res = await habitApi.complete(habit.id)
        updateHabit(habit.id, {
          completedToday: true,
          streak: res.data.streak,
        })
        if (res.data.milestone) {
          addToast({ type: 'success', message: `🎉 ${res.data.milestone}` })
        }
      } else {
        const res = await habitApi.undo(habit.id)
        updateHabit(habit.id, {
          completedToday: false,
          streak: res.data.streak,
        })
      }
    } catch {
      // Revert optimistic update on error
      updateHabit(habit.id, { completedToday: habit.completedToday })
      addToast({ type: 'error', message: 'Could not update habit. Try again.' })
    } finally {
      setLoading(false)
    }
  }

  const icon = ICONS[habit.icon] ?? '✓'
  const isCompleted = habit.completedToday

  return (
    <div
      className={`
        group relative flex items-center gap-4 rounded-2xl bg-white p-4 shadow-sm
        border-l-4 transition-all duration-200
        ${isCompleted ? 'opacity-80' : 'hover:shadow-md hover:-translate-y-0.5'}
      `}
      style={{ borderLeftColor: habit.color }}
    >
      {/* Icon */}
      <div
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-xl"
        style={{ backgroundColor: habit.color + '20' }}
      >
        {icon}
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <p className={`font-semibold text-slate-800 truncate ${isCompleted ? 'line-through text-slate-400' : ''}`}>
          {habit.name}
        </p>
        {habit.description && (
          <p className="text-xs text-slate-400 truncate">{habit.description}</p>
        )}
        {habit.streak > 0 && (
          <div className="mt-1 flex items-center gap-1">
            <Flame size={12} className="text-orange-400" />
            <span className="text-xs font-medium text-orange-500">
              {habit.streak} {habit.streak === 1 ? 'day' : 'days'}
            </span>
          </div>
        )}
      </div>

      {/* Reminder bell */}
      <button
        onClick={() => setShowReminders(true)}
        className="shrink-0 text-slate-300 hover:text-brand-400 transition-colors"
        title="Set reminder"
      >
        <Bell size={15} />
      </button>

      {/* Action button */}
      <button
        onClick={toggle}
        disabled={loading}
        className={`
          shrink-0 flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium
          transition-all duration-200 active:scale-95
          ${isCompleted
            ? 'bg-emerald-50 text-emerald-600 hover:bg-emerald-100'
            : 'bg-slate-100 text-slate-600 hover:bg-brand-50 hover:text-brand-600'
          }
        `}
      >
        {loading ? (
          <Loader2 size={14} className="animate-spin" />
        ) : isCompleted ? (
          <>
            <Check size={14} />
            <span className="hidden sm:inline">Done</span>
          </>
        ) : (
          <>
            <RotateCcw size={14} className="opacity-0 group-hover:opacity-100 transition-opacity" />
            <span className="hidden sm:inline">Mark done</span>
          </>
        )}
      </button>

      {showReminders && (
        <ReminderSettings habitId={habit.id} onClose={() => setShowReminders(false)} />
      )}
    </div>
  )
}
