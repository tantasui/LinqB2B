import React, { useState } from 'react'
import { Store, Lock, LogOut, Copy, AlertCircle, Landmark } from 'lucide-react'
import { toast } from 'sonner'
import { useAuth } from '../../context/AuthContext'
import client from '../../api/client'

const Settings: React.FC = () => {
  const { user, logout } = useAuth()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [updating, setUpdating] = useState(false)

  const handleUpdatePassword = async () => {
    if (!newPassword || !currentPassword) {
      toast.error('All password fields are required')
      return
    }
    if (newPassword !== confirmPassword) {
      toast.error('New passwords do not match')
      return
    }
    if (newPassword.length < 6) {
      toast.error('New password must be at least 6 characters')
      return
    }
    setUpdating(true)
    try {
      await client.put(`/merchants/${user?.id}/password`, {
        current_password: currentPassword,
        new_password: newPassword,
      })
      toast.success('Password updated successfully')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err: any) {
      if (err.response?.status === 401) {
        toast.error('Current password is incorrect')
      } else {
        toast.error('Failed to update password')
      }
    } finally {
      setUpdating(false)
    }
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
      <div className="lg:col-span-2 flex flex-col gap-8">
        <section className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 shadow-sm relative overflow-hidden group">
          <div className="absolute top-0 right-0 w-64 h-64 bg-gradient-to-br from-primary-container/10 to-transparent rounded-bl-full -z-10 transition-transform duration-500 group-hover:scale-110"></div>
          <h2 className="text-xl font-bold text-on-surface mb-6 flex items-center gap-2">
            <Store className="text-primary" />
            Merchant Profile
          </h2>
          <div className="space-y-5">
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2">Legal Entity Name</label>
              <div className="bg-surface-container-low px-4 py-3 rounded-lg border border-outline-variant/20 text-on-surface">
                {user?.name || 'Business Name'}
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2">Primary Bank Account</label>
                <div className="bg-surface-container-low px-4 py-3 rounded-lg border border-outline-variant/20 text-on-surface flex items-center gap-3">
                  <Landmark className="text-outline" size={16} />
                  {user?.accountNumber || '**** 8942'}
                </div>
              </div>
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2">Bank Name</label>
                <div className="bg-surface-container-low px-4 py-3 rounded-lg border border-outline-variant/20 text-on-surface">
                  {user?.bankName || '—'}
                </div>
              </div>
            </div>
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2">Sui Wallet Address</label>
              <div
                className="bg-surface-container-low px-4 py-3 rounded-lg border border-outline-variant/20 text-on-surface font-mono text-sm break-all flex justify-between items-center group/copy cursor-pointer hover:bg-surface-container-highest transition-colors"
                onClick={() => user?.suiAddress && navigator.clipboard.writeText(user.suiAddress)}
              >
                {user?.suiAddress || '0x...'}
                <Copy className="text-outline group-hover/copy:text-primary transition-colors flex-shrink-0 ml-2" size={16} />
              </div>
            </div>
          </div>
        </section>

        <section className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 shadow-sm">
          <h2 className="text-xl font-bold text-on-surface mb-6 flex items-center gap-2">
            <Lock className="text-primary" />
            Security
          </h2>
          <div className="space-y-5">
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2" htmlFor="current_password">Current Password</label>
              <input
                className="w-full bg-surface-container-low border border-outline-variant/20 rounded-lg px-4 py-3 text-on-surface focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
                id="current_password"
                placeholder="Enter current password"
                type="password"
                value={currentPassword}
                onChange={e => setCurrentPassword(e.target.value)}
              />
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2" htmlFor="new_password">New Password</label>
                <input
                  className="w-full bg-surface-container-low border border-outline-variant/20 rounded-lg px-4 py-3 text-on-surface focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
                  id="new_password"
                  placeholder="Enter new password"
                  type="password"
                  value={newPassword}
                  onChange={e => setNewPassword(e.target.value)}
                />
              </div>
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2" htmlFor="confirm_password">Confirm New Password</label>
                <input
                  className="w-full bg-surface-container-low border border-outline-variant/20 rounded-lg px-4 py-3 text-on-surface focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
                  id="confirm_password"
                  placeholder="Confirm new password"
                  type="password"
                  value={confirmPassword}
                  onChange={e => setConfirmPassword(e.target.value)}
                />
              </div>
            </div>
            <div className="flex justify-end pt-2">
              <button
                className="bg-primary hover:bg-primary/90 text-on-primary px-6 py-2.5 rounded-lg font-bold transition-all active:scale-95 shadow-md disabled:opacity-50 disabled:cursor-not-allowed"
                type="button"
                disabled={updating}
                onClick={handleUpdatePassword}
              >
                {updating ? 'Updating…' : 'Update Password'}
              </button>
            </div>
          </div>
        </section>
      </div>

      <div className="flex flex-col gap-8">
        <section className="bg-error-container/20 border border-error/20 rounded-xl p-6">
          <h2 className="text-xl font-bold text-error mb-2 flex items-center gap-2">
            <AlertCircle className="text-error" />
            Account Actions
          </h2>
          <p className="text-sm text-on-surface-variant mb-6">Securely log out of your institutional session.</p>
          <button
            onClick={logout}
            className="w-full bg-error hover:bg-error/90 text-on-error px-4 py-3 rounded-lg font-bold transition-all active:scale-95 flex justify-center items-center gap-2 shadow-sm"
          >
            <LogOut size={18} />
            Logout Session
          </button>
        </section>
      </div>
    </div>
  )
}

export default Settings
