import React from 'react'
import DataTable from '../components/DataTable'
import { Filter, Download, CheckCircle, Clock, Landmark, PieChart } from 'lucide-react'

const Settlements: React.FC = () => {
  const settlements = [
    { date: 'Oct 24, 14:30', usdc: '50,000.00', ngn: '₦58,150,000', ref: 'REF-99281A', status: 'Settled' },
    { date: 'Oct 23, 09:15', usdc: '12,500.00', ngn: '₦14,537,500', ref: 'REF-44512B', status: 'Settled' },
    { date: 'Oct 22, 16:45', usdc: '105,000.00', ngn: '₦122,115,000', ref: 'REF-88712C', status: 'Pending' },
  ]

  const columns = [
    { header: 'Date (USDC/UTC)', accessor: 'date' },
    { header: 'USDC Amount', accessor: 'usdc', render: (row: any) => <span className="font-bold">{row.usdc}</span> },
    { header: 'NGN Disbursed', accessor: 'ngn', render: (row: any) => <span className="text-on-surface-variant">{row.ngn}</span> },
    { header: 'Bank Ref', accessor: 'ref', render: (row: any) => <span className="font-mono text-xs text-outline">{row.ref}</span> },
    { 
      header: 'Status', 
      accessor: 'status',
      render: (row: any) => (
        <div className="text-right">
          <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold ${
            row.status === 'Settled' ? 'bg-emerald-100 text-emerald-800' : 'bg-amber-100 text-amber-800'
          }`}>
            {row.status}
          </span>
        </div>
      )
    }
  ]

  return (
    <div className="flex flex-col gap-8">
      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 glass-shadow relative overflow-hidden group">
          <div className="flex justify-between items-start mb-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-on-surface-variant">Total Volume Settled</span>
            <div className="w-10 h-10 rounded-lg bg-surface-container-high flex items-center justify-center text-primary">
              <PieChart size={20} />
            </div>
          </div>
          <h3 className="text-2xl font-bold text-on-surface mb-1">$2,450,180.00 <span className="text-lg text-outline font-medium">USDC</span></h3>
          <div className="flex items-center gap-2 mt-2">
            <span className="flex items-center text-[10px] font-bold text-emerald-600 bg-emerald-50 px-2 py-1 rounded-md">
              +12.5%
            </span>
            <span className="text-xs text-outline">vs last month</span>
          </div>
        </div>
        
        <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 glass-shadow relative overflow-hidden group">
          <div className="flex justify-between items-start mb-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-on-surface-variant">Total NGN Disbursed</span>
            <div className="w-10 h-10 rounded-lg bg-surface-container-high flex items-center justify-center text-secondary">
              <Landmark size={20} />
            </div>
          </div>
          <h3 className="text-2xl font-bold text-on-surface mb-1">₦2.85B</h3>
          <div className="mt-2">
            <span className="text-xs font-medium text-on-surface-variant bg-surface-container px-2 py-1 rounded-md">
              Avg Rate: ₦1,163/$
            </span>
          </div>
        </div>

        <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 glass-shadow relative overflow-hidden group">
          <div className="flex justify-between items-start mb-4">
            <span className="text-[10px] font-bold uppercase tracking-wider text-on-surface-variant">Success Rate</span>
            <div className="w-10 h-10 rounded-lg bg-surface-container-high flex items-center justify-center text-tertiary">
              <CheckCircle size={20} />
            </div>
          </div>
          <h3 className="text-2xl font-bold text-on-surface mb-1">99.8%</h3>
          <div className="flex items-center gap-2 mt-2">
            <span className="flex items-center text-[10px] font-bold text-error bg-error-container/30 px-2 py-1 rounded-md">
              2 Failed
            </span>
            <span className="text-xs text-outline">this week</span>
          </div>
        </div>
      </div>

      <div className="flex flex-col xl:flex-row gap-8">
        <div className="flex-1 bg-surface-container-lowest border border-outline-variant/30 rounded-2xl glass-shadow overflow-hidden">
          <div className="p-6 border-b border-outline-variant/30 flex justify-between items-center">
            <h2 className="text-xl font-bold text-on-surface">Recent Settlements</h2>
            <button className="flex items-center gap-2 text-sm font-bold text-primary hover:bg-primary-container/10 px-3 py-1.5 rounded-lg transition-colors">
              <Filter size={16} /> Filter
            </button>
          </div>
          <DataTable columns={columns} data={settlements} onRowClick={() => {}} />
        </div>

        {/* Side Panel Mockup */}
        <div className="hidden xl:flex flex-col w-[380px] bg-surface-container-lowest border border-outline-variant/30 rounded-2xl glass-shadow overflow-hidden flex-shrink-0">
          <div className="p-6 border-b border-outline-variant/30 bg-surface-container-low/50 relative overflow-hidden">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-bold text-on-surface">Settlement Details</h3>
              <button className="text-outline hover:text-on-surface"><Clock size={20} /></button>
            </div>
            <div className="text-center py-4">
              <p className="text-[10px] font-bold text-on-surface-variant mb-2 uppercase tracking-wider">Amount Settled</p>
              <h2 className="text-4xl font-black text-primary tracking-tight">50,000<span className="text-xl text-primary/70 font-medium">.00</span></h2>
              <p className="text-sm text-outline mt-1">USDC</p>
            </div>
          </div>
          <div className="p-6 flex-1 flex flex-col gap-5">
            {[
              { label: 'NGN Disbursed', value: '₦58,150,000' },
              { label: 'Exchange Rate', value: '₦1,163 / $1' },
              { label: 'Nomba Reference', value: 'NMB-7729-XYZ', mono: true },
              { label: 'Date & Time', value: 'Oct 24, 2023 • 14:30 UTC' }
            ].map(item => (
              <div key={item.label} className="flex justify-between items-center py-2 border-b border-outline-variant/20">
                <span className="text-sm text-outline">{item.label}</span>
                <span className={`text-sm font-bold text-on-surface ${item.mono ? 'font-mono bg-surface-container px-2 py-1 rounded' : ''}`}>{item.value}</span>
              </div>
            ))}
            <div className="flex justify-between items-center py-2">
              <span className="text-sm text-outline">Status</span>
              <span className="inline-flex items-center px-3 py-1 rounded-full text-sm font-bold bg-emerald-100 text-emerald-800 gap-1">
                <CheckCircle size={14} /> Settled
              </span>
            </div>
          </div>
          <div className="p-6 bg-surface-container-low/30 border-t border-outline-variant/30">
            <button className="w-full py-3 bg-white text-primary border border-primary/20 hover:border-primary hover:bg-primary-container/5 rounded-xl font-bold transition-all flex justify-center items-center gap-2">
              <Download size={18} /> Download Receipt
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default Settlements
