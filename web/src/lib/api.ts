const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8091/api'

export type Conversation = {
  id: string
  title: string
  archived: boolean
  createdAt: string
  updatedAt: string
}

export type Message = {
  id: string
  conversationId: string
  role: 'user' | 'assistant' | 'system'
  content: string
  userContent?: string
  llmContent?: string
  attachments?: string[]
  thinking?: string
  hasError?: boolean
  elapsedMs?: number
  createdAt: string
}

function normalizeCreateMessageError(rawMessage: string, includesFiles: boolean): string {
  const message = rawMessage.trim()
  const lower = message.toLowerCase()
  if (includesFiles) {
    if (lower.includes('request body too large') || lower.includes('payload too large') || lower.includes('too large')) {
      return message
    }
    if (lower.includes('failed to fetch') || lower.includes('networkerror')) {
      return 'Upload failed. One or more files may be too large for the server upload limit.'
    }
  }
  return message || 'Failed to send message'
}

export async function createConversation(): Promise<Conversation> {
  const response = await fetch(`${API_BASE}/conversations`, {
    method: 'POST',
  })
  if (!response.ok) {
    throw new Error('Failed to create conversation')
  }
  return response.json()
}

export async function listConversations(includeArchived = false): Promise<Conversation[]> {
  const params = new URLSearchParams()
  if (includeArchived) {
    params.set('includeArchived', '1')
  }
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const response = await fetch(`${API_BASE}/conversations${suffix}`)
  if (!response.ok) {
    throw new Error('Failed to load conversations')
  }
  const data = (await response.json()) as { items: Conversation[] }
  return data.items
}

export async function listMessages(conversationId: string): Promise<Message[]> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}/messages`)
  if (!response.ok) {
    throw new Error('Failed to load messages')
  }
  const data = (await response.json()) as { items: Message[] }
  return data.items
}

export async function createMessage(conversationId: string, content: string, files: File[] = []): Promise<void> {
  let response: Response
  try {
    if (files.length > 0) {
      const formData = new FormData()
      formData.set('content', content)
      for (const file of files) {
        formData.append('files', file)
      }
      response = await fetch(`${API_BASE}/conversations/${conversationId}/messages`, {
        method: 'POST',
        body: formData,
      })
    } else {
      response = await fetch(`${API_BASE}/conversations/${conversationId}/messages`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ content }),
      })
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : 'Failed to send message'
    throw new Error(normalizeCreateMessageError(message, files.length > 0))
  }
  if (!response.ok) {
    let message = 'Failed to send message'
    try {
      const data = (await response.json()) as { error?: string }
      if (data.error) {
        message = data.error
      }
    } catch {
      // Keep default message when response is not JSON.
    }
    throw new Error(normalizeCreateMessageError(message, files.length > 0))
  }
}

export async function createFailedMessage(
  conversationId: string,
  content: string,
  attachments: string[] = [],
): Promise<Message> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}/messages/failed`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      content,
      attachments,
    }),
  })
  if (!response.ok) {
    throw new Error('Failed to persist failed message')
  }
  return response.json()
}

export async function requeueMessage(
  conversationId: string,
  messageId: string,
): Promise<{ userMessage: Message; assistantMessageId: string }> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}/messages/${messageId}/requeue`, {
    method: 'POST',
  })
  if (!response.ok) {
    let message = 'Failed to requeue message'
    try {
      const data = (await response.json()) as { error?: string }
      if (data.error) {
        message = data.error
      }
    } catch {
      // Keep default message when response is not JSON.
    }
    throw new Error(message)
  }
  return response.json()
}

export function conversationStreamUrl(conversationId: string): string {
  const httpUrl = `${API_BASE}/conversations/${conversationId}/stream`
  try {
    const url = new URL(httpUrl)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    return url.toString()
  } catch {
    return httpUrl.replace(/^http/i, 'ws')
  }
}

export async function renameConversation(conversationId: string, title: string): Promise<void> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}`, {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ title }),
  })
  if (!response.ok) {
    throw new Error('Failed to rename conversation')
  }
}

export async function suggestConversationTitle(conversationId: string): Promise<string> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}/suggest-title`, {
    method: 'POST',
  })
  if (!response.ok) {
    throw new Error('Failed to suggest conversation title')
  }
  const data = (await response.json()) as { title?: string }
  if (!data.title) {
    throw new Error('Title suggestion was empty')
  }
  return data.title
}

export async function archiveConversation(conversationId: string): Promise<void> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}/archive`, {
    method: 'PATCH',
  })
  if (!response.ok) {
    throw new Error('Failed to archive conversation')
  }
}

export async function restoreConversation(conversationId: string): Promise<void> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}/restore`, {
    method: 'PATCH',
  })
  if (!response.ok) {
    throw new Error('Failed to restore conversation')
  }
}

export async function deleteConversation(conversationId: string): Promise<void> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}`, {
    method: 'DELETE',
  })
  if (!response.ok) {
    throw new Error('Failed to delete conversation')
  }
}

export async function stopConversationGeneration(conversationId: string): Promise<void> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}/stop`, {
    method: 'POST',
  })
  if (!response.ok && response.status !== 409) {
    throw new Error('Failed to stop generation')
  }
}

export type Settings = {
  llm_url: string
  llm_model: string
  system_prompt: string
}

export async function getSettings(): Promise<Settings> {
  const response = await fetch(`${API_BASE}/settings`)
  if (!response.ok) {
    throw new Error('Failed to load settings')
  }
  return response.json()
}

export async function updateSettings(settings: Partial<Settings>): Promise<Settings> {
  const response = await fetch(`${API_BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  if (!response.ok) {
    throw new Error('Failed to save settings')
  }
  return response.json()
}

export function messageAttachmentDownloadUrl(
  conversationId: string,
  messageId: string,
  attachmentIndex: number,
): string {
  return `${API_BASE}/conversations/${conversationId}/messages/${messageId}/attachments/${attachmentIndex}/download`
}
