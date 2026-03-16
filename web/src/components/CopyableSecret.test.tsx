import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { beforeEach, describe, expect, it, vi } from "vitest"

import { copyText } from "../clipboard"
import { CopyableSecret } from "./CopyableSecret"

vi.mock("../clipboard", () => ({
  copyText: vi.fn(),
}))

const copyTextMock = vi.mocked(copyText)

describe("CopyableSecret", () => {
  beforeEach(() => {
    copyTextMock.mockReset()
  })

  it("shows copied feedback and green text after a successful copy", async () => {
    copyTextMock.mockResolvedValue(true)

    render(
      <CopyableSecret
        idleLabel="API Key:"
        copiedLabel="Copied!"
        secret="sk-1234567890"
        maskedSecret="sk-1234••••"
        visible={false}
      />,
    )

    const secret = screen.getByText("sk-1234••••")
    fireEvent.click(secret)

    expect(copyTextMock).toHaveBeenCalledWith("sk-1234567890")
    await waitFor(() => {
      expect(screen.getByText("Copied!")).toBeInTheDocument()
      expect(secret).toHaveStyle({ color: "var(--green)" })
    })
  })

  it("keeps the default label and color when copy fails", async () => {
    copyTextMock.mockResolvedValue(false)

    render(
      <CopyableSecret
        idleLabel="Pool Key:"
        copiedLabel="Copied!"
        secret="pool-secret-123"
        maskedSecret="pool-sec••••"
        visible={false}
      />,
    )

    const secret = screen.getByText("pool-sec••••")
    fireEvent.click(secret)

    expect(copyTextMock).toHaveBeenCalledWith("pool-secret-123")
    await waitFor(() => {
      expect(screen.getByText("Pool Key:")).toBeInTheDocument()
      expect(screen.queryByText("Copied!")).not.toBeInTheDocument()
      expect(secret).not.toHaveStyle({ color: "var(--green)" })
    })
  })
})
