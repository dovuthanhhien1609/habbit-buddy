import React, { useEffect, useState } from 'react'
import { Plus, Flame, Loader2 } from 'lucide-react'
import { format } from 'date-fns'
import { habits as habitApi, Habit } from '../api/client'
import { useStore } from '../store/useStore'
import { HabitCard } from '../components/HabitCard'
import { ProgressBar } from '../components/ProgressBar'
import { AddHabitModal } from '../components/AddHabitModal'

export function Dashboard() {
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const habits = useStore((s) => s.habits)
  const setHabits = useStore((s) => s.setHabits)
  const user = useStore((s) => s.user)

  useEffect(() => {
    habitApi.getDashboard()
      .then((res) => setHabits(res.data.habits))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const completedCount = habits.filter((h) => h.completedToday).length
  const longestStreak = habits.reduce((max, h) => Math.max(max, h.streak), 0)

  const handleHabitCreated = (habit: Habit) => {
    setHabits([...habits, habit])
    setShowModal(false)
  }

  const today = format(new Date(), 'EEEE, MMMM d')

  return (
    <>
      <div className="space-y-6">
        {/* Header */}
        <div>
          <p className="text-sm text-slate-400 font-medium">{today}</p>
          <h1 className="text-2xl font-bold text-slate-800 mt-0.5">
            Hey, {user?.username ?? 'there'} 👋
          </h1>
        </div>

        {/* Stats row */}
        {habits.length > 0 && (
          <div className="grid grid-cols-2 gap-3">
            <div className="rounded-2xl bg-white border border-slate-100 shadow-sm p-4">
              <p className="text-xs text-slate-400 font-medium uppercase tracking-wide">Today</p>
              <p className="text-3xl font-bold text-brand-600 mt-1">{completedCount}</p>
              <p className="text-sm text-slate-500">of {habits.length} done</p>
            </div>
            <div className="rounded-2xl bg-white border border-slate-100 shadow-sm p-4">
              <p className="text-xs text-slate-400 font-medium uppercase tracking-wide">Best streak</p>
              <div className="flex items-center gap-1.5 mt-1">
                <Flame className="text-orange-400" size={20} />
                <p className="text-3xl font-bold text-orange-500">{longestStreak}</p>
              </div>
              <p className="text-sm text-slate-500">days</p>
            </div>
          </div>
        )}

        {/* Progress bar */}
        {habits.length > 0 && (
          <ProgressBar completed={completedCount} total={habits.length} />
        )}

        {/* Habits section */}
        <div>
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-semibold text-slate-700">Today's Habits</h2>
            <button
              onClick={() => setShowModal(true)}
              className="flex items-center gap-1.5 rounded-xl bg-brand-500 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-600 transition-colors"
            >
              <Plus size={14} />
              Add
            </button>
          </div>

          {loading ? (
            <div className="flex justify-center py-12">
              <Loader2 className="animate-spin text-brand-500" size={28} />
            </div>
          ) : habits.length === 0 ? (
            <div className="rounded-2xl border-2 border-dashed border-slate-200 p-10 text-center">
              <p className="text-4xl mb-3">🌱</p>
              <p className="font-semibold text-slate-600">No habits yet</p>
              <p className="text-sm text-slate-400 mt-1">Add your first habit to get started</p>
              <button
                onClick={() => setShowModal(true)}
                className="mt-4 inline-flex items-center gap-2 rounded-xl bg-brand-500 px-4 py-2 text-sm font-medium text-white hover:bg-brand-600 transition-colors"
              >
                <Plus size={14} />
                Add a habit
              </button>
            </div>
          ) : (
            <div className="space-y-3">
              {/* Incomplete first */}
              {[...habits]
                .sort((a, b) => (a.completedToday ? 1 : 0) - (b.completedToday ? 1 : 0))
                .map((habit) => (
                  <HabitCard key={habit.id} habit={habit} />
                ))}
            </div>
          )}
        </div>

        {/* All done state */}
        {!loading && habits.length > 0 && completedCount === habits.length && (
          <div className="rounded-2xl bg-gradient-to-br from-emerald-50 to-brand-50 border border-emerald-100 p-6 text-center animate-fade-in">
            <p className="text-3xl mb-2">🎉</p>
            <p className="font-bold text-slate-700">All done for today!</p>
            <p className="text-sm text-slate-500 mt-1">Come back tomorrow to keep your streaks going.</p>
          </div>
        )}
      </div>

      {showModal && (
        <AddHabitModal onClose={() => setShowModal(false)} onCreated={handleHabitCreated} />
      )}
    </>
  )
}
