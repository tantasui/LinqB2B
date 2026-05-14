import { useState, useEffect, useRef } from 'react'
import { useParams } from 'react-router-dom'
import './PaymentPage.css'

// ── Types ────────────────────────────────────────────────────────────────────

interface MerchantInfo {
  id: string
  name: string
  suiAddress: string
}

interface OrderResponse {
  pending_order_id: string
  amount_usdc: number
  merchant_address: string
  exchange_rate: number
  expires_at: string
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function formatNGN(amount: number): string {
  return new Intl.NumberFormat('en-NG', {
    style: 'currency',
    currency: 'NGN',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(amount)
}

function formatUSDC(amount: number): string {
  return amount.toFixed(6)
}

function getTimeRemaining(expiresAt: string) {
  const total = new Date(expiresAt).getTime() - Date.now()
  if (total <= 0) return { total: 0, minutes: 0, seconds: 0 }
  const minutes = Math.floor((total / 1000 / 60) % 60)
  const seconds = Math.floor((total / 1000) % 60)
  return { total, minutes, seconds }
}

// ── Component ────────────────────────────────────────────────────────────────

export default function PaymentPage() {
  const { merchantId } = useParams<{ merchantId: string }>()

  // State
  const [merchant, setMerchant] = useState<MerchantInfo | null>(null)
  const [loadingMerchant, setLoadingMerchant] = useState(true)
  const [merchantError, setMerchantError] = useState('')

  const [amountNGN, setAmountNGN] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  const [order, setOrder] = useState<OrderResponse | null>(null)
  const [copied, setCopied] = useState(false)
  const [timeLeft, setTimeLeft] = useState({ total: 0, minutes: 0, seconds: 0 })

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // ── Load merchant info ─────────────────────────────────────────────────
  useEffect(() => {
    if (!merchantId) return

    setLoadingMerchant(true)
    fetch(`/api/merchants/${merchantId}`)
      .then((res) => {
        if (!res.ok) throw new Error('Merchant not found')
        return res.json()
      })
      .then((data: MerchantInfo) => {
        setMerchant(data)
        setMerchantError('')
      })
      .catch((err) => setMerchantError(err.message))
      .finally(() => setLoadingMerchant(false))
  }, [merchantId])

  // ── Countdown timer ────────────────────────────────────────────────────
  useEffect(() => {
    if (!order) return

    const tick = () => setTimeLeft(getTimeRemaining(order.expires_at))
    tick()
    timerRef.current = setInterval(tick, 1000)

    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [order])

  // ── Handlers ───────────────────────────────────────────────────────────
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitError('')

    const amount = parseFloat(amountNGN.replace(/,/g, ''))
    if (isNaN(amount) || amount <= 0) {
      setSubmitError('Please enter a valid amount greater than 0')
      return
    }
    if (amount > 10_000_000) {
      setSubmitError('Amount cannot exceed ₦10,000,000')
      return
    }

    setSubmitting(true)
    try {
      const res = await fetch(`/api/merchants/${merchantId}/orders`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ amount_ngn: amount }),
      })

      if (res.status === 503) {
        setSubmitError('Exchange rate service is temporarily unavailable. Please try again in a moment.')
        return
      }
      if (!res.ok) {
        const text = await res.text()
        setSubmitError(text || 'Something went wrong. Please try again.')
        return
      }

      const data: OrderResponse = await res.json()
      setOrder(data)
    } catch {
      setSubmitError('Network error. Please check your connection and try again.')
    } finally {
      setSubmitting(false)
    }
  }

