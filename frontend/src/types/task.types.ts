export type TaskStatus = 'In progress' | 'In review' | 'To do' | 'Completed' | 'Changes requested' | 'Blocked' | 'Cancelled'
export type Priority = 'Urgent' | 'High' | 'Medium' | 'Low'

export type Task = {
  id: string
  title: string
  project: string
  projectColor: string
  assignees: string[]
  due: string
  priority: Priority
  status: TaskStatus
  myWorkStatus?: 'assigned' | 'in_progress' | 'ready_for_review' | ''
  reviewFeedback?: string
  reviewDecision?: string
  overdue?: boolean
}

export type Project = {
  name: string
  code: string
  color: string
  progress: number
  completed: number
  total: number
  members: string[]
  due: string
}
