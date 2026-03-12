import { create } from 'zustand'
import { Habit, User } from '../api/client'

export interface Toast {
  id: string
  message: string
  type: 'success' | 'info' | 'error'
  streak?: number
}

interface Store {
  // Auth
  token: string | null
  user: User | null

  // Habits
  habits: Habit[]
  dashboardDate: string

  // UI
  toasts: Toast[]

  // Actions
  setToken: (token: string) => void
  setUser: (user: User) => void
  setHabits: (habits: Habit[]) => void
  updateHabit: (habitId: string, updates: Partial<Habit>) => void
  removeHabit: (habitId: string) => void
  addToast: (toast: Omit<Toast, 'id'>) => void
  removeToast: (id: string) => void
  logout: () => void
}

export const useStore = create<Store>((set) => ({
  token: localStorage.getItem('hb_token'),
  user: (() => {
    try {
      const raw = localStorage.getItem('hb_user')
      return raw ? JSON.parse(raw) : null
    } catch {
      return null
    }
  })(),
  habits: [],
  dashboardDate: new Date().toISOString().slice(0, 10),
  toasts: [],

  setToken: (token) => {
    localStorage.setItem('hb_token', token)
    set({ token })
  },

  setUser: (user) => {
    localStorage.setItem('hb_user', JSON.stringify(user))
    set({ user })
  },

  setHabits: (habits) => set({ habits }),

  updateHabit: (habitId, updates) =>
    set((state) => ({
      habits: state.habits.map((h) =>
        h.id === habitId ? { ...h, ...updates } : h
      ),
    })),

  removeHabit: (habitId) =>
    set((state) => ({
      habits: state.habits.filter((h) => h.id !== habitId),
    })),

  addToast: (toast) => {
    const id = Math.random().toString(36).slice(2)
    set((state) => ({ toasts: [...state.toasts, { ...toast, id }] }))
    // Auto-remove after 4 seconds
    setTimeout(() => {
      set((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) }))
    }, 4000)
  },

  removeToast: (id) =>
    set((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) })),

  logout: () => {
    localStorage.removeItem('hb_token')
    localStorage.removeItem('hb_user')
    set({ token: null, user: null, habits: [] })
  },
}))
