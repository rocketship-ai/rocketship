import { useState, useEffect } from 'react'

/**
 * Hook that debounces a value by the specified delay.
 * Returns the debounced value that updates after the delay has passed
 * since the last change to the input value.
 *
 * @param value - The value to debounce
 * @param delay - Delay in milliseconds (default: 300ms)
 * @returns The debounced value
 */
export function useDebouncedValue<T>(value: T, delay: number = 300): T {
  const [debouncedValue, setDebouncedValue] = useState<T>(value)

  useEffect(() => {
    // Set up a timer to update the debounced value after the delay
    const timer = setTimeout(() => {
      setDebouncedValue(value)
    }, delay)

    // Clean up the timer if the value or delay changes before it fires
    return () => {
      clearTimeout(timer)
    }
  }, [value, delay])

  return debouncedValue
}
