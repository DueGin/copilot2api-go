import { useEffect, useRef, useState } from "react"

import { copyText } from "../clipboard"

const COPY_FEEDBACK_MS = 1500

interface CopyableSecretProps {
  idleLabel: string
  copiedLabel: string
  secret: string
  maskedSecret: string
  visible: boolean
  copyTitle?: string
}

export function CopyableSecret({
  idleLabel,
  copiedLabel,
  secret,
  maskedSecret,
  visible,
  copyTitle = "Click to copy",
}: CopyableSecretProps) {
  const [copied, setCopied] = useState(false)
  const resetTimerRef = useRef<number | null>(null)

  useEffect(() => {
    return () => {
      if (resetTimerRef.current !== null) {
        window.clearTimeout(resetTimerRef.current)
      }
    }
  }, [])

  const handleCopy = async () => {
    if (!secret) {
      return
    }

    const didCopy = await copyText(secret)
    if (!didCopy) {
      return
    }

    setCopied(true)

    if (resetTimerRef.current !== null) {
      window.clearTimeout(resetTimerRef.current)
    }

    resetTimerRef.current = window.setTimeout(() => {
      setCopied(false)
      resetTimerRef.current = null
    }, COPY_FEEDBACK_MS)
  }

  return (
    <>
      <span style={{ color: "var(--text-muted)", flexShrink: 0 }}>
        {copied ? copiedLabel : idleLabel}
      </span>
      <button
        type="button"
        onClick={() => void handleCopy()}
        title={copyTitle}
        style={{
          background: "none",
          border: "none",
          borderRadius: 0,
          cursor: "pointer",
          flex: 1,
          font: "inherit",
          padding: 0,
          textAlign: "left",
          color: copied ? "var(--green)" : undefined,
        }}
      >
        {visible ? secret : maskedSecret}
      </button>
    </>
  )
}
