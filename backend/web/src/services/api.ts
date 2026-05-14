import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

export default api

export const getDeposits = (merchantId: string, params: any) => 
  api.get(`/v1/merchants/${merchantId}/deposits`, { params })

export const getSettlements = (merchantId: string, params: any) => 
  api.get(`/v1/merchants/${merchantId}/settlements`, { params })

export const getOrders = (merchantId: string, params: any) => 
  api.get(`/v1/merchants/${merchantId}/orders`, { params })

export const createOrder = (merchantId: string, data: any) => 
  api.post(`/v1/merchants/${merchantId}/orders`, data)
