import React from 'react'
import SummaryCard from '../components/SummaryCard'
import { Landmark, Clock, AlertCircle, ArrowDownLeft } from 'lucide-react'

const Home: React.FC = () => {
  const recentActivity = [
    { id: 1, type: 'deposit', provider: 'Polygon', time: 'Today, 10:42 AM', txn: '0x4f...8a2b', amount: '+500.00 USDC', status: 'Settled' },
    { id: 2, type: 'settlement', provider: 'GTBank', time: 'Today, 09:15 AM', ref: 'SET-9921', amount: '-₦ 250,000', status: 'Pending' },
    { id: 3, type: 'deposit', provider: 'Solana', time: 'Yesterday, 14:30 PM', txn: '8Hj...pL9', amount: '+1,200.00 USDC', status: 'Settled' },
    { id: 4, type: 'failed', provider: 'Deposit Attempt', time: 'Yesterday, 11:05 AM', note: 'Network Timeout', amount: '150.00 USDC', status: 'Failed' },
    { id: 5, type: 'settlement', provider: 'Zenith Bank', time: 'Oct 24, 16:45 PM', ref: 'SET-9884', amount: '-₦ 1,500,000', status: 'Settled' },
  ]

  return (
    <div className="flex flex-col gap-8">
      {/* Bento Grid for Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <SummaryCard 
          variant="primary"
          title="USDC Received"
          mainValue="24,592.50"
          currency="USDC"
          subValues={[
            { label: 'Today', value: '+1,240.00' },
            { label: 'This Week', value: '+8,450.25' },
            { label: 'All-Time', value: '142k+' }
          ]}
        />
        
        <SummaryCard 
          title="NGN Settled"
          mainValue="₦ 18.2M"
          icon={<Landmark className="text-secondary" size={16} />}
          subValues={[
            { label: 'Today', value: '₦ 450K' },
            { label: 'This Week', value: '₦ 3.2M' }
          ]}
        />

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 lg:col-span-3">
          <div className="bg-surface-container-lowest rounded-2xl p-5 border border-outline-variant shadow-sm flex items-center gap-4 hover:border-primary-fixed-dim transition-colors cursor-pointer group">
            <div className="w-12 h-12 rounded-full bg-amber-50 flex items-center justify-center group-hover:bg-amber-100 transition-colors">
              <Clock className="text-amber-600" />
            </div>
            <div>
              <div className="text-2xl font-bold text-on-background">14</div>
              <div className="text-sm text-on-surface-variant font-medium">Pending Settlements</div>
            </div>
          </div>
          <div className="bg-surface-container-lowest rounded-2xl p-5 border border-outline-variant shadow-sm flex items-center gap-4 hover:border-error-container transition-colors cursor-pointer group">
            <div className="w-12 h-12 rounded-full bg-red-50 flex items-center justify-center group-hover:bg-red-100 transition-colors">
              <AlertCircle className="text-error" />
            </div>
            <div>
              <div className="text-2xl font-bold text-error">2</div>
              <div className="text-sm text-on-surface-variant font-medium">Failed Deposits</div>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Activity Feed */}
      <div className="mt-4">
        <div className="flex justify-between items-center mb-4 px-2">
          <h2 className="text-xl font-bold text-on-background">Recent Activity</h2>
          <button className="text-xs font-semibold uppercase tracking-wider text-primary hover:text-primary-container transition-colors">View All</button>
        </div>
        <div className="bg-surface-container-lowest rounded-2xl border border-outline-variant shadow-[0px_10px_30px_rgba(0,0,0,0.03)] overflow-hidden">
          <div className="divide-y divide-outline-variant/50">
            {recentActivity.map((activity) => (
              <div key={activity.id} className="flex items-center justify-between p-4 hover:bg-surface-container-low transition-colors">
                <div className="flex items-center gap-4">
                  <div className={`w-10 h-10 rounded-xl flex items-center justify-center border ${
                    activity.type === 'deposit' ? 'bg-emerald-50 border-emerald-100' : 
                    activity.type === 'failed' ? 'bg-red-50 border-red-100' : 'bg-primary-fixed border-primary-fixed-dim'
                  }`}>
                    {activity.type === 'deposit' ? <ArrowDownLeft className="text-emerald-600" size={18} /> : 
                     activity.type === 'failed' ? <AlertCircle className="text-error" size={18} /> : <Landmark className="text-primary" size={18} />}
                  </div>
                  <div>
                    <div className="text-base font-bold text-on-background mb-0.5">
                      {activity.type === 'deposit' ? `Deposit via ${activity.provider}` : 
                       activity.type === 'settlement' ? `Settlement to ${activity.provider}` : activity.provider}
                    </div>
                    <div className="text-xs text-on-surface-variant flex items-center gap-2">
                      <span>{activity.time}</span>
                      <span className="w-1 h-1 rounded-full bg-outline-variant"></span>
                      <span>{activity.txn || activity.ref || activity.note}</span>
                    </div>
                  </div>
                </div>
                <div className="text-right">
                  <div className={`text-base font-bold ${activity.type === 'deposit' ? 'text-emerald-600' : activity.type === 'failed' ? 'text-on-surface-variant opacity-70' : 'text-on-background'}`}>
                    {activity.amount}
                  </div>
                  <div className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider mt-1 ${
                    activity.status === 'Settled' ? 'bg-emerald-100 text-emerald-700' : 
                    activity.status === 'Pending' ? 'bg-amber-100 text-amber-700' : 'bg-red-100 text-red-700'
                  }`}>
                    {activity.status}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

export default Home
