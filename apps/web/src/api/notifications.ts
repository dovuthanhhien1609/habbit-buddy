import api from './client'
import { Notification } from '../types/notification'

export const notifications = {
  list: () =>
    api.get<{ notifications: Notification[] }>('/notifications'),

  markRead: (id: string) =>
    api.post<Notification>(`/notifications/${id}/read`),
}
