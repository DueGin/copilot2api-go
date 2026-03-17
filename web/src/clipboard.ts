import { useCallback, useEffect, useRef, useState } from "react"

const COPY_FEEDBACK_MS = 1500

/**
 * Hook that copies text to clipboard and tracks which value was last copied.
 * Returns [copiedValue, copy] where copiedValue is the last copied string
 * (or null after timeout) and copy() triggers clipboard write.
 */
export function useCopyFeedback(): [string | null, (text: string) => void] {
  const [copied, setCopied] = useState<string | null>(null)
  const timerRef = useRef<number | null>(null)

  useEffect(() => {
    return () => {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current)
      }
    }
  }, [])

  const copy = useCallback((text: string) => {
    void copyText(text).then((ok) => {
      if (!ok) return
      setCopied(text)
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current)
      }
      timerRef.current = window.setTimeout(() => {
        setCopied(null)
        timerRef.current = null
      }, COPY_FEEDBACK_MS)
    })
  }, [])

  return [copied, copy]
}

export async function copyText(text: string): Promise<boolean> {
  if (navigator.clipboard) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      return fallbackCopy(text)
    }
  }

  return fallbackCopy(text)
}

function fallbackCopy(text: string): boolean {
  const textarea = document.createElement("textarea")

  try {
    textarea.value = text
    textarea.style.position = "fixed"
    textarea.style.opacity = "0"
    document.body.appendChild(textarea)
    textarea.select()
    return document.execCommand("copy")
  } catch {
    return false
  } finally {
    document.body.removeChild(textarea)
  }
}
