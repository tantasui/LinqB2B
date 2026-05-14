import React from 'react'
import { MoreHorizontal } from 'lucide-react'

interface SummaryCardProps {
  title: string
  mainValue: string
  currency?: string
  subValues: { label: string; value: string }[]
  variant?: 'primary' | 'secondary'
  icon?: React.ReactNode
}

const SummaryCard: React.FC<SummaryCardProps> = ({ title, mainValue, currency, subValues, variant = 'secondary', icon }) => {
  if (variant === 'primary') {
    return (
      <div className="lg:col-span-2 bg-gradient-to-br from-primary-container to-primary rounded-2xl p-6 shadow-[0px_15px_35px_rgba(124,58,237,0.25)] relative overflow-hidden flex flex-col justify-between min-h-[220px]">
        <div className="absolute top-0 right-0 -mr-10 -mt-10 w-48 h-48 bg-white opacity-10 rounded-full blur-2xl"></div>
        <div className="absolute bottom-0 left-0 -ml-10 -mb-10 w-32 h-32 bg-secondary opacity-20 rounded-full blur-xl"></div>
        <div className="relative z-10 flex justify-between items-start">
          <div className="flex items-center gap-2 bg-white/20 backdrop-blur-md rounded-full px-3 py-1 border border-white/10">
            <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
            <span className="text-[12px] font-semibold text-white uppercase tracking-wider">{title}</span>
          </div>
          <MoreHorizontal className="text-white/50" />
        </div>
        <div className="relative z-10 mt-8">
          <div className="text-5xl font-bold text-white mb-1">
            {mainValue} <span className="text-2xl text-white/70">{currency}</span>
          </div>
          <div className="flex items-center gap-4 text-white/80 text-sm">
            {subValues.map((sv, i) => (
              <React.Fragment key={sv.label}>
                <div className="flex flex-col">
                  <span className="text-white/50 text-xs uppercase tracking-wider">{sv.label}</span>
                  <span className="font-medium">{sv.value}</span>
                </div>
                {i < subValues.length - 1 && <div className="w-px h-8 bg-white/20"></div>}
              </React.Fragment>
            ))}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="bg-surface-container-lowest rounded-2xl p-6 border border-outline-variant shadow-[0px_10px_30px_rgba(0,0,0,0.05)] flex flex-col justify-between h-full">
      <div className="flex justify-between items-start mb-6">
        <div className="flex items-center gap-2">
          <div className="w-8 h-8 rounded-full bg-surface-container-high flex items-center justify-center">
            {icon}
          </div>
          <span className="text-[12px] font-semibold text-on-surface uppercase tracking-wider">{title}</span>
        </div>
      </div>
      <div>
        <div className="text-3xl font-bold text-on-background mb-2">{mainValue}</div>
        <div className="grid grid-cols-2 gap-4 mt-4 bg-surface-container-low rounded-xl p-3">
          {subValues.map(sv => (
            <div key={sv.label}>
              <div className="text-xs text-on-surface-variant mb-1 font-medium uppercase tracking-wider">{sv.label}</div>
              <div className="font-semibold text-on-surface text-sm">{sv.value}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export default SummaryCard
