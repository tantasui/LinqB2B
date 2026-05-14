import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import { ProtectedRoute } from './components/ProtectedRoute'
import { Login } from './pages/Login'
import { Register } from './pages/Register'
import Layout from './components/Layout'
import Home from './pages/Home'
import Deposits from './pages/Deposits'
import Settlements from './pages/Settlements'
import PaymentLinks from './pages/PaymentLinks'
import Settings from './pages/Settings'

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <Layout><Home /></Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/deposits"
            element={
              <ProtectedRoute>
                <Layout><Deposits /></Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/settlements"
            element={
              <ProtectedRoute>
                <Layout><Settlements /></Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/payment-links"
            element={
              <ProtectedRoute>
                <Layout><PaymentLinks /></Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/settings"
            element={
              <ProtectedRoute>
                <Layout><Settings /></Layout>
              </ProtectedRoute>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}

export default App
