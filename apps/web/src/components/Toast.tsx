import React from 'react'
import { X, CheckCircle, AlertCircle, Info } from 'lucide-react'
import { useStore } from '../store/useStore'

export function ToastContainer() {
  const toasts = useStore((s) => s.toasts)
  const removeToast = useStore((s) => s.removeToast)

  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 pointer-events-none">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className="pointer-events-auto animate-slide-in flex items-start gap-3 rounded-xl bg-white shadow-lg border border-slate-100 px-4 py-3 min-w-[280px] max-w-sm"
        >
          <span className="mt-0.5 shrink-0">
            {toast.type === 'success' && <CheckCircle className="text-emerald-500" size={18} />}
            {toast.type === 'error'   && <AlertCircle className="text-red-500" size={18} />}
            {toast.type === 'info'    && <Info className="text-brand-500" size={18} />}
          </span>
          <p className="text-sm text-slate-700 flex-1 leading-snug">{toast.message}</p>
          <button
            onClick={() => removeToast(toast.id)}
            className="shrink-0 text-slate-400 hover:text-slate-600 transition-colors"
          >
            <X size={14} />
          </button>
        </div>
      ))}
    </div>
  )
}
