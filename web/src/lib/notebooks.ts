const API_BASE = import.meta.env.DEV
  ? (import.meta.env.VITE_API_BASE_DEV ?? 'http://localhost:8091/api')
  : (import.meta.env.VITE_API_BASE ?? '/api')

export type Notebook = {
  id: string
  name: string
  description: string
  systemPrompt: string
  pendingJobs: number
  createdAt: string
  updatedAt: string
}

export type NotebookFile = {
  id: string
  notebookId: string
  name: string
  contentType: string
  sizeBytes: number
  fileKind: 'document' | 'csv' | 'image'
  status: 'pending' | 'processing' | 'ready' | 'error'
  error?: string
  createdAt: string
  updatedAt: string
}

export type NotebookConversation = {
  id: string
  title: string
  notebookId: string
  archived: boolean
  createdAt: string
  updatedAt: string
}

export type CSVTableData = {
  columns: string[]
  rows: string[][]
}

function apiPath(path: string): string {
  return `${API_BASE}${path}`
}

function withCreds(init?: RequestInit): RequestInit {
  return { credentials: 'include', ...init }
}

async function responseErrorMessage(response: Response, fallback: string): Promise<string> {
  try {
    const data = (await response.json()) as { error?: string }
    return data.error || fallback
  } catch {
    return fallback
  }
}

async function fetchJson<T>(path: string, init: RequestInit | undefined, errorMessage: string): Promise<T> {
  const response = await fetch(apiPath(path), withCreds(init))
  if (!response.ok) {
    throw new Error(await responseErrorMessage(response, errorMessage))
  }
  return response.json()
}

async function fetchNoContent(path: string, init: RequestInit | undefined, errorMessage: string): Promise<void> {
  const response = await fetch(apiPath(path), withCreds(init))
  if (!response.ok) {
    throw new Error(await responseErrorMessage(response, errorMessage))
  }
}

// Notebooks CRUD

export async function listNotebooks(): Promise<Notebook[]> {
  const data = await fetchJson<{ items: Notebook[] }>('/notebooks', undefined, 'Failed to list notebooks')
  return data.items ?? []
}

export async function createNotebook(payload: {
  name: string
  description?: string
  systemPrompt?: string
}): Promise<Notebook> {
  return fetchJson<Notebook>(
    '/notebooks',
    { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) },
    'Failed to create notebook',
  )
}

export async function getNotebook(id: string): Promise<Notebook> {
  return fetchJson<Notebook>(`/notebooks/${id}`, undefined, 'Failed to get notebook')
}

export async function updateNotebook(
  id: string,
  patch: Partial<{ name: string; description: string; systemPrompt: string }>,
): Promise<Notebook> {
  return fetchJson<Notebook>(
    `/notebooks/${id}`,
    { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(patch) },
    'Failed to update notebook',
  )
}

export async function deleteNotebook(id: string): Promise<void> {
  return fetchNoContent(`/notebooks/${id}`, { method: 'DELETE' }, 'Failed to delete notebook')
}

// Files

export async function listNotebookFiles(notebookId: string): Promise<NotebookFile[]> {
  const data = await fetchJson<{ items: NotebookFile[] }>(
    `/notebooks/${notebookId}/files`,
    undefined,
    'Failed to list notebook files',
  )
  return data.items ?? []
}

export function uploadNotebookFile(
  notebookId: string,
  file: File,
  onProgress?: (pct: number) => void,
): Promise<NotebookFile> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const fd = new FormData()
    fd.append('file', file)

    xhr.open('POST', apiPath(`/notebooks/${notebookId}/files`))
    xhr.withCredentials = true

    if (onProgress) {
      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          onProgress(Math.round((e.loaded / e.total) * 100))
        }
      })
    }

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText) as NotebookFile)
        } catch {
          reject(new Error('Invalid response from server'))
        }
      } else {
        try {
          const data = JSON.parse(xhr.responseText) as { error?: string }
          reject(new Error(data.error ?? 'Upload failed'))
        } catch {
          reject(new Error('Upload failed'))
        }
      }
    }

    xhr.onerror = () => reject(new Error('Network error during upload'))
    xhr.send(fd)
  })
}

export async function deleteNotebookFile(notebookId: string, fileId: string): Promise<void> {
  return fetchNoContent(
    `/notebooks/${notebookId}/files/${fileId}`,
    { method: 'DELETE' },
    'Failed to delete notebook file',
  )
}

// Jobs

export async function getPendingJobCount(notebookId: string): Promise<number> {
  const data = await fetchJson<{ count: number }>(
    `/notebooks/${notebookId}/jobs/count`,
    undefined,
    'Failed to get job count',
  )
  return data.count ?? 0
}

// Conversations

export async function listNotebookConversations(notebookId: string): Promise<NotebookConversation[]> {
  const data = await fetchJson<{ items: NotebookConversation[] }>(
    `/notebooks/${notebookId}/conversations`,
    undefined,
    'Failed to list notebook conversations',
  )
  return data.items ?? []
}

export async function createNotebookConversation(notebookId: string): Promise<NotebookConversation> {
  return fetchJson<NotebookConversation>(
    `/notebooks/${notebookId}/conversations`,
    { method: 'POST' },
    'Failed to create notebook conversation',
  )
}

// CSV viewer

export async function getCSVTableData(notebookId: string, fileId: string): Promise<CSVTableData> {
  return fetchJson<CSVTableData>(
    `/notebooks/${notebookId}/csv/${fileId}`,
    undefined,
    'Failed to get CSV data',
  )
}
