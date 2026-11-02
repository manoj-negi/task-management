const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "")

export type AdminTaskDetails = {
  id: string
  title: string
  description: string
  projectId: string
  project: string
  priority: "low" | "medium" | "high" | "urgent"
  status: string
  startDate: string
  dueDate: string
  assignees: string[]
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    credentials: "include",
    headers: { ...(init.body ? { "Content-Type": "application/json" } : {}), ...init.headers },
  })
  const body = await response.json().catch(() => ({})) as { error?: string }
  if (!response.ok) throw new Error(body.error || `Request failed (${response.status})`)
  return body as T
}

export function fetchAdminTaskDetails(id: string): Promise<AdminTaskDetails> {
  return request(`/api/admin/tasks/${encodeURIComponent(id)}/details`)
}

export function updateAdminTask(id: string, input: {
  title: string; description: string; project: string; priority: AdminTaskDetails["priority"]; status: string; startDate: string; dueDate: string; assignedUserIds: string[]
}): Promise<{ message: string }> {
  return request(`/api/admin/tasks/${encodeURIComponent(id)}/manage`, { method: "POST", body: JSON.stringify(input) })
}

export function deleteAdminTask(id: string): Promise<{ message: string }> {
  return request(`/api/admin/tasks/${encodeURIComponent(id)}/delete`, { method: "POST", body: JSON.stringify({}) })
}

export function createAdminSubtask(parentId: string, input: { title: string; description: string; priority: AdminTaskDetails["priority"]; dueDate: string; assignedUserIds: string[] }) {
  return request<{ id: string; message: string }>(`/api/admin/tasks/${encodeURIComponent(parentId)}/subtasks`, { method: "POST", body: JSON.stringify(input) })
}
