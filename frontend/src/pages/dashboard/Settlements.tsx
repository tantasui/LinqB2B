import React, { useEffect, useState } from 'react'
import DataTable from '../../components/DataTable'
import { TableSkeleton } from '../../components/Skeleton'
import { Filter, CheckCircle, Landmark, PieChart } from 'lucide-react'
import { useAuth } from '../../context/AuthContext'
import client from '../../api/client'

interface Settlement {
  id: string
  createdAt: string
  merchantId: string
  depositId: string
  amountUsdc: number
  amountNgn: number
  exchangeRate: number
  bankReference: string
  nombaReference: string
  status: string
}

function fmtDate(iso: string) {
  const d = new Date(iso)
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) +
    ', ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
}

const columns = [
  {
    header: 'Date (UTC)',
    accessor: 'createdAt',
    render: (row: Settlement) => <span className="text-sm">{fmtDate(row.createdAt)}</span>
  },
  {
    header: 'USDC Amount',
    accessor: 'amountUsdc',
    render: (row: Settlement) => (
      <span className="font-bold">{row.amountUsdc.toLocaleString('en-US', { minimumFractionDigits: 2 })}</span>
    )
  },
  {
    header: 'NGN Disbursed',
    accessor: 'amountNgn',
    render: (row: Settlement) => (
      <span className="text-on-surface-variant">
        ₦{row.amountNgn.toLocaleString('en-NG', { minimumFractionDigits: 2 })}
      </span>
    )
  },
  {
    header: 'Bank Ref',
    accessor: 'bankReference',
    render: (row: Settlement) => (
      <span className="font-mono text-xs text-outline">{row.bankReference || '—'}</span>
    )
  },
  {
    header: 'Status',
    accessor: 'status',
    render: (row: Settlement) => (
      <div className="text-right">
        <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold ${
          row.status === 'completed' ? 'bg-emerald-100 text-emerald-800' :
          row.status === 'failed' ? 'bg-red-100 text-red-800' : 'bg-amber-100 text-amber-800'
        }`}>
          {row.status.charAt(0).toUpperCase() + row.status.slice(1)}
        </span>
      </div>
    )
  }
]

const Settlements: React.FC = () => {
  const { user } = useAuth()
  const [settlements, setSettlements] = useState<Settlement[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!user?.id) return
    client.get<Settlement[]>(`/merchants/${user.id}/settlements`)
      .then(res => setSettlements(res.data || []))
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [user?.id])

  const totalUsdc = settlements.reduce((s, r) => s + r.amountUsdc, 0)
  const totalNgn = settlements.filter(r => r.status === 'completed').reduce((s, r) => s + r.amountNgn, 0)
  const completedCount = settlements.filter(r => r.status === 'completed').length
  const successRate = settlements.length === 0 ? 100 : Math.round((completedCount / settlements.length) * 1000) / 10

  return (
    <div className="flex flex-col gap-8">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 glass-shadow relative overflow-hidden group">
          <div className="flex justify-between items-start mb-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-on-surface-variant">Total Volume Settled</span>
            <div className="w-10 h-10 rounded-lg bg-surface-container-high flex items-center justify-center text-primary">
              <PieChart size={20} />
            </div>
          </div>
          <h3 className="text-2xl font-bold text-on-surface mb-1">
            {loading ? '—' : totalUsdc.toLocaleString('en-US', { minimumFractionDigits: 2 })}{' '}
            <span className="text-lg text-outline font-medium">USDC</span>
          </h3>
        </div>

        <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 glass-shadow relative overflow-hidden group">
          <div className="flex justify-between items-start mb-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-on-surface-variant">Total NGN Disbursed</span>
            <div className="w-10 h-10 rounded-lg bg-surface-container-high flex items-center justify-center text-secondary">
              <Landmark size={20} />
            </div>
          </div>
          <h3 className="text-2xl font-bold text-on-surface mb-1">
            {loading ? '—' : totalNgn >= 1_000_000
              ? `₦${(totalNgn / 1_000_000).toFixed(1)}M`
              : `₦${totalNgn.toLocaleString('en-NG')}`}
          </h3>
        </div>

        <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 glass-shadow relative overflow-hidden group">
          <div className="flex justify-between items-start mb-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-on-surface-variant">Success Rate</span>
            <div className="w-10 h-10 rounded-lg bg-surface-container-high flex items-center justify-center text-tertiary">
              <CheckCircle size={20} />
            </div>
          </div>
          <h3 className="text-2xl font-bold text-on-surface mb-1">
            {loading ? '—' : `${successRate}%`}
          </h3>
        </div>
      </div>

      <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-2xl glass-shadow overflow-hidden">
        <div className="p-6 border-b border-outline-variant/30 flex justify-between items-center">
          <h2 className="text-xl font-bold text-on-surface">Settlements</h2>
          <button className="flex items-center gap-2 text-sm font-bold text-primary hover:bg-primary-container/10 px-3 py-1.5 rounded-lg transition-colors">
            <Filter size={16} /> Filter
          </button>
        </div>
        {loading ? (
          <div className="p-6"><TableSkeleton /></div>
        ) : settlements.length === 0 ? (
          <div className="p-12 text-center text-on-surface-variant">No settlements yet</div>
        ) : (
          <DataTable columns={columns} data={settlements} onRowClick={() => {}} />
        )}
      </div>
    </div>
  )
}

export default Settlements
