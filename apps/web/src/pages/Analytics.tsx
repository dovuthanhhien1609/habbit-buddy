import React, { useEffect, useState } from 'react'
import { Flame, TrendingUp, Loader2 } from 'lucide-react'
import { habits as habitApi, Habit } from '../api/client'

interface HabitWithHistory extends Habit {
  history: string[]
}

// Generate a 10-week heatmap grid of dates (most recent on right)
function buildHeatmapDates(): string[] {
  const dates: string[] = []
  const today = new Date()
  for (let i = 69; i >= 0; i--) {
    const d = new Date(today)
    d.setDate(today.getDate() - i)
    dates.push(d.toISOString().slice(0, 10))
  }
  return dates
}

function Heatmap({ completedDates }: { completedDates: string[] }) {
  const allDates = buildHeatmapDates()
  const completedSet = new Set(completedDates)
  const weeks: string[][] = []
  for (let i = 0; i < allDates.length; i += 7) {
    weeks.push(allDates.slice(i, i + 7))
  }

  return (
    <div className="flex gap-1">
      {weeks.map((week, wi) => (
        <div key={wi} className="flex flex-col gap-1">
          {week.map((date) => (
            <div
              key={date}
              title={date}
              className={`h-3 w-3 rounded-sm transition-colors ${
                completedSet.has(date) ? 'bg-brand-500' : 'bg-slate-100'
              }`}
            />
          ))}
        </div>
      ))}
    </div>
  )
}

export function Analytics() {
  const [loading, setLoading] = useState(true)
  const [habits, setHabits] = useState<HabitWithHistory[]>([])

  useEffect(() => {
    habitApi.getAnalytics()
      .then((res) => setHabits(
        (res.data.habits ?? []).map((h) => ({ ...h, history: h.history ?? [] })) as HabitWithHistory[]
      ))
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const overallRate =
    habits.length === 0
      ? 0
      : habits.reduce((sum, h) => sum + h.history.length / 30, 0) / habits.length

  const allHistory = habits.flatMap((h) => h.history)
  const uniqueHistory = [...new Set(allHistory)]

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-slate-800">Analytics</h1>
        <p className="text-sm text-slate-400 mt-0.5">Last 30 days</p>
      </div>

      {loading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="animate-spin text-brand-500" size={28} />
        </div>
      ) : habits.length === 0 ? (
        <div className="rounded-2xl border-2 border-dashed border-slate-200 p-10 text-center">
          <p className="text-4xl mb-3">📊</p>
          <p className="font-semibold text-slate-600">No data yet</p>
          <p className="text-sm text-slate-400 mt-1">Start completing habits to see analytics</p>
        </div>
      ) : (
        <>
          {/* Overall card */}
          <div className="rounded-2xl bg-white border border-slate-100 shadow-sm p-5">
            <div className="flex items-center gap-2 mb-3">
              <TrendingUp size={18} className="text-brand-500" />
              <span className="font-semibold text-slate-700">Overall completion rate</span>
            </div>
            <div className="flex items-end gap-3 mb-3">
              <span className="text-4xl font-bold text-brand-600">
                {Math.round(overallRate * 100)}%
              </span>
              <span className="text-sm text-slate-400 mb-1">avg across all habits</span>
            </div>
            <div className="h-2 bg-slate-100 rounded-full overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-brand-500 to-indigo-400 rounded-full transition-all"
                style={{ width: `${overallRate * 100}%` }}
              />
            </div>
          </div>

          {/* Contribution heatmap */}
          <div className="rounded-2xl bg-white border border-slate-100 shadow-sm p-5">
            <p className="font-semibold text-slate-700 mb-4">Contribution Heatmap</p>
            <div className="overflow-x-auto scrollbar-hide">
              <Heatmap completedDates={uniqueHistory} />
            </div>
            <div className="flex items-center gap-2 mt-3 text-xs text-slate-400">
              <span>Less</span>
              <div className="h-3 w-3 rounded-sm bg-slate-100" />
              <div className="h-3 w-3 rounded-sm bg-brand-200" />
              <div className="h-3 w-3 rounded-sm bg-brand-500" />
              <span>More</span>
            </div>
          </div>

          {/* Per-habit breakdown */}
          <div className="space-y-3">
            {habits
              .sort((a, b) => b.streak - a.streak)
              .map((habit) => {
                const rate = habit.history.length / 30
                return (
                  <div
                    key={habit.id}
                    className="rounded-2xl bg-white border border-slate-100 shadow-sm p-4"
                  >
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-3">
                        <div
                          className="h-8 w-8 rounded-lg flex items-center justify-center text-sm"
                          style={{ backgroundColor: habit.color + '20' }}
                        >
                          {habit.icon === 'running' ? '🏃' :
                           habit.icon === 'book' ? '📚' :
                           habit.icon === 'water' ? '💧' :
                           habit.icon === 'gym' ? '💪' : '✓'}
                        </div>
                        <span className="font-semibold text-slate-700 text-sm">{habit.name}</span>
                      </div>
                      <div className="flex items-center gap-1.5">
                        {habit.streak > 0 && (
                          <>
                            <Flame size={13} className="text-orange-400" />
                            <span className="text-sm font-medium text-orange-500">
                              {habit.streak}d
                            </span>
                          </>
                        )}
                        <span className="text-sm font-bold text-slate-700 ml-2">
                          {Math.round(rate * 100)}%
                        </span>
                      </div>
                    </div>
                    <div className="h-1.5 bg-slate-100 rounded-full overflow-hidden">
                      <div
                        className="h-full rounded-full transition-all"
                        style={{
                          width: `${rate * 100}%`,
                          backgroundColor: habit.color,
                        }}
                      />
                    </div>
                    <p className="text-xs text-slate-400 mt-1.5">
                      {habit.history.length} of 30 days completed
                    </p>
                  </div>
                )
              })}
          </div>
        </>
      )}
    </div>
  )
}
