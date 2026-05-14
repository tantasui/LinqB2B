import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Toaster } from 'sonner'
import { AuthProvider } from './context/AuthContext'
import { ProtectedRoute } from './components/ProtectedRoute'
import { Login } from './pages/Login'
import { Register } from './pages/Register'
import Layout from './components/Layout'
import Home from './pages/dashboard/Home'
import Deposits from './pages/dashboard/Deposits'
import Settlements from './pages/dashboard/Settlements'
import PaymentLinks from './pages/dashboard/PaymentLinks'
import Settings from './pages/dashboard/Settings'
import PaymentPage from './pages/PaymentPage'

function App() {
  return (
    <AuthProvider>
      <Toaster position="top-right" richColors />
      <BrowserRouter>
        <Routes>
          <Route path="/pay/:merchantId" element={<PaymentPage />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <Layout><Home /></Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/dashboard/deposits"
            element={
              <ProtectedRoute>
                <Layout><Deposits /></Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/dashboard/settlements"
            element={
              <ProtectedRoute>
                <Layout><Settlements /></Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/dashboard/payment-links"
            element={
              <ProtectedRoute>
                <Layout><PaymentLinks /></Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/dashboard/settings"
            element={
              <ProtectedRoute>
                <Layout><Settings /></Layout>
              </ProtectedRoute>
            }
          />
          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}

export default App
