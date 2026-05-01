import api from './client'
import { Reminder, CreateReminderRequest } from '../types/reminder'

export const reminders = {
  list: (habitId: string) =>
    api.get<{ reminders: Reminder[] }>(`/habits/${habitId}/reminders`),

  create: (habitId: string, data: CreateReminderRequest) =>
    api.post<Reminder>(`/habits/${habitId}/reminders`, data),

  update: (habitId: string, reminderId: string, data: Partial<CreateReminderRequest>) =>
    api.put<Reminder>(`/habits/${habitId}/reminders/${reminderId}`, data),

  remove: (habitId: string, reminderId: string) =>
    api.delete(`/habits/${habitId}/reminders/${reminderId}`),
}
