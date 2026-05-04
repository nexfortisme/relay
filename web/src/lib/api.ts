const API_BASE = import.meta.env.DEV
  ? (import.meta.env.VITE_API_BASE_DEV ?? "http://localhost:8091/api")
  : (import.meta.env.VITE_API_BASE ?? "/api");

export type Conversation = {
  id: string;
  title: string;
  archived: boolean;
  createdAt: string;
  updatedAt: string;
};

export type MessageFile = {
  id: string;
  name: string;
};

export type Message = {
  id: string;
  conversationId: string;
  role: "user" | "assistant" | "system";
  content: string;
  userContent?: string;
  llmContent?: string;
  attachments?: MessageFile[];
  thinking?: string;
  hasError?: boolean;
  elapsedMs?: number;
  inputTokens?: number;
  outputTokens?: number;
  reasoningTokens?: number;
  totalTokens?: number;
  createdAt: string;
};

function normalizeCreateMessageError(rawMessage: string, includesFiles: boolean): string {
  const message = rawMessage.trim();
  const lower = message.toLowerCase();
  if (includesFiles) {
    if (
      lower.includes("request body too large") ||
      lower.includes("payload too large") ||
      lower.includes("too large")
    ) {
      return message;
    }
    if (lower.includes("failed to fetch") || lower.includes("networkerror")) {
      return "Upload failed. One or more files may be too large for the server upload limit.";
    }
  }
  return message || "Failed to send message";
}

function apiPath(path: string): string {
  return `${API_BASE}${path}`;
}

async function responseErrorMessage(response: Response, fallback: string): Promise<string> {
  try {
    const data = (await response.json()) as { error?: string };
    return data.error || fallback;
  } catch {
    return fallback;
  }
}

// Single source of truth for "we just got 401"; the auth store subscribes to
// this so navigation back to /login happens once even if many requests fail.
type UnauthorizedHandler = () => void;
let unauthorizedHandler: UnauthorizedHandler | null = null;
export function setUnauthorizedHandler(handler: UnauthorizedHandler | null): void {
  unauthorizedHandler = handler;
}

// Paths that should not trigger the unauthorized handler — they ARE the
// auth flow, and a 401 there is expected (bad password, expired refresh on
// page load, etc.).
const authPathPrefix = "/auth/";

function withCredentials(init: RequestInit | undefined): RequestInit {
  return { credentials: "include", ...init };
}

async function apiFetch(path: string, init?: RequestInit): Promise<Response> {
  const response = await fetch(apiPath(path), withCredentials(init));
  if (response.status === 401 && !path.startsWith(authPathPrefix) && unauthorizedHandler) {
    unauthorizedHandler();
  }
  return response;
}

async function fetchJson<T>(
  path: string,
  init: RequestInit | undefined,
  errorMessage: string,
): Promise<T> {
  const response = await apiFetch(path, init);
  if (!response.ok) {
    throw new Error(await responseErrorMessage(response, errorMessage));
  }
  return response.json();
}

async function fetchNoContent(
  path: string,
  init: RequestInit | undefined,
  errorMessage: string,
): Promise<void> {
  const response = await apiFetch(path, init);
  if (!response.ok) {
    throw new Error(await responseErrorMessage(response, errorMessage));
  }
}

export type AuthUser = {
  id: string;
  username: string;
};

export async function authMe(): Promise<AuthUser> {
  return fetchJson<AuthUser>("/auth/me", undefined, "Not authenticated");
}

export async function authLogin(
  username: string,
  password: string,
  rememberMe: boolean,
): Promise<AuthUser> {
  return fetchJson<AuthUser>(
    "/auth/login",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password, rememberMe }),
    },
    "Login failed",
  );
}

export async function authRegister(
  username: string,
  password: string,
  rememberMe: boolean,
): Promise<AuthUser> {
  return fetchJson<AuthUser>(
    "/auth/register",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password, rememberMe }),
    },
    "Registration failed",
  );
}

export async function authLogout(): Promise<void> {
  await fetchNoContent("/auth/logout", { method: "POST" }, "Logout failed");
}

export async function authRefresh(): Promise<AuthUser | null> {
  const response = await fetch(apiPath("/auth/refresh"), withCredentials({ method: "POST" }));
  if (response.status === 401) {
    return null;
  }
  if (!response.ok) {
    throw new Error(await responseErrorMessage(response, "Refresh failed"));
  }
  return response.json();
}

export async function createConversation(): Promise<Conversation> {
  return fetchJson<Conversation>(
    "/conversations",
    { method: "POST" },
    "Failed to create conversation",
  );
}

