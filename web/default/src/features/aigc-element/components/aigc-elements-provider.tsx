/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import React, { useState } from 'react'
import useDialogState from '@/hooks/use-dialog'
import type { AigcElement, AigcElementsDialogType } from '../types'

type AigcElementsContextType = {
  open: AigcElementsDialogType | null
  setOpen: (str: AigcElementsDialogType | null) => void
  currentRow: AigcElement | null
  setCurrentRow: React.Dispatch<React.SetStateAction<AigcElement | null>>
  // When true, list every user's elements (admin view).
  showAll: boolean
  setShowAll: React.Dispatch<React.SetStateAction<boolean>>
  refreshTrigger: number
  triggerRefresh: () => void
}

const AigcElementsContext =
  React.createContext<AigcElementsContextType | null>(null)

export function AigcElementsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<AigcElementsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<AigcElement | null>(null)
  const [showAll, setShowAll] = useState(false)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <AigcElementsContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        showAll,
        setShowAll,
        refreshTrigger,
        triggerRefresh,
      }}
    >
      {children}
    </AigcElementsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useAigcElements = () => {
  const ctx = React.useContext(AigcElementsContext)
  if (!ctx) {
    throw new Error(
      'useAigcElements has to be used within <AigcElementsProvider>'
    )
  }
  return ctx
}
