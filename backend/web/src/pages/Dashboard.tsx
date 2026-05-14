import React, { useState, useEffect } from 'react';
import { LogOut, Copy, CheckCircle2 } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import client from '../api/client';

interface Deposit {
  id: string;
  merchantName: string;
  txHash: string;
  amountUsdc: number;
  status: string;
  createdAt: string;
}

const STATUS_COLORS: Record<string, string> = {
  received: 'bg-blue-500/20 text-blue-300',
  processing: 'bg-yellow-500/20 text-yellow-300 pulse',
  swept: 'bg-purple-500/20 text-purple-300',
  fiat_pending: 'bg-orange-500/20 text-orange-300 pulse',
  completed: 'bg-green-500/20 text-green-300',
  mismatch_detected: 'bg-red-500/20 text-red-300',
  amount_validated: 'bg-emerald-500/20 text-emerald-300',
};

const STATUS_LABELS: Record<string, string> = {
  received: '⬇ Received',
  processing: '🔄 Processing',
  swept: '🏦 Swept',
  fiat_pending: '💱 Fiat Pending',
  completed: '✅ Completed',
  mismatch_detected: '⚠️ Mismatch',
  amount_validated: '✅ Validated',
};

export const Dashboard: React.FC = () => {
  const { user, logout } = useAuth();
  const [deposits, setDeposits] = useState<Deposit[]>([]);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    const fetchDeposits = async () => {
      try {
        const res = await client.get('/merchants/deposits');
        setDeposits(res.data || []);
      } catch (err) {
        console.error('Failed to fetch deposits', err);
      }
    };

    fetchDeposits();
    const interval = setInterval(fetchDeposits, 5000);
    return () => clearInterval(interval);
  }, []);

  const handleCopy = () => {
    if (user?.suiAddress) {
      navigator.clipboard.writeText(user.suiAddress);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const shortHash = (hash: string) => {
    if (!hash) return '';
    return hash.length > 12 ? `${hash.slice(0, 6)}...${hash.slice(-4)}` : hash;
  };

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100">
      {/* Header */}
      <header className="border-b border-gray-800 bg-gray-900/50 backdrop-blur-sm sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="w-10 h-10 rounded-xl bg-blue-600 flex items-center justify-center text-lg font-bold shadow-lg shadow-blue-500/20">
              B2
            </div>
            <div>
              <h1 className="text-lg font-bold">{user?.name || 'B2B Merchant'}</h1>
              <div className="flex items-center gap-2 text-xs">
                <span className="bg-yellow-500/10 text-yellow-500 px-2 py-0.5 rounded-full font-medium border border-yellow-500/20">
                  Sui Mainnet
                </span>
                <span className="flex items-center gap-1.5 text-gray-400">
                  <span className="w-1.5 h-1.5 rounded-full bg-green-400 pulse"></span>
                  Live Connection
                </span>
              </div>
            </div>
          </div>
          
          <button
            onClick={logout}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-400 hover:text-white bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors border border-gray-700"
          >
            <LogOut className="w-4 h-4" />
            Sign Out
          </button>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-8 grid grid-cols-1 lg:grid-cols-3 gap-8">
        
        {/* Left Column: Merchant Profile */}
        <div className="space-y-6">
          <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6 shadow-xl">
            <h2 className="font-bold text-gray-100 mb-6 flex items-center gap-2">
              Business Profile
            </h2>
            
            <div className="space-y-5">
              <div>
                <label className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-1">
                  Bank Details
                </label>
                <div className="bg-gray-950 border border-gray-800 rounded-lg p-3">
                  <p className="font-medium text-sm text-gray-200">{user?.bankName}</p>
                  <p className="text-xs text-gray-400 font-mono mt-0.5">{user?.accountNumber}</p>
                </div>
              </div>

              <div>
                <label className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-1">
                  Sui Settlement Address
                </label>
                <div className="flex items-center gap-2">
                  <div className="bg-gray-950 border border-gray-800 rounded-lg p-3 flex-1 overflow-hidden">
                    <p className="text-xs text-blue-400 font-mono truncate">{user?.suiAddress}</p>
                  </div>
                  <button
                    onClick={handleCopy}
                    className="p-3 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded-lg transition-colors text-gray-400 hover:text-white"
                  >
                    {copied ? <CheckCircle2 className="w-4 h-4 text-green-400" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Right Column: Deposits Feed */}
        <div className="lg:col-span-2">
          <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6 shadow-xl h-[calc(100vh-8rem)] flex flex-col">
            <div className="flex items-center justify-between mb-6">
              <h2 className="font-bold text-gray-100 flex items-center gap-2">
                Deposit Feed
                <span className="bg-gray-800 text-gray-400 text-xs px-2 py-0.5 rounded-full border border-gray-700">
                  {deposits.length}
                </span>
              </h2>
            </div>
            
            <div className="flex-1 overflow-y-auto space-y-3 pr-2 custom-scrollbar">
              {deposits.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-gray-500 space-y-3">
                  <div className="w-12 h-12 rounded-full bg-gray-800 flex items-center justify-center">
                    <div className="w-6 h-6 border-2 border-gray-600 rounded-full border-t-transparent animate-spin"></div>
                  </div>
                  <p className="text-sm font-medium">Waiting for deposits...</p>
                </div>
              ) : (
                deposits.map(d => {
                  const cls = STATUS_COLORS[d.status] || 'bg-gray-800 text-gray-300';
                  const lbl = STATUS_LABELS[d.status] || d.status;
                  
                  return (
                    <div key={d.id} className="bg-gray-950 border border-gray-800 rounded-xl p-4 hover:border-gray-700 transition-colors">
                      <div className="flex items-center justify-between mb-2">
                        <span className="font-medium text-sm text-gray-200">{d.merchantName}</span>
                        <span className={`text-xs px-2.5 py-1 rounded-md font-semibold tracking-wide ${cls}`}>
                          {lbl}
                        </span>
                      </div>
                      <div className="flex items-center justify-between mt-3">
                        <div className="flex flex-col">
                          <span className="text-[10px] text-gray-500 uppercase font-semibold mb-0.5">Transaction Hash</span>
                          <span className="font-mono text-xs text-gray-400">{shortHash(d.txHash)}</span>
                        </div>
                        <div className="flex flex-col items-end">
                          <span className="text-[10px] text-gray-500 uppercase font-semibold mb-0.5">Amount</span>
                          <span className="text-green-400 font-bold text-sm">{d.amountUsdc.toFixed(2)} USDC</span>
                        </div>
                      </div>
                      <div className="mt-3 pt-3 border-t border-gray-800/50 flex items-center justify-between text-[11px] text-gray-500">
                        <span>ID: {shortHash(d.id)}</span>
                        <span>{new Date(d.createdAt).toLocaleString()}</span>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>
        </div>
      </main>
    </div>
  );
};
