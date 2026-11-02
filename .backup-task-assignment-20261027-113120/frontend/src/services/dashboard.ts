import type { Priority, Project, Task, TaskStatus } from "../types/task.types"

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "")

type DashboardResponse = { tasks: Array<{ id: string; title: string; project: string; projectColor: string; assignees: string[]; due: string; priority: string; status: string }>; projects: Array<{ name: string; code: string; color: string; progress: number; completed: number; total: number; members: string[]; due: string }> }

function priority(value: string): Priority { return (["Urgent", "High", "Medium", "Low"].includes(value) ? value : "Medium") as Priority }
function status(value: string): TaskStatus { return (["In progress", "In review", "To do", "Completed"].includes(value) ? value : "To do") as TaskStatus }
function cssColor(value: string): string {
  if (value.startsWith("#")) return value
  const colors: Record<string, string> = { violet: "#7C6CF2", blue: "#3B82F6", orange: "#F59E0B" }
  return colors[value] || "#64748B"
}

export async function fetchDashboard(token: string): Promise<{ tasks: Task[]; projects: Project[] }> {
  const response = await fetch(`${API_BASE_URL}/api/dashboard`, { headers: { Authorization: `Bearer ${token}` } })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    const message = typeof body.error === "string" ? body.error : `Dashboard request failed (${response.status})`
    throw new Error(message)
  }
  const data = body as DashboardResponse
  return {
    tasks: (data.tasks || []).map((item) => ({ ...item, projectColor: cssColor(item.projectColor), priority: priority(item.priority), status: status(item.status) })),
    projects: (data.projects || []).map((item) => ({ ...item, color: cssColor(item.color) })),
  }
}

export async function createTask(token: string, input: { title: string; project: string; priority: Priority; dueDate: string }): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/api/tasks`, {
    method: "POST",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
    body: JSON.stringify(input),
  })
  const body = await response.json().catch(() => ({})) as { error?: string }
  if (!response.ok) throw new Error(body.error || `Could not create task (${response.status})`)
}
