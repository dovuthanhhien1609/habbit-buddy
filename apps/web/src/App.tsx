import React from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useStore } from './store/useStore'
import { useWebSocket } from './hooks/useWebSocket'
import { Layout } from './components/Layout'
import { ToastContainer } from './components/Toast'
import { Dashboard } from './pages/Dashboard'
import { Analytics } from './pages/Analytics'
import { Login } from './pages/Login'
import { Register } from './pages/Register'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const token = useStore((s) => s.token)
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

function AppContent() {
  // Connect WebSocket once authenticated
  useWebSocket()

  return (
    <>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />

        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Layout>
                <Dashboard />
              </Layout>
            </ProtectedRoute>
          }
        />
        <Route
          path="/analytics"
          element={
            <ProtectedRoute>
              <Layout>
                <Analytics />
              </Layout>
            </ProtectedRoute>
          }
        />

        {/* Catch-all */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>

      <ToastContainer />
    </>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AppContent />
    </BrowserRouter>
  )
}
