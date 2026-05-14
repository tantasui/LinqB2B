import { BrowserRouter, Routes, Route } from 'react-router-dom'
import PaymentPage from './pages/PaymentPage'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/pay/:merchantId" element={<PaymentPage />} />
        <Route path="*" element={
          <div style={{
            minHeight: '100vh',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: '#0f172a',
            color: '#94a3b8',
            fontFamily: '"Plus Jakarta Sans", system-ui, sans-serif',
          }}>
            <p>Payment page not found. Use /pay/:merchantId</p>
          </div>
        } />
      </Routes>
    </BrowserRouter>
  )
}

export default App
