import { useEffect, useRef } from 'react'
import { useStore } from '../store/useStore'
import { Notification } from '../types/notification'

interface HabitEventPayload {
  habitId: string
  habitName?: string
  streak: number
  completedAt?: string
}

interface ReminderEventPayload {
  notificationId: string
  userId: string
  habitId: string
  title: string
  body: string
}

interface WSEvent {
  type: string
  payload: HabitEventPayload | ReminderEventPayload
}

export function useWebSocket() {
  const token = useStore((s) => s.token)
  const updateHabit = useStore((s) => s.updateHabit)
  const addToast = useStore((s) => s.addToast)
  const addNotification = useStore((s) => s.addNotification)
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const retryDelay = useRef(1000)

  useEffect(() => {
    if (!token) return

    const connect = () => {
      const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
      const host = window.location.host
      const url = `${protocol}://${host}/ws?token=${token}`

      const ws = new WebSocket(url)
      wsRef.current = ws

      ws.onopen = () => {
        retryDelay.current = 1000 // reset backoff
      }

      ws.onmessage = (event) => {
        try {
          const msg: WSEvent = JSON.parse(event.data)
          handleEvent(msg)
        } catch {
          // ignore malformed messages
        }
      }

      ws.onclose = () => {
        // Reconnect with exponential backoff (max 30s)
        retryRef.current = setTimeout(() => {
          retryDelay.current = Math.min(retryDelay.current * 2, 30000)
          connect()
        }, retryDelay.current)
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    const handleEvent = (msg: WSEvent) => {
      if (msg.type === 'HABIT_COMPLETED') {
        const p = msg.payload as HabitEventPayload
        updateHabit(p.habitId, {
          completedToday: true,
          streak: p.streak,
        })
        addToast({
          type: 'success',
          message: p.habitName
            ? `${p.habitName} completed! 🔥 ${p.streak} day streak`
            : 'Habit completed!',
          streak: p.streak,
        })
      }

      if (msg.type === 'HABIT_UNDONE') {
        const p = msg.payload as HabitEventPayload
        updateHabit(p.habitId, {
          completedToday: false,
          streak: p.streak,
        })
      }

      if (msg.type === 'reminder' || msg.type === 'HABIT_REMINDER') {
        const p = msg.payload as ReminderEventPayload
        // Only show toast when the tab is visible
        if (document.visibilityState === 'visible') {
          addToast({
            type: 'info',
            message: p.body ? `${p.title}: ${p.body}` : p.title,
          })
        }
        // Always add to notification store so the bell reflects it
        const notification: Notification = {
          id: p.notificationId,
          userId: p.userId,
          type: 'reminder',
          title: p.title,
          body: p.body ?? '',
          read: false,
          createdAt: new Date().toISOString(),
        }
        addNotification(notification)
      }
    }

    connect()

    return () => {
      if (retryRef.current) clearTimeout(retryRef.current)
      wsRef.current?.close()
    }
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps
}
