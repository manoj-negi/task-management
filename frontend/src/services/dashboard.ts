import type { Priority, Project, Task, TaskStatus } from "../types/task.types"

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "")

export type TeamActivity = { id: string; actorName: string; action: string; taskTitle: string; projectName: string; targetName?: string; createdAt: string }

export async function fetchTeamActivity(): Promise<TeamActivity[]> {
  const response = await fetch(`${API_BASE_URL}/api/activity`, { credentials: "include" })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    const message = typeof body.error === "string" ? body.error : `Activity request failed (${response.status})`
    throw new Error(message)
  }
  return Array.isArray(body) ? body as TeamActivity[] : []
}

export type EmployeeTaskInsight = { id: string; name: string; total: number; inProgress: number; inReview: number; completed: number; overdue: number }
type DashboardResponse = { tasks: Array<{ id: string; title: string; project: string; projectColor: string; assignees: string[]; due: string; priority: string; status: string; overdue?: boolean }>; projects: Array<{ name: string; code: string; color: string; progress: number; completed: number; total: number; members: string[]; due: string }>; employeeStats?: EmployeeTaskInsight[] | null }

function priority(value: string): Priority { return (["Urgent", "High", "Medium", "Low"].includes(value) ? value : "Medium") as Priority }
function status(value: string): TaskStatus { return (["In progress", "In review", "To do", "Completed", "Changes requested", "Blocked", "Cancelled"].includes(value) ? value : "To do") as TaskStatus }
function cssColor(value: string): string {
  if (value.startsWith("#")) return value
  const colors: Record<string, string> = { violet: "#7C6CF2", blue: "#3B82F6", orange: "#F59E0B" }
  return colors[value] || "#64748B"
}

export async function fetchDashboard(): Promise<{ tasks: Task[]; projects: Project[]; employeeStats: EmployeeTaskInsight[] }> {
  const response = await fetch(`${API_BASE_URL}/api/dashboard`, { credentials: "include" })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    const message = typeof body.error === "string" ? body.error : `Dashboard request failed (${response.status})`
    throw new Error(message)
  }
  const data = body as DashboardResponse
  return {
    tasks: (data.tasks || []).map((item) => ({ ...item, projectColor: cssColor(item.projectColor), priority: priority(item.priority), status: status(item.status) })),
    projects: (data.projects || []).map((item) => ({ ...item, color: cssColor(item.color) })),
    employeeStats: data.employeeStats || [],
  }
}

export type AssignableUser = { id: string; fullName: string; email: string }

export async function fetchAssignableUsers(project: string): Promise<AssignableUser[]> {
  const response = await fetch(`${API_BASE_URL}/api/assignable-users?project=${encodeURIComponent(project)}`, {
    credentials: "include",
  })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    const message = typeof body.error === "string" ? body.error : `Could not load employees (${response.status})`
    throw new Error(message)
  }
  return Array.isArray(body) ? body as AssignableUser[] : []
}

export async function createTask(input: { title: string; description: string; project: string; priority: Priority; dueDate: string; assignedUserIds: string[] }): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/api/tasks`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  })
  const body = await response.json().catch(() => ({})) as { error?: string }
  if (!response.ok) throw new Error(body.error || `Could not create task (${response.status})`)
}
