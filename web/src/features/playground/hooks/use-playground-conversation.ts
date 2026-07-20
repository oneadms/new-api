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
import { useCallback, useState } from 'react'

import {
  appendUserMessagePair,
  applyMessageEdit,
  createRegeneratedMessages,
  removeMessageByKey,
} from '../lib'
import type { Message, PlaygroundMode, PlaygroundSubmitInput } from '../types'

type UsePlaygroundConversationOptions = {
  messages: Message[]
  updateMessages: (
    updater: Message[] | ((prev: Message[]) => Message[])
  ) => void
  sendChat: (messages: Message[]) => void
  sendImageGeneration: (prompt: string, imageReferenceUrls?: string[]) => void
  mode: PlaygroundMode
}

export function usePlaygroundConversation({
  messages,
  updateMessages,
  sendChat,
  sendImageGeneration,
  mode,
}: UsePlaygroundConversationOptions) {
  const [editingMessageKey, setEditingMessageKey] = useState<string | null>(
    null
  )

  const dispatchMessages = useCallback(
    (nextMessages: Message[]) => {
      const userMessage = [...nextMessages]
        .reverse()
        .find((message) => message.from === 'user')
      if (userMessage?.mode === 'image') {
        sendImageGeneration(
          userMessage.versions[0]?.content || '',
          userMessage.imageReferenceUrls || []
        )
        return
      }
      sendChat(nextMessages)
    },
    [sendChat, sendImageGeneration]
  )

  const handleSendMessage = useCallback(
    (input: PlaygroundSubmitInput | string) => {
      const normalizedInput =
        typeof input === 'string' ? { text: input } : input
      const nextMessages = appendUserMessagePair(
        messages,
        normalizedInput.text,
        {
          mode,
          imageReferenceUrls:
            mode === 'image' ? normalizedInput.imageReferenceUrls : undefined,
        }
      )
      updateMessages(nextMessages)
      dispatchMessages(nextMessages)
    },
    [dispatchMessages, messages, mode, updateMessages]
  )

  const handleRegenerateMessage = useCallback(
    (message: Message) => {
      const nextMessages = createRegeneratedMessages(messages, message.key)
      if (!nextMessages) return

      updateMessages(nextMessages)
      dispatchMessages(nextMessages)
    },
    [dispatchMessages, messages, updateMessages]
  )

  const handleEditMessage = useCallback((message: Message) => {
    setEditingMessageKey(message.key)
  }, [])

  const handleEditOpenChange = useCallback((open: boolean) => {
    if (!open) {
      setEditingMessageKey(null)
    }
  }, [])

  const applyEdit = useCallback(
    (newContent: string, shouldSubmit: boolean) => {
      if (!editingMessageKey) return

      const editResult = applyMessageEdit(
        messages,
        editingMessageKey,
        newContent,
        shouldSubmit
      )
      if (!editResult) return

      setEditingMessageKey(null)
      updateMessages(editResult.messages)

      if (editResult.shouldSend) {
        dispatchMessages(editResult.messages)
      }
    },
    [dispatchMessages, editingMessageKey, messages, updateMessages]
  )

  const handleDeleteMessage = useCallback(
    (message: Message) => {
      updateMessages((previousMessages) =>
        removeMessageByKey(previousMessages, message.key)
      )
    },
    [updateMessages]
  )

  return {
    editingMessageKey,
    handleSendMessage,
    handleRegenerateMessage,
    handleEditMessage,
    handleEditOpenChange,
    applyEdit,
    handleDeleteMessage,
  }
}
