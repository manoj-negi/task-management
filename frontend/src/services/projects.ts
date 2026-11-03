export type ManagedProject = {
  id: string
  name: string
  description: string
  status: 'planned' | 'active' | 'on_hold' | 'completed' | 'archived'
  startDate: string
  dueDate: string
  createdAt: string
  totalTasks: number
  completedTasks: number
  memberCount: number
}

export type ProjectMember = {
  id: string
  fullName: string
  email: string
  isActive: boolean
  joinedAt: string
}

export type ProjectTask = {
  id: string
  title: string
  priority: string
  status: string
  due: string
  assignees: string[]
}

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080').replace(/\/$/, '')

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    credentials: 'include',
    headers: {
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
  })
  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(typeof body.error === 'string' ? body.error : `Request failed (${response.status})`)
  }
  return body as T
}

export const fetchProjects = () => request<ManagedProject[]>('/api/projects')
export const fetchProjectMembers = (id: string) => request<ProjectMember[]>(`/api/projects/${encodeURIComponent(id)}/members`)
export const fetchProjectTasks = (id: string, page = 1, pageSize = 10) => {
  const offset = (page - 1) * pageSize
  return request<{ tasks: ProjectTask[]; total: number; limit: number; offset: number }>(
    `/api/projects/${encodeURIComponent(id)}/tasks?limit=${pageSize}&offset=${offset}`,
  )
}
export const fetchActiveEmployees = async (): Promise<Array<{id:string; full_name:string; email:string; is_active:boolean; role:string}>> => {
  const data = await request<{employees:Array<{id:string; full_name:string; email:string; is_active:boolean; role:string}>}>('/api/admin/employees')
  return data.employees.filter((employee) => employee.is_active && employee.role === 'employee')
}
export const saveProject = (input: {id?:string; name:string; description:string; status:string; startDate:string; dueDate:string}) =>
  request<ManagedProject>(input.id ? `/api/projects/${encodeURIComponent(input.id)}` : '/api/projects', {
    method: 'POST',
    body: JSON.stringify({name:input.name, description:input.description, status:input.status, startDate:input.startDate, dueDate:input.dueDate}),
  })
export const archiveProject = (id:string) => request<{ok:boolean}>(`/api/projects/${encodeURIComponent(id)}/archive`, {method:'POST',body:JSON.stringify({})})
export const deleteProject = (id:string) => request<{ok:boolean}>(`/api/projects/${encodeURIComponent(id)}/delete`, {method:'POST',body:JSON.stringify({})})
export const addProjectMember = (projectId:string, userId:string) => request<{ok:boolean}>(`/api/projects/${encodeURIComponent(projectId)}/members/add`, {method:'POST',body:JSON.stringify({userId})})
export const removeProjectMember = (projectId:string, userId:string) => request<{ok:boolean}>(`/api/projects/${encodeURIComponent(projectId)}/members/remove`, {method:'POST',body:JSON.stringify({userId})})
