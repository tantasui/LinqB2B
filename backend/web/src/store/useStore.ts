import { create } from 'zustand'

interface Merchant {
  id: string
  name: string
  bankAccount: string
  suiAddress: string
  currency: string
}

interface AuthState {
  token: string | null
  merchantId: string | null
  merchant: Merchant | null
  setToken: (token: string | null) => void
  setMerchantId: (id: string | null) => void
  setMerchant: (merchant: Merchant | null) => void
  logout: () => void
}

export const useStore = create<AuthState>((set) => ({
  token: localStorage.getItem('auth_token'),
  merchantId: localStorage.getItem('merchant_id'),
  merchant: null,
  setToken: (token) => {
    if (token) localStorage.setItem('auth_token', token)
    else localStorage.removeItem('auth_token')
    set({ token })
  },
  setMerchantId: (id) => {
    if (id) localStorage.setItem('merchant_id', id)
    else localStorage.removeItem('merchant_id')
    set({ merchantId: id })
  },
  setMerchant: (merchant) => set({ merchant }),
  logout: () => {
    localStorage.removeItem('auth_token')
    localStorage.removeItem('merchant_id')
    set({ token: null, merchantId: null, merchant: null })
  },
}))
