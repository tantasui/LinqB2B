import { useState, useEffect, useRef } from 'react'
import { useParams } from 'react-router-dom'
import client from '../api/client'
import './PaymentPage.css'

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

interface OrderStatus {
  status: 'pending' | 'received' | 'expired'
  deposit_status: string | null
}

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

export default function PaymentPage() {
  const { merchantId } = useParams<{ merchantId: string }>()

  const [merchant, setMerchant] = useState<MerchantInfo | null>(null)
  const [loadingMerchant, setLoadingMerchant] = useState(true)
  const [merchantError, setMerchantError] = useState('')

  const [amountNGN, setAmountNGN] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  const [order, setOrder] = useState<OrderResponse | null>(null)
  const [orderStatus, setOrderStatus] = useState<OrderStatus | null>(null)
  const [copied, setCopied] = useState(false)
  const [timeLeft, setTimeLeft] = useState({ total: 0, minutes: 0, seconds: 0 })

  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    if (!merchantId) return

    setLoadingMerchant(true)
    client.get<MerchantInfo>(`/merchants/${merchantId}`)
      .then((res) => {
        setMerchant(res.data)
        setMerchantError('')
      })
      .catch(() => setMerchantError('Merchant not found'))
      .finally(() => setLoadingMerchant(false))
  }, [merchantId])

  useEffect(() => {
    if (!order) return

    const tick = () => setTimeLeft(getTimeRemaining(order.expires_at))
    tick()
    timerRef.current = setInterval(tick, 1000)

    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [order])

  // Poll order status every 5s until payment received or expired
  useEffect(() => {
    if (!order) return

    const poll = async () => {
      try {
        const res = await client.get<OrderStatus>(`/orders/${order.pending_order_id}/status`)
        const data = res.data
        setOrderStatus(data)
        if (data.status === 'received') {
          if (pollRef.current) clearInterval(pollRef.current)
        }
      } catch {
        // silently ignore poll errors
      }
    }

    poll()
    pollRef.current = setInterval(poll, 5000)

    return () => {
      if (pollRef.current) clearInterval(pollRef.current)
    }
  }, [order])

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
      const res = await client.post<OrderResponse>(`/merchants/${merchantId}/orders`, { amount_ngn: amount })
      const data = res.data
      setOrder(data)
    } catch (err: any) {
      if (err.response?.status === 503) {
        setSubmitError('Exchange rate service is temporarily unavailable. Please try again in a moment.')
      } else {
        setSubmitError(err.response?.data || 'Something went wrong. Please try again.')
      }
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

  if (order && orderStatus?.status === 'received') {
    return (
      <div className="pay-container">
        <div className="pay-card animate-fade-in">
          <div className="pay-merchant-badge">{merchant?.name}</div>
          <div className="pay-error-icon">✅</div>
          <h2 className="pay-title">Payment Received!</h2>
          <p className="pay-subtitle">
            Your payment of {formatUSDC(order.amount_usdc)} USDC has been confirmed.
            The merchant will process your order shortly.
          </p>
        </div>
      </div>
    )
  }

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
