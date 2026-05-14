import React from 'react'
import DataTable from '../components/DataTable'
import { Plus, Copy, Download, Trash2, QrCode } from 'lucide-react'

const PaymentLinks: React.FC = () => {
  const links = [
    { amount: '₦ 150,000.00', url: 'pay.gp/x8j9q2', date: 'Oct 26, 2023, 10:45 AM', status: 'Active' },
    { amount: '₦ 45,500.00', url: 'pay.gp/m4v1p0', date: 'Oct 24, 2023, 02:15 PM', status: 'Used' },
    { amount: '₦ 12,000.00', url: 'pay.gp/k9z7b3', date: 'Oct 20, 2023, 09:00 AM', status: 'Expired' },
  ]

  const columns = [
    { header: 'Amount (NGN)', accessor: 'amount', render: (row: any) => <span className="text-lg font-bold">{row.amount}</span> },
    { header: 'Link', accessor: 'url', render: (row: any) => <span className="text-primary underline underline-offset-4 font-medium">{row.url}</span> },
    { 
      header: 'QR Code', 
      accessor: 'qr',
      render: () => (
        <div className="h-10 w-10 bg-surface-container rounded-lg flex items-center justify-center border border-outline-variant">
          <QrCode className="text-on-surface-variant" size={20} />
        </div>
      )
    },
    { header: 'Created At', accessor: 'date', render: (row: any) => <span className="text-on-surface-variant text-sm">{row.date}</span> },
    { 
      header: 'Status', 
      accessor: 'status',
      render: (row: any) => (
        <span className={`inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold ${
          row.status === 'Active' ? 'bg-primary-container text-on-primary-container' : 'bg-surface-dim text-on-surface-variant border border-outline-variant'
        }`}>
          {row.status}
        </span>
      )
    },
    { 
      header: '', 
      accessor: 'actions',
      render: () => (
        <div className="flex items-center justify-end gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
          <button className="p-2 text-on-surface-variant hover:bg-surface-container rounded-full transition-colors" title="Copy Link"><Copy size={18} /></button>
          <button className="p-2 text-on-surface-variant hover:bg-surface-container rounded-full transition-colors" title="Download QR"><Download size={18} /></button>
          <button className="p-2 text-error hover:bg-error-container hover:text-on-error-container rounded-full transition-colors" title="Delete"><Trash2 size={18} /></button>
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
        <button className="inline-flex items-center justify-center gap-2 bg-primary text-on-primary font-bold px-6 py-3 rounded-full hover:bg-primary-container hover:text-on-primary-container transition-all shadow-lg active:scale-95">
          <Plus size={18} />
          Generate Payment Link
        </button>
      </div>

      <div className="bg-surface/60 backdrop-blur-xl border border-outline-variant rounded-xl shadow-lg overflow-hidden">
        <DataTable columns={columns} data={links} />
        <div className="px-6 py-4 border-t border-outline-variant bg-surface-container-lowest/30 flex items-center justify-between">
          <span className="text-sm text-on-surface-variant">Showing 3 of 12 links</span>
          <div className="flex items-center gap-2">
            <button className="px-3 py-1.5 border border-outline-variant rounded-md text-sm hover:bg-surface-container disabled:opacity-50 transition-colors" disabled>Previous</button>
            <button className="px-3 py-1.5 border border-outline-variant rounded-md text-sm hover:bg-surface-container transition-colors">Next</button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default PaymentLinks
