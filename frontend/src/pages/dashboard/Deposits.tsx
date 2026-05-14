import React, { useEffect, useState } from 'react'
import DataTable from '../../components/DataTable'
import { TableSkeleton } from '../../components/Skeleton'
import { ChevronDown, ArrowDownLeft } from 'lucide-react'
import { useAuth } from '../../context/AuthContext'
import client from '../../api/client'

interface Deposit {
  id: string
  createdAt: string
  txHash: string
  amountUsdc: number
  status: string
}

const statusColors: Record<string, string> = {
  completed: 'bg-emerald-50 text-emerald-700 border-emerald-200',
  pending_order_validation: 'bg-amber-50 text-amber-700 border-amber-200',
  swept: 'bg-blue-50 text-blue-700 border-blue-200',
  refunded: 'bg-slate-100 text-slate-600 border-slate-200',
  amount_validated: 'bg-purple-50 text-purple-700 border-purple-200',
  failed: 'bg-red-50 text-red-700 border-red-200',
}

function fmtDate(iso: string) {
  const d = new Date(iso)
  return {
    date: d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }),
    time: d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) + ' UTC',
  }
}

const columns = [
  {
    header: 'Date',
    accessor: 'createdAt',
    render: (row: Deposit) => {
      const { date, time } = fmtDate(row.createdAt)
      return (
        <div>
          <div className="font-bold">{date}</div>
          <div className="text-on-surface-variant text-xs">{time}</div>
        </div>
      )
    }
  },
  {
    header: 'USDC Amount',
    accessor: 'amountUsdc',
    render: (row: Deposit) => (
      <div className="flex items-center gap-2">
        <div className="w-6 h-6 rounded-full bg-surface-container flex items-center justify-center">
          <ArrowDownLeft size={12} className="text-surface-tint" />
        </div>
        <span className="font-bold text-base">{row.amountUsdc.toLocaleString('en-US', { minimumFractionDigits: 2 })}</span>
      </div>
    )
  },
  {
    header: 'Status',
    accessor: 'status',
    render: (row: Deposit) => (
      <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold tracking-wide uppercase border ${statusColors[row.status] ?? 'bg-surface-container text-on-surface-variant border-outline-variant'}`}>
        {row.status.replace(/_/g, ' ')}
      </span>
    )
  },
  {
    header: 'TX Hash',
    accessor: 'txHash',
    render: (row: Deposit) => <span className="font-mono text-xs">{row.txHash ? `${row.txHash.slice(0, 6)}…${row.txHash.slice(-4)}` : '—'}</span>
  },
]

const Deposits: React.FC = () => {
  const { user } = useAuth()
  const [deposits, setDeposits] = useState<Deposit[]>([])
  const [statusFilter, setStatusFilter] = useState('all')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!user?.id) return
    client.get<Deposit[]>(`/merchants/${user.id}/deposits`)
      .then(res => setDeposits(res.data || []))
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [user?.id])

  const filtered = statusFilter === 'all'
    ? deposits
    : statusFilter === 'pending'
      ? deposits.filter(d => !['completed', 'refunded', 'failed'].includes(d.status))
      : deposits.filter(d => d.status === statusFilter)

  const totalVolume = deposits.reduce((s, d) => s + d.amountUsdc, 0)
  const pendingVolume = deposits
    .filter(d => !['completed', 'refunded', 'failed'].includes(d.status))
    .reduce((s, d) => s + d.amountUsdc, 0)

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col md:flex-row gap-4 mb-2">
        <div className="bg-surface-container-lowest p-4 rounded-xl border border-outline-variant/30 shadow-sm flex-1">
          <p className="text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-1">Total Volume</p>
          <p className="text-xl font-bold text-primary">
            {loading ? '—' : `${totalVolume.toLocaleString('en-US', { minimumFractionDigits: 2 })} USDC`}
          </p>
        </div>
        <div className="bg-surface-container-lowest p-4 rounded-xl border border-outline-variant/30 shadow-sm flex-1">
          <p className="text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-1">Pending Validation</p>
          <p className="text-xl font-bold text-surface-tint">
            {loading ? '—' : `${pendingVolume.toLocaleString('en-US', { minimumFractionDigits: 2 })} USDC`}
          </p>
        </div>
      </div>

      <div className="bg-surface-container-lowest rounded-t-xl border border-outline-variant/30 border-b-0 p-4 flex flex-col lg:flex-row justify-between items-center gap-4">
        <div className="flex flex-wrap items-center gap-3 w-full lg:w-auto">
          <div className="relative">
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="appearance-none bg-surface-container-low border border-outline-variant/50 text-sm rounded-lg pl-4 pr-10 py-2.5 focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-all cursor-pointer"
            >
              <option value="all">All Statuses</option>
              <option value="pending">Pending Validation</option>
              <option value="completed">Completed</option>
            </select>
            <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 text-on-surface-variant pointer-events-none" size={16} />
          </div>
        </div>
      </div>

      {loading ? (
        <div className="p-6 bg-surface-container-lowest rounded-b-xl border border-outline-variant/30 border-t-0">
          <TableSkeleton />
        </div>
      ) : filtered.length === 0 ? (
        <div className="p-12 text-center text-on-surface-variant bg-surface-container-lowest rounded-b-xl border border-outline-variant/30 border-t-0">
          No deposits found
        </div>
      ) : (
        <DataTable columns={columns} data={filtered} />
      )}
    </div>
  )
}

export default Deposits