  const handleCopy = async () => {
    if (!order) return
    try {
      await navigator.clipboard.writeText(order.merchant_address)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Fallback for older browsers
      const textarea = document.createElement('textarea')
      textarea.value = order.merchant_address
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  // ── Render: Loading ────────────────────────────────────────────────────
  if (loadingMerchant) {
    return (
      <div className="pay-container">
        <div className="pay-card">
          <div className="skeleton" style={{ height: 24, width: '50%', marginBottom: 16 }} />
          <div className="skeleton" style={{ height: 40, marginBottom: 12 }} />
          <div className="skeleton" style={{ height: 20, width: '70%' }} />
        </div>
      </div>
    )
  }

  // ── Render: Error ──────────────────────────────────────────────────────
  if (merchantError) {
    return (
      <div className="pay-container">
        <div className="pay-card animate-fade-in">
          <div className="pay-error-icon">⚠️</div>
          <h2 className="pay-title">Merchant Not Found</h2>
          <p className="pay-subtitle">This payment link may be invalid or expired.</p>
        </div>
      </div>
    )
  }

  // ── Render: Payment Instructions ───────────────────────────────────────
  if (order) {
    const expired = timeLeft.total <= 0

    return (
      <div className="pay-container">
        <div className="pay-card animate-fade-in">
          <div className="pay-merchant-badge">{merchant?.name}</div>

          {expired ? (
            <>
              <div className="pay-error-icon">⏰</div>
              <h2 className="pay-title">Payment Expired</h2>
              <p className="pay-subtitle">This payment request has expired. Please create a new one.</p>
              <button
                className="pay-button"
                onClick={() => { setOrder(null); setAmountNGN('') }}
              >
                Start Over
              </button>
            </>
          ) : (
            <>
              <h2 className="pay-title">Send USDC to Complete Payment</h2>

              <div className="pay-amount-box">
                <span className="pay-amount-label">Send exactly</span>
                <span className="pay-amount-value">{formatUSDC(order.amount_usdc)} USDC</span>
              </div>

              <div className="pay-address-section">
                <span className="pay-address-label">To this address</span>
                <div className="pay-address-box" onClick={handleCopy}>
                  <span className="pay-address-text">{order.merchant_address}</span>
                  <span className={`pay-copy-btn ${copied ? 'copied' : ''}`}>
                    {copied ? '✓ Copied' : 'Copy'}
                  </span>
                </div>
              </div>

              <div className="pay-info-grid">
                <div className="pay-info-item">
                  <span className="pay-info-label">Exchange Rate</span>
                  <span className="pay-info-value">₦{order.exchange_rate.toLocaleString()} / USDC</span>
                </div>
                <div className="pay-info-item">
                  <span className="pay-info-label">Time Remaining</span>
                  <span className={`pay-info-value ${timeLeft.minutes < 5 ? 'text-danger' : 'text-success'}`}>
                    {timeLeft.minutes}m {timeLeft.seconds.toString().padStart(2, '0')}s
                  </span>
                </div>
              </div>

              <p className="pay-disclaimer">
                Send the exact amount shown above. Sending a different amount may result in a failed payment.
              </p>
            </>
          )}
        </div>
      </div>
    )
  }

  // ── Render: Amount Entry Form ──────────────────────────────────────────
  return (
    <div className="pay-container">
      <div className="pay-card animate-fade-in">
        <div className="pay-merchant-badge">{merchant?.name}</div>
        <h2 className="pay-title">Make a Payment</h2>
        <p className="pay-subtitle">Enter the amount you'd like to pay in Naira</p>

        <form onSubmit={handleSubmit} className="pay-form">
          <div className="pay-input-wrapper">
            <span className="pay-input-prefix">₦</span>
            <input
              type="text"
              inputMode="numeric"
              placeholder="0"
              value={amountNGN}
              onChange={(e) => {
                const v = e.target.value.replace(/[^0-9.]/g, '')
                setAmountNGN(v)
              }}
              className="pay-input"
              autoFocus
              disabled={submitting}
            />
          </div>

          {amountNGN && parseFloat(amountNGN) > 0 && (
            <p className="pay-amount-hint">
              {formatNGN(parseFloat(amountNGN.replace(/,/g, '')))}
            </p>
          )}

          {submitError && <p className="pay-error">{submitError}</p>}

          <button
            type="submit"
            className={`pay-button ${submitting ? 'disabled' : ''}`}
            disabled={submitting}
          >
            {submitting ? (
              <span className="pay-button-loading">
                <span className="pay-spinner" />
                Processing...
              </span>
            ) : (
              'Continue'
            )}
          </button>
        </form>
      </div>
    </div>
  )
}