export async function listConversations(includeArchived = false): Promise<Conversation[]> {
  const params = new URLSearchParams();
  if (includeArchived) {
    params.set("includeArchived", "1");
  }
  const suffix = params.toString() ? `?${params.toString()}` : "";
  const data = await fetchJson<{ items: Conversation[] }>(
    `/conversations${suffix}`,
    undefined,
    "Failed to load conversations",
  );
  return data.items;
}

export async function listMessages(conversationId: string): Promise<Message[]> {
  const data = await fetchJson<{ items: Message[] }>(
    `/conversations/${conversationId}/messages`,
    undefined,
    "Failed to load messages",
  );
  return data.items;
}

export async function createMessage(
  conversationId: string,
  content: string,
  files: File[] = [],
): Promise<void> {
  let response: Response;
  try {
    if (files.length > 0) {
      const formData = new FormData();
      formData.set("content", content);
      for (const file of files) {
        formData.append("files", file);
      }
      response = await apiFetch(`/conversations/${conversationId}/messages`, {
        method: "POST",
        body: formData,
      });
    } else {
      response = await apiFetch(`/conversations/${conversationId}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ content }),
      });
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : "Failed to send message";
    throw new Error(normalizeCreateMessageError(message, files.length > 0));
  }
  if (!response.ok) {
    const message = await responseErrorMessage(response, "Failed to send message");
    throw new Error(normalizeCreateMessageError(message, files.length > 0));
  }
}

export async function createFailedMessage(
  conversationId: string,
  content: string,
  attachments: string[] = [],
): Promise<Message> {
  return fetchJson<Message>(
    `/conversations/${conversationId}/messages/failed`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        content,
        attachments,
      }),
    },
    "Failed to persist failed message",
  );
}

export async function requeueMessage(
  conversationId: string,
  messageId: string,
): Promise<{ userMessage: Message; assistantMessageId: string }> {
  return fetchJson<{ userMessage: Message; assistantMessageId: string }>(
    `/conversations/${conversationId}/messages/${messageId}/requeue`,
    {
      method: "POST",
    },
    "Failed to requeue message",
  );
}

export function conversationStreamUrl(conversationId: string): string {
  const httpUrl = `${API_BASE}/conversations/${conversationId}/stream`;
  try {
    const url = new URL(httpUrl);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    return url.toString();
  } catch {
    return httpUrl.replace(/^http/i, "ws");
  }
}

export async function renameConversation(conversationId: string, title: string): Promise<void> {
  await fetchNoContent(
    `/conversations/${conversationId}`,
    {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ title }),
    },
    "Failed to rename conversation",
  );
}

export async function suggestConversationTitle(conversationId: string): Promise<string> {
  const data = await fetchJson<{ title?: string }>(
    `/conversations/${conversationId}/suggest-title`,
    { method: "POST" },
    "Failed to suggest conversation title",
  );
  if (!data.title) {
    throw new Error("Title suggestion was empty");
  }
  return data.title;
}

export async function archiveConversation(conversationId: string): Promise<void> {
  await fetchNoContent(
    `/conversations/${conversationId}/archive`,
    { method: "PATCH" },
    "Failed to archive conversation",
  );
}

export async function restoreConversation(conversationId: string): Promise<void> {
  await fetchNoContent(
    `/conversations/${conversationId}/restore`,
    { method: "PATCH" },
    "Failed to restore conversation",
  );
}

export async function deleteConversation(conversationId: string): Promise<void> {
  await fetchNoContent(
    `/conversations/${conversationId}`,
    { method: "DELETE" },
    "Failed to delete conversation",
  );
}

export async function stopConversationGeneration(conversationId: string): Promise<void> {
  const response = await apiFetch(`/conversations/${conversationId}/stop`, {
    method: "POST",
  });
  if (!response.ok && response.status !== 409) {
    throw new Error(await responseErrorMessage(response, "Failed to stop generation"));
  }
}

export type Settings = {
  llm_url: string;
  llm_model: string;
  llm_api_key: string;
  system_prompt: string;
};

export async function getSettings(): Promise<Settings> {
  return fetchJson<Settings>("/settings", undefined, "Failed to load settings");
}

export async function updateSettings(settings: Partial<Settings>): Promise<Settings> {
  return fetchJson<Settings>(
    "/settings",
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(settings),
    },
    "Failed to save settings",
  );
}

export function fileDownloadUrl(fileId: string): string {
  return `${API_BASE}/files/${fileId}/download`;
}
