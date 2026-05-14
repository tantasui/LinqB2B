import React from 'react'
import { AlertTriangle, X } from 'lucide-react'

interface ConfirmationDialogProps {
  isOpen: boolean
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  onConfirm: () => void
  onCancel: () => void
  isDanger?: boolean
}

const ConfirmationDialog: React.FC<ConfirmationDialogProps> = ({ 
  isOpen, title, message, confirmText = 'Confirm', cancelText = 'Cancel', onConfirm, onCancel, isDanger = false 
}) => {
  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-inverse-surface/40 backdrop-blur-sm">
      <div className="bg-surface-container-lowest rounded-2xl shadow-2xl border border-outline-variant/30 w-full max-w-md overflow-hidden relative animate-in fade-in zoom-in duration-200">
        <div className={`h-1 w-full ${isDanger ? 'bg-error' : 'bg-primary'}`}></div>
        <div className="p-6">
          <div className="flex justify-between items-start mb-4">
            <div className={`w-10 h-10 rounded-full flex items-center justify-center ${isDanger ? 'bg-red-50 text-error' : 'bg-blue-50 text-primary'}`}>
              <AlertTriangle size={20} />
            </div>
            <button onClick={onCancel} className="text-on-surface-variant hover:text-on-surface">
              <X size={20} />
            </button>
          </div>
          <h3 className="text-xl font-bold text-on-surface mb-2">{title}</h3>
          <p className="text-sm text-on-surface-variant mb-6">{message}</p>
          <div className="flex flex-col sm:flex-row gap-3 justify-end">
            <button 
              onClick={onCancel}
              className="px-6 py-2.5 rounded-xl font-bold text-sm bg-surface-container-low text-on-surface hover:bg-surface-container transition-colors"
            >
              {cancelText}
            </button>
            <button 
              onClick={onConfirm}
              className={`px-6 py-2.5 rounded-xl font-bold text-sm text-white transition-all active:scale-95 ${
                isDanger ? 'bg-error hover:bg-error/90 shadow-red-200 shadow-lg' : 'bg-primary hover:bg-primary/90 shadow-indigo-200 shadow-lg'
              }`}
            >
              {confirmText}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

export default ConfirmationDialog
