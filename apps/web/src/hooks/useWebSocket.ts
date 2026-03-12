import { useEffect, useRef } from 'react'
import { useStore } from '../store/useStore'

interface WSEvent {
  type: string
  payload: {
    habitId: string
    habitName?: string
    streak: number
    completedAt?: string
  }
}

export function useWebSocket() {
  const token = useStore((s) => s.token)
  const updateHabit = useStore((s) => s.updateHabit)
  const addToast = useStore((s) => s.addToast)
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
        updateHabit(msg.payload.habitId, {
          completedToday: true,
          streak: msg.payload.streak,
        })
        addToast({
          type: 'success',
          message: msg.payload.habitName
            ? `${msg.payload.habitName} completed! 🔥 ${msg.payload.streak} day streak`
            : 'Habit completed!',
          streak: msg.payload.streak,
        })
      }

      if (msg.type === 'HABIT_UNDONE') {
        updateHabit(msg.payload.habitId, {
          completedToday: false,
          streak: msg.payload.streak,
        })
      }
    }

    connect()

    return () => {
      if (retryRef.current) clearTimeout(retryRef.current)
      wsRef.current?.close()
    }
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps
}
