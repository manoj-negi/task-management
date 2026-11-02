import { ApiRequestError } from "./auth"

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "")

export type TaskAssigneeDetail = { id: string; fullName: string; email: string; workStatus: string; assignedBy: string; assignedAt: string }
export type TaskSubtask = { id: string; title: string; status: string; workStatus: string; dueDate: string; canUpdate: boolean }
export type TaskAttachment = { id: string; name: string; mediaType: string; size: number; uploader: string; uploadedAt: string; downloadUrl: string }
export type TaskComment = { id: string; parentId?: string; author: string; role: string; comment: string; createdAt: string }
export type TaskTimelineEvent = { id: string; actor: string; action: string; detail?: string; createdAt: string }
export type TaskDetail = {
  id: string; title: string; description: string; project: string; parentTask: string; status: string; priority: string
  startDate: string; dueDate: string; createdBy: string; assignedBy: string; assignedAt: string; createdAt: string; myWorkStatus: string
  reviewFeedback: string; reviewDecision: string; progress: number; subtaskCount: number; subtasksDone: number
}
export type TaskDetailsResponse = {
  task: TaskDetail; assignees: TaskAssigneeDetail[]; subtasks: TaskSubtask[]; attachments: TaskAttachment[]
  latestSubmission: { summary: string; referenceLinks: string[]; attachmentIds: string[]; submittedAt: string }
  submissions?: { id: string; submittedBy: string; summary: string; referenceLinks: string[]; attachmentIds: string[]; submittedAt: string }[]
  reviews?: { decision: string; feedback: string; reviewer: string; createdAt: string }[]
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response
  try {
    response = await fetch(`${API_BASE_URL}${path}`, { ...init, credentials: "include" })
  } catch {
    throw new Error("Cannot connect to the server. Make sure the backend is running.")
  }
  const data = await response.json().catch(() => ({})) as { error?: string }
  if (!response.ok) throw new ApiRequestError(data.error || `Request failed (${response.status})`, response.status)
  return data as T
}

export const fetchTaskDetails = (id: string) => request<TaskDetailsResponse>(`/api/tasks/${encodeURIComponent(id)}/details`)
export async function fetchTaskComments(id: string) {
  const data = await request<{ comments: TaskComment[] }>(`/api/tasks/${encodeURIComponent(id)}/comments?limit=100&offset=0`)
  return data.comments
}
export async function addTaskComment(id: string, comment: string, parentCommentId?: string) {
  return request<{ id: string; message: string }>(`/api/tasks/${encodeURIComponent(id)}/comments`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ comment, parentCommentId }) })
}
export async function fetchTaskActivity(id: string) {
  const data = await request<{ activity: TaskTimelineEvent[] }>(`/api/tasks/${encodeURIComponent(id)}/activity?limit=100&offset=0`)
  return data.activity
}
export async function updateTaskStatus(id: string, status: "blocked" | "in_progress") {
  return request<{ message: string; status: string }>(`/api/tasks/${encodeURIComponent(id)}/status`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ status }) })
}
export async function updateSubtaskStatus(taskId: string, subtaskId: string, status: string, summary?: string) {
  return request<{ message: string; status: string }>(`/api/tasks/${encodeURIComponent(taskId)}/subtasks/${encodeURIComponent(subtaskId)}/status`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ status, summary }) })
}
export async function uploadTaskAttachment(taskId: string, file: File) {
  const body = new FormData()
  body.append("file", file)
  return request<{ attachment: TaskAttachment }>(`/api/tasks/${encodeURIComponent(taskId)}/attachments`, { method: "POST", body })
}
export async function downloadTaskAttachment(taskId: string, attachment: TaskAttachment) {
  let response: Response
  try { response = await fetch(`${API_BASE_URL}/api/tasks/${encodeURIComponent(taskId)}/attachments/${encodeURIComponent(attachment.id)}`, { credentials: "include" }) }
  catch { throw new Error("Cannot connect to the server.") }
  if (!response.ok) {
    const data = await response.json().catch(() => ({})) as { error?: string }
    throw new ApiRequestError(data.error || `Download failed (${response.status})`, response.status)
  }
  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const link = document.createElement("a")
  link.href = url
  link.download = attachment.name
  document.body.append(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

export function reviewAdminTask(taskId: string, decision: "approved" | "changes_requested", feedback: string) {
  return request<{ message: string }>(`/api/admin/tasks/${encodeURIComponent(taskId)}`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ decision, feedback }) })
}
