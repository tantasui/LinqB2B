import React, { useEffect, useState } from 'react'
import SummaryCard from '../../components/SummaryCard'
import { Skeleton } from '../../components/Skeleton'
import { Landmark, Clock, AlertCircle, ArrowDownLeft } from 'lucide-react'
import { useAuth } from '../../context/AuthContext'
import client from '../../api/client'

interface Stats {
  usdcTotalReceived: number
  usdcToday: number
  usdcThisWeek: number
  ngnTotalSettled: number
  ngnToday: number
  ngnThisWeek: number
  pendingCount: number
  failedCount: number
}

interface Deposit {
  id: string
  createdAt: string
  txHash: string
  amountUsdc: number
  status: string
}

function fmtUsdc(n: number) {
  return n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function fmtNgn(n: number) {
  if (n >= 1_000_000) return `₦${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `₦${(n / 1_000).toFixed(0)}K`
  return `₦${n.toFixed(0)}`
}

function depositStatus(status: string) {
  if (status === 'completed') return 'Settled'
  if (status === 'refunded' || status === 'failed') return 'Failed'
  return 'Pending'
}

function fmtTime(iso: string) {
  const d = new Date(iso)
  const now = new Date()
  const diff = now.getTime() - d.getTime()
  if (diff < 86400000) return `Today, ${d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })}`
  if (diff < 172800000) return `Yesterday, ${d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })}`
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

const Home: React.FC = () => {
  const { user } = useAuth()
  const [stats, setStats] = useState<Stats | null>(null)
  const [deposits, setDeposits] = useState<Deposit[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!user?.id) return
    Promise.all([
      client.get<Stats>(`/merchants/${user.id}/stats`),
      client.get<Deposit[]>(`/merchants/${user.id}/deposits`),
    ]).then(([statsRes, depositsRes]) => {
      setStats(statsRes.data)
      setDeposits((depositsRes.data || []).slice(0, 5))
    }).catch(console.error).finally(() => setLoading(false))
  }, [user?.id])

  return (
    <div className="flex flex-col gap-8">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {loading ? (
          <>
            <Skeleton className="lg:col-span-2 h-[220px]" />
            <Skeleton className="h-[220px]" />
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 lg:col-span-3">
              <Skeleton className="h-20" />
              <Skeleton className="h-20" />
            </div>
          </>
        ) : (
          <>
            <SummaryCard
              variant="primary"
              title="USDC Received"
              mainValue={fmtUsdc(stats?.usdcTotalReceived ?? 0)}
              currency="USDC"
              subValues={[
                { label: 'Today', value: `+${fmtUsdc(stats?.usdcToday ?? 0)}` },
                { label: 'This Week', value: `+${fmtUsdc(stats?.usdcThisWeek ?? 0)}` },
              ]}
            />

            <SummaryCard
              title="NGN Settled"
              mainValue={fmtNgn(stats?.ngnTotalSettled ?? 0)}
              icon={<Landmark className="text-secondary" size={16} />}
              subValues={[
                { label: 'Today', value: fmtNgn(stats?.ngnToday ?? 0) },
                { label: 'This Week', value: fmtNgn(stats?.ngnThisWeek ?? 0) },
              ]}
            />

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6 lg:col-span-3">
              <div className="bg-surface-container-lowest rounded-2xl p-5 border border-outline-variant shadow-sm flex items-center gap-4 hover:border-primary-fixed-dim transition-colors cursor-pointer group">
                <div className="w-12 h-12 rounded-full bg-amber-50 flex items-center justify-center group-hover:bg-amber-100 transition-colors">
                  <Clock className="text-amber-600" />
                </div>
                <div>
                  <div className="text-2xl font-bold text-on-background">{stats?.pendingCount ?? 0}</div>
                  <div className="text-sm text-on-surface-variant font-medium">Pending Deposits</div>
                </div>
              </div>
              <div className="bg-surface-container-lowest rounded-2xl p-5 border border-outline-variant shadow-sm flex items-center gap-4 hover:border-error-container transition-colors cursor-pointer group">
                <div className="w-12 h-12 rounded-full bg-red-50 flex items-center justify-center group-hover:bg-red-100 transition-colors">
                  <AlertCircle className="text-error" />
                </div>
                <div>
                  <div className="text-2xl font-bold text-error">{stats?.failedCount ?? 0}</div>
                  <div className="text-sm text-on-surface-variant font-medium">Failed Deposits</div>
                </div>
              </div>
            </div>
          </>
        )}
      </div>

      <div className="mt-4">
        <div className="flex justify-between items-center mb-4 px-2">
          <h2 className="text-xl font-bold text-on-background">Recent Activity</h2>
        </div>
        <div className="bg-surface-container-lowest rounded-2xl border border-outline-variant shadow-[0px_10px_30px_rgba(0,0,0,0.03)] overflow-hidden">
          {loading ? (
            <div className="p-4 space-y-4">
              {[...Array(5)].map((_, i) => <Skeleton key={i} className="h-16 w-full" />)}
            </div>
          ) : deposits.length === 0 ? (
            <div className="p-12 text-center text-on-surface-variant">No recent activity</div>
          ) : (
            <div className="divide-y divide-outline-variant/50">
              {deposits.map((dep) => {
                const status = depositStatus(dep.status)
                return (
                  <div key={dep.id} className="flex items-center justify-between p-4 hover:bg-surface-container-low transition-colors">
                    <div className="flex items-center gap-4">
                      <div className={`w-10 h-10 rounded-xl flex items-center justify-center border ${
                        status === 'Failed' ? 'bg-red-50 border-red-100' : 'bg-emerald-50 border-emerald-100'
                      }`}>
                        {status === 'Failed'
                          ? <AlertCircle className="text-error" size={18} />
                          : <ArrowDownLeft className="text-emerald-600" size={18} />}
                      </div>
                      <div>
                        <div className="text-base font-bold text-on-background mb-0.5">Deposit</div>
                        <div className="text-xs text-on-surface-variant flex items-center gap-2">
                          <span>{fmtTime(dep.createdAt)}</span>
                          {dep.txHash && (
                            <>
                              <span className="w-1 h-1 rounded-full bg-outline-variant"></span>
                              <span className="font-mono">{dep.txHash.slice(0, 10)}…</span>
                            </>
                          )}
                        </div>
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="text-base font-bold text-emerald-600">+{fmtUsdc(dep.amountUsdc)} USDC</div>
                      <div className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider mt-1 ${
                        status === 'Settled' ? 'bg-emerald-100 text-emerald-700' :
                        status === 'Pending' ? 'bg-amber-100 text-amber-700' : 'bg-red-100 text-red-700'
                      }`}>
                        {status}
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default Home
