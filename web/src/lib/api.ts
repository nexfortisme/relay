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
  thinking?: string
  createdAt: string
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

export async function createMessage(conversationId: string, content: string): Promise<void> {
  const response = await fetch(`${API_BASE}/conversations/${conversationId}/messages`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ content }),
  })
  if (!response.ok) {
    throw new Error('Failed to send message')
  }
}

export function conversationStreamUrl(conversationId: string): string {
  const streamHttpUrl = conversationHttpStreamUrl(conversationId)
  try {
    const url = new URL(streamHttpUrl)
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    return url.toString()
  } catch {
    return streamHttpUrl.replace(/^http/i, 'ws')
  }
}

export function conversationHttpStreamUrl(conversationId: string): string {
  return `${API_BASE}/conversations/${conversationId}/stream`
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
