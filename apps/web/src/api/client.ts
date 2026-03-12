import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
})

// Attach JWT to every request
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('hb_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// On 401, clear auth and redirect to login
api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('hb_token')
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

export default api

// ---- typed endpoint helpers ----

export interface User {
  id: string
  email: string
  username: string
  createdAt: string
}

export interface Habit {
  id: string
  userId: string
  name: string
  description: string
  color: string
  icon: string
  isActive: boolean
  createdAt: string
  streak: number
  completedToday: boolean
}

export interface DashboardResponse {
  date: string
  completedCount: number
  totalCount: number
  completionRate: number
  habits: Habit[]
}

export interface HabitStats {
  habitId: string
  habitName: string
  streak: number
  longestStreak: number
  totalCompleted: number
  rate30Day: number
  history: string[]
}

export const auth = {
  register: (data: { email: string; username: string; password: string }) =>
    api.post<{ token: string; user: User }>('/auth/register', data),

  login: (data: { email: string; password: string }) =>
    api.post<{ token: string; user: User }>('/auth/login', data),
}

export const habits = {
  getDashboard: () => api.get<DashboardResponse>('/dashboard'),

  list: () => api.get<{ habits: Habit[] }>('/habits'),

  create: (data: { name: string; description?: string; color?: string; icon?: string }) =>
    api.post<Habit>('/habits', data),

  update: (id: string, data: Partial<Pick<Habit, 'name' | 'description' | 'color' | 'icon'>>) =>
    api.patch<Habit>(`/habits/${id}`, data),

  archive: (id: string) => api.delete(`/habits/${id}`),

  complete: (id: string) =>
    api.post<{ habitId: string; streak: number; completedToday: boolean; milestone: string }>(
      `/habits/${id}/complete`
    ),

  undo: (id: string) =>
    api.delete<{ habitId: string; streak: number; completedToday: boolean }>(
      `/habits/${id}/complete`
    ),

  getStats: (id: string) => api.get<HabitStats>(`/habits/${id}/stats`),

  getAnalytics: () =>
    api.get<{ habits: (Habit & { history: string[] })[] }>('/analytics'),
}
