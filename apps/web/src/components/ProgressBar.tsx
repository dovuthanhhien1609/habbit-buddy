import React from 'react'

interface Props {
  completed: number
  total: number
}

export function ProgressBar({ completed, total }: Props) {
  const pct = total === 0 ? 0 : Math.round((completed / total) * 100)

  return (
    <div className="space-y-1">
      <div className="flex justify-between text-sm text-slate-500">
        <span>{completed} of {total} done</span>
        <span>{pct}%</span>
      </div>
      <div className="h-2 bg-slate-200 rounded-full overflow-hidden">
        <div
          className="h-full bg-gradient-to-r from-brand-500 to-indigo-400 rounded-full transition-all duration-500"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}
