import { useEffect, useRef } from 'react'

export const usePolling = (callback: () => void, delay: number = 30000) => {
  const savedCallback = useRef(callback)

  useEffect(() => {
    savedCallback.current = callback
  }, [callback])

  useEffect(() => {
    const tick = () => savedCallback.current()
    const id = setInterval(tick, delay)
    return () => clearInterval(id)
  }, [delay])
}
