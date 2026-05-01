export interface Reminder {
  id: string
  habitId: string
  userId: string
  remindAt: string // "HH:MM" time string
  daysOfWeek: number[] // 0=Sun..6=Sat
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface CreateReminderRequest {
  remindAt: string
  daysOfWeek: number[]
  enabled: boolean
}
