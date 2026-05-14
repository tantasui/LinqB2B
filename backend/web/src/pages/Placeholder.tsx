import React from 'react'

const Placeholder: React.FC<{ name: string }> = ({ name }) => (
  <div className="flex flex-col items-center justify-center min-h-[50vh] text-on-surface-variant">
    <h2 className="text-2xl font-bold">{name} Page</h2>
    <p>This page is currently being converted from the design.</p>
  </div>
)

export default Placeholder
