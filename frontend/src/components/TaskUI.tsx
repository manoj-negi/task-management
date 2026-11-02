import { ArrowDownRight, ArrowUpRight } from 'lucide-react'
import type { TaskStatus, Priority } from '../types/task.types'

export function Avatar({
  initials,
  index = 0,
}: {
  initials: string
  index?: number
}) {
  return <span className={`avatar avatar-${index % 5}`}>{initials}</span>
}

export function StatusBadge({ status }: { status: TaskStatus }) {
  return (
    <span className={`status-badge status-${status.toLowerCase().replaceAll(' ', '-')}`}>
      <span className="status-dot" />
      {status}
    </span>
  )
}

export function PriorityLabel({ priority }: { priority: Priority }) {
  const icon =
    priority === 'Urgent' || priority === 'High'
      ? 'up'
      : priority === 'Low'
        ? 'down'
        : 'flat'

  return (
    <span className={`priority priority-${priority.toLowerCase()}`}>
      {icon === 'up' ? (
        <ArrowUpRight size={14} />
      ) : icon === 'down' ? (
        <ArrowDownRight size={14} />
      ) : (
        <span className="priority-line" />
      )}
      {priority}
    </span>
  )
}
