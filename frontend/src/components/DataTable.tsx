import React from 'react'

interface Column {
  header: string
  accessor: string
  render?: (row: any) => React.ReactNode
}

interface DataTableProps {
  columns: Column[]
  data: any[]
  onRowClick?: (row: any) => void
}

const DataTable: React.FC<DataTableProps> = ({ columns, data, onRowClick }) => {
  return (
    <div className="bg-surface-container-lowest border border-outline-variant shadow-[0_10px_30px_rgba(0,0,0,0.03)] overflow-hidden rounded-xl overflow-x-auto">
      <table className="w-full text-left border-collapse min-w-[800px]">
        <thead>
          <tr className="bg-surface-container/50 border-b border-outline-variant/30">
            {columns.map((col, i) => (
              <th key={i} className="p-4 text-xs font-bold uppercase tracking-wider text-on-surface-variant">
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-outline-variant/20">
          {data.map((row, i) => (
            <tr
              key={i}
              onClick={() => onRowClick?.(row)}
              className={`transition-colors ${onRowClick ? 'cursor-pointer hover:bg-surface-container-low/50' : ''}`}
            >
              {columns.map((col, j) => (
                <td key={j} className="p-4 text-sm text-on-surface">
                  {col.render ? col.render(row) : row[col.accessor]}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default DataTable
