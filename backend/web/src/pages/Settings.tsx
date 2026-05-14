import React from 'react'
import { Store, Lock, Webhook, LogOut, Copy, AlertCircle, Landmark } from 'lucide-react'
import { useStore } from '../store/useStore'

const Settings: React.FC = () => {
  const { merchant, logout } = useStore()

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
      {/* Left Column: Profile & Security */}
      <div className="lg:col-span-2 flex flex-col gap-8">
        {/* Merchant Profile Card */}
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
                {merchant?.name || 'Apex Global Trading LLC'}
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2">Primary Bank Account</label>
                <div className="bg-surface-container-low px-4 py-3 rounded-lg border border-outline-variant/20 text-on-surface flex items-center gap-3">
                  <Landmark className="text-outline" size={16} />
                  {merchant?.bankAccount || '**** 8942'}
                </div>
              </div>
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2">Settlement Currency</label>
                <div className="bg-surface-container-low px-4 py-3 rounded-lg border border-outline-variant/20 text-on-surface">
                  {merchant?.currency || 'USD'}
                </div>
              </div>
            </div>
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2">Sui Wallet Address</label>
              <div className="bg-surface-container-low px-4 py-3 rounded-lg border border-outline-variant/20 text-on-surface font-mono text-sm break-all flex justify-between items-center group/copy cursor-pointer hover:bg-surface-container-highest transition-colors">
                {merchant?.suiAddress || '0x7b4a...f92e11a'}
                <Copy className="text-outline group-hover/copy:text-primary transition-colors" size={16} />
              </div>
            </div>
          </div>
        </section>

        {/* Security / Password Card */}
        <section className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 shadow-sm">
          <h2 className="text-xl font-bold text-on-surface mb-6 flex items-center gap-2">
            <Lock className="text-primary" />
            Security
          </h2>
          <form className="space-y-5">
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2" htmlFor="current_password">Current Password</label>
              <input 
                className="w-full bg-surface-container-low border border-outline-variant/20 rounded-lg px-4 py-3 text-on-surface focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all" 
                id="current_password" 
                placeholder="Enter current password" 
                type="password"
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
                />
              </div>
              <div>
                <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2" htmlFor="confirm_password">Confirm New Password</label>
                <input 
                  className="w-full bg-surface-container-low border border-outline-variant/20 rounded-lg px-4 py-3 text-on-surface focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all" 
                  id="confirm_password" 
                  placeholder="Confirm new password" 
                  type="password"
                />
              </div>
            </div>
            <div className="flex justify-end pt-2">
              <button className="bg-primary hover:bg-primary/90 text-on-primary px-6 py-2.5 rounded-lg font-bold transition-all active:scale-95 shadow-md" type="button">Update Password</button>
            </div>
          </form>
        </section>
      </div>

      {/* Right Column: Webhooks & Actions */}
      <div className="flex flex-col gap-8">
        {/* Webhook Configuration */}
        <section className="bg-surface-container-lowest border border-outline-variant/30 rounded-xl p-6 shadow-sm">
          <h2 className="text-xl font-bold text-on-surface mb-2 flex items-center gap-2">
            <Webhook className="text-primary" />
            Webhooks
          </h2>
          <p className="text-sm text-on-surface-variant mb-6">Receive real-time event notifications to your server.</p>
          <div className="space-y-4">
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2" htmlFor="webhook_url">Endpoint URL</label>
              <input 
                className="w-full bg-surface-container-low border border-outline-variant/20 rounded-lg px-4 py-3 text-on-surface focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all" 
                id="webhook_url" 
                placeholder="https://api.yourdomain.com/webhooks" 
                type="url"
              />
            </div>
            <div>
              <label className="block text-xs font-bold uppercase tracking-wider text-on-surface-variant mb-2" htmlFor="webhook_secret">Signing Secret</label>
              <input 
                className="w-full bg-surface-container-low border border-outline-variant/20 rounded-lg px-4 py-3 text-on-surface focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all" 
                id="webhook_secret" 
                placeholder="whsec_..." 
                type="password"
              />
              <p className="text-[10px] text-on-surface-variant mt-2">Used to verify the events were sent by Linq.</p>
            </div>
            <div className="pt-4 border-t border-outline-variant/20">
              <label className="flex items-center gap-3 cursor-pointer group">
                <div className="relative flex items-center">
                  <input defaultChecked className="sr-only peer" type="checkbox"/>
                  <div className="w-11 h-6 bg-surface-container-highest rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary"></div>
                </div>
                <span className="text-sm font-bold text-on-surface group-hover:text-primary transition-colors">Enable Webhooks</span>
              </label>
            </div>
            <button className="w-full bg-surface-container hover:bg-surface-container-highest text-primary px-4 py-2.5 rounded-lg font-bold transition-all active:scale-95 border border-outline-variant/20 mt-4" type="button">Save Webhook Settings</button>
          </div>
        </section>

        {/* Danger Zone */}
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
