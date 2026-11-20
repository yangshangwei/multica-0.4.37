"use client"

import { createContext, useContext, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from "react"

const PopupVisibilityContext = createContext<boolean | null>(null)

/** A retained, hidden surface must also close the popups it portals to body. */
export function PopupVisibilityProvider({ visible, children }: { visible: boolean; children: ReactNode }) {
  const parentVisible = useContext(PopupVisibilityContext)
  return <PopupVisibilityContext.Provider value={parentVisible !== false && visible}>{children}</PopupVisibilityContext.Provider>
}

export function usePopupVisibility() {
  return useContext(PopupVisibilityContext) !== false
}

/** Select has no native close action, so only explicit scopes need an adapter. */
export function useHasPopupVisibilityScope() {
  return useContext(PopupVisibilityContext) !== null
}

interface PopupActions {
  close(): void
  unmount(): void
}

interface ChangeDetails {
  cancel(): void
  readonly isCanceled: boolean
}

/** Base UI remains the owner of open state and produces the close event details. */
export function useScopedPopup<Details extends ChangeDetails>({
  open,
  defaultOpen,
  actionsRef,
  onOpenChange,
}: {
  open?: boolean
  defaultOpen?: boolean
  actionsRef?: RefObject<PopupActions | null>
  onOpenChange?: (open: boolean, details: Details) => void
}) {
  const visible = usePopupVisibility()
  const localActionsRef = useRef<PopupActions | null>(null)
  const resolvedActionsRef = actionsRef ?? localActionsRef
  const lastOpenRef = useRef(open ?? defaultOpen ?? false)
  const initialDefaultOpen = useRef(visible ? defaultOpen : false)
  const [dismissed, setDismissed] = useState(false)

  useLayoutEffect(() => {
    if (open !== undefined) lastOpenRef.current = open
    if (!visible && lastOpenRef.current) {
      // A controlled owner may decline closure during a save. Keep that popup
      // suppressed until it acknowledges false or a fresh trigger opens it.
      if (open) setDismissed(true)
      resolvedActionsRef.current?.close()
      lastOpenRef.current = false
    }
    if (open === false) setDismissed(false)
  }, [visible, open, resolvedActionsRef])

  return {
    // Never turn an uncontrolled root into a controlled one mid-lifecycle.
    // Imperative close clears its native store; hidden portal DOM is inert
    // during the same commit, before that native transition settles.
    open: open === undefined ? undefined : !visible || dismissed ? false : open,
    defaultOpen: initialDefaultOpen.current,
    actionsRef: resolvedActionsRef,
    onOpenChange(nextOpen: boolean, details: Details) {
      if (nextOpen && !visible) {
        details.cancel()
        return
      }
      onOpenChange?.(nextOpen, details)
      if (details.isCanceled) return
      if (nextOpen) setDismissed(false)
      lastOpenRef.current = nextOpen
    },
  }
}
