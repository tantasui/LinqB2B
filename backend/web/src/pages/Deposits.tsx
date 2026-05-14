import React, { useState } from 'react'
import DataTable from '../components/DataTable'
import { Calendar, ChevronDown, MoreVertical, ArrowDownLeft } from 'lucide-react'

const Deposits: React.FC = () => {
  const [statusFilter, setStatusFilter] = useState('all')

  const deposits = [
    { date: 'Oct 24, 2023', time: '14:32:01 UTC', amount: '50,000.00', status: 'completed', hash: '0x71C...3a9B' },
    { date: 'Oct 24, 2023', time: '10:15:44 UTC', amount: '120,500.00', status: 'pending_order_validation', hash: '0x1f9...e2A1' },
    { date: 'Oct 23, 2023', time: '08:00:12 UTC', amount: '25,000.00', status: 'swept', hash: '0x4aB...9c2F' },
    { date: 'Oct 21, 2023', time: '16:45:00 UTC', amount: '10,000.00', status: 'refunded', hash: '0x8bC...1dE4' },
    { date: 'Oct 20, 2023', time: '09:12:33 UTC', amount: '75,000.00', status: 'amount_validated', hash: '0x9eF...2aB3' },
  ]

  const columns = [
    { 
      header: 'Date', 
      accessor: 'date',
      render: (row: any) => (
        <div>
          <div className="font-bold">{row.date}</div>
          <div className="text-on-surface-variant text-xs">{row.time}</div>
        </div>
      )
    },
    { 
      header: 'USDC Amount', 
      accessor: 'amount',
      render: (row: any) => (
        <div className="flex items-center gap-2">
          <div className="w-6 h-6 rounded-full bg-surface-container flex items-center justify-center">
            <ArrowDownLeft size={12} className="text-surface-tint" />
          </div>
          <span className="font-bold text-base">{row.amount}</span>
        </div>
      )
    },
    { 
      header: 'Status', 
      accessor: 'status',
      render: (row: any) => {
        const colors: any = {
          completed: 'bg-emerald-50 text-emerald-700 border-emerald-200',
          pending_order_validation: 'bg-amber-50 text-amber-700 border-amber-200',
          swept: 'bg-blue-50 text-blue-700 border-blue-200',
          refunded: 'bg-slate-100 text-slate-600 border-slate-200',
          amount_validated: 'bg-purple-50 text-purple-700 border-purple-200',
        }
        return (
          <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold tracking-wide uppercase border ${colors[row.status]}`}>
            {row.status.replace(/_/g, ' ')}
          </span>
        )
      }
    },
    { header: 'TX Hash', accessor: 'hash', render: (row: any) => <span className="font-mono text-xs">{row.hash}</span> },
    { 
      header: '', 
      accessor: 'actions',
      render: () => (
        <div className="text-right">
          <button className="p-2 rounded-lg hover:bg-surface-variant text-on-surface-variant transition-colors opacity-50 hover:opacity-100">
            <MoreVertical size={16} />
          </button>
        </div>
      )
    }
  ]

  return (
    <div className="flex flex-col gap-6">
      {/* Metrics Bento Mini */}
      <div className="flex flex-col md:flex-row gap-4 mb-2">
        <div className="bg-surface-container-lowest p-4 rounded-xl border border-outline-variant/30 shadow-sm flex-1">
          <p className="text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-1">Total Volume</p>
          <p className="text-xl font-bold text-primary">2.4M USDC</p>
        </div>
        <div className="bg-surface-container-lowest p-4 rounded-xl border border-outline-variant/30 shadow-sm flex-1">
          <p className="text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-1">Pending Validation</p>
          <p className="text-xl font-bold text-surface-tint">120k USDC</p>
        </div>
      </div>

      {/* Controls */}
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
          <div className="relative flex items-center bg-surface-container-low border border-outline-variant/50 rounded-lg overflow-hidden px-3 py-2.5">
            <Calendar className="text-on-surface-variant" size={16} />
            <span className="text-sm text-on-surface mx-2">Oct 01 - Oct 31, 2023</span>
          </div>
        </div>
        <div className="flex items-center gap-4 w-full lg:w-auto justify-between lg:justify-end">
          <div className="flex items-center gap-2">
            <span className="text-sm text-on-surface-variant">Sort by:</span>
            <button className="text-sm text-primary font-bold flex items-center gap-1">
              Date (Newest) <ChevronDown size={14} />
            </button>
          </div>
        </div>
      </div>

      <DataTable columns={columns} data={deposits} onRowClick={(row) => console.log('Row clicked', row)} />

      {/* Pagination */}
      <div className="flex flex-col sm:flex-row justify-between items-center px-2">
        <div className="flex items-center gap-2 text-sm text-on-surface-variant">
          <span>Show</span>
          <select className="bg-surface-container-low border border-outline-variant/50 rounded-md py-1 px-2">
            <option value="10">10</option>
            <option value="25">25</option>
          </select>
          <span>per page</span>
        </div>
        <div className="flex items-center gap-1 mt-4 sm:mt-0">
          <button className="p-2 rounded-lg text-on-surface-variant hover:bg-surface-container-low disabled:opacity-50" disabled>
            Prev
          </button>
          <button className="w-8 h-8 rounded-lg bg-primary-container text-on-primary-container font-bold text-sm">1</button>
          <button className="w-8 h-8 rounded-lg text-on-surface-variant hover:bg-surface-container-low font-bold text-sm">2</button>
          <button className="p-2 rounded-lg text-on-surface-variant hover:bg-surface-container-low">
            Next
          </button>
        </div>
      </div>
    </div>
  )
}

export default Deposits
