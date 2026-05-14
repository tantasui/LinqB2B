import React, { useEffect, useState } from 'react'
import DataTable from '../../components/DataTable'
import { TableSkeleton } from '../../components/Skeleton'
import { Plus, Copy, Trash2, X } from 'lucide-react'
import { toast } from 'sonner'
import { useAuth } from '../../context/AuthContext'
import client from '../../api/client'

interface PaymentLink {
  id: string
  merchantId: string
  amountNgn: number
  url: string
  createdAt: string
  status: string
}

function fmtDate(iso: string) {
  const d = new Date(iso)
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }) +
    ', ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
}

const PaymentLinks: React.FC = () => {
  const { user } = useAuth()
  const [links, setLinks] = useState<PaymentLink[]>([])
  const [loading, setLoading] = useState(true)
  const [showForm, setShowForm] = useState(false)
  const [amountNgn, setAmountNgn] = useState('')
  const [creating, setCreating] = useState(false)

  const fetchLinks = () => {
    if (!user?.id) return
    client.get<PaymentLink[]>(`/merchants/${user.id}/payment-links`)
      .then(res => setLinks(res.data || []))
      .catch(console.error)
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchLinks() }, [user?.id])

  const handleCreate = async () => {
    const amount = parseFloat(amountNgn)
    if (!amount || amount <= 0) {
      toast.error('Enter a valid NGN amount')
      return
    }
    setCreating(true)
    try {
      await client.post(`/merchants/${user?.id}/payment-links`, { amount_ngn: amount })
      toast.success('Payment link created')
      setAmountNgn('')
      setShowForm(false)
      fetchLinks()
    } catch {
      toast.error('Failed to create payment link')
    } finally {
      setCreating(false)
    }
  }

  const handleCopy = (url: string) => {
    navigator.clipboard.writeText(url)
    toast.success('Link copied!')
  }

  const handleDelete = async (id: string) => {
    try {
      await client.delete(`/merchants/${user?.id}/payment-links/${id}`)
      toast.success('Link deleted')
      setLinks(prev => prev.filter(l => l.id !== id))
    } catch {
      toast.error('Failed to delete link')
    }
  }

  const columns = [
    {
      header: 'Amount (NGN)',
      accessor: 'amountNgn',
      render: (row: PaymentLink) => (
        <span className="text-lg font-bold">₦ {row.amountNgn.toLocaleString('en-NG', { minimumFractionDigits: 2 })}</span>
      )
    },
    {
      header: 'Link',
      accessor: 'url',
      render: (row: PaymentLink) => (
        <span className="text-primary underline underline-offset-4 font-medium text-sm break-all">{row.url}</span>
      )
    },
    {
      header: 'QR Code',
      accessor: 'qr',
      render: (row: PaymentLink) => (
        <img
          src={`https://api.qrserver.com/v1/create-qr-code/?data=${encodeURIComponent(row.url)}&size=80x80`}
          alt="QR"
          className="w-10 h-10 rounded border border-outline-variant"
        />
      )
    },
    {
      header: 'Created At',
      accessor: 'createdAt',
      render: (row: PaymentLink) => <span className="text-on-surface-variant text-sm">{fmtDate(row.createdAt)}</span>
    },
    {
      header: 'Status',
      accessor: 'status',
      render: (row: PaymentLink) => (
        <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold ${
          row.status === 'active' ? 'bg-primary-container text-on-primary-container' : 'bg-surface-dim text-on-surface-variant border border-outline-variant'
        }`}>
          {row.status}
        </span>
      )
    },
    {
      header: '',
      accessor: 'actions',
      render: (row: PaymentLink) => (
        <div className="flex items-center justify-end gap-2">
          <button
            className="p-2 text-on-surface-variant hover:bg-surface-container rounded-full transition-colors"
            title="Copy Link"
            onClick={() => handleCopy(row.url)}
          >
            <Copy size={18} />
          </button>
          <button
            className="p-2 text-error hover:bg-error-container hover:text-on-error-container rounded-full transition-colors"
            title="Delete"
            onClick={() => handleDelete(row.id)}
          >
            <Trash2 size={18} />
          </button>
        </div>
      )
    }
  ]

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold text-on-surface">Payment Links</h1>
          <p className="text-on-surface-variant mt-2">Manage, track, and generate new payment links to share with customers.</p>
        </div>
        <button
          className="inline-flex items-center justify-center gap-2 bg-primary text-on-primary font-bold px-6 py-3 rounded-full hover:bg-primary-container hover:text-on-primary-container transition-all shadow-lg active:scale-95"
          onClick={() => setShowForm(true)}
        >
          <Plus size={18} />
          Generate Payment Link
        </button>
      </div>

      {showForm && (
        <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 shadow-sm flex flex-col sm:flex-row items-end gap-4">
          <div className="flex-1">
            <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2">
              Amount (NGN)
            </label>
            <input
              type="number"
              min="1"
              placeholder="e.g. 150000"
              value={amountNgn}
              onChange={e => setAmountNgn(e.target.value)}
              className="w-full bg-surface-container-low border border-outline-variant/20 rounded-lg px-4 py-3 text-on-surface focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
              onKeyDown={e => e.key === 'Enter' && handleCreate()}
            />
          </div>
          <div className="flex gap-3">
            <button
              onClick={handleCreate}
              disabled={creating}
              className="bg-primary text-on-primary font-bold px-6 py-3 rounded-lg hover:bg-primary/90 transition-all active:scale-95 disabled:opacity-50"
            >
              {creating ? 'Creating…' : 'Create'}
            </button>
            <button
              onClick={() => { setShowForm(false); setAmountNgn('') }}
              className="p-3 rounded-lg border border-outline-variant hover:bg-surface-container transition-colors"
            >
              <X size={18} />
            </button>
          </div>
        </div>
      )}

      <div className="bg-surface/60 backdrop-blur-xl border border-outline-variant rounded-xl shadow-lg overflow-hidden">
        {loading ? (
          <div className="p-6"><TableSkeleton /></div>
        ) : links.length === 0 ? (
          <div className="p-12 text-center text-on-surface-variant">No payment links yet. Generate your first one!</div>
        ) : (
          <>
            <DataTable columns={columns} data={links} />
            <div className="px-6 py-4 border-t border-outline-variant bg-surface-container-lowest/30 flex items-center justify-between">
              <span className="text-sm text-on-surface-variant">Showing {links.length} link{links.length !== 1 ? 's' : ''}</span>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

export default PaymentLinks
