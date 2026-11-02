import Sidebar from './layouts/Sidebar'
import AdminApprovals from './components/AdminApprovals'
import EmployeeTaskDetails from './components/EmployeeTaskDetails'
import EmployeeManagement from './components/EmployeeManagement'
import ProjectManagement from './components/ProjectManagement'
import PaginationControls from './components/PaginationControls'
import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react'
import {
  Activity,
  ArrowRight,
  Bell,
  CalendarDays,
  CircleAlert,
  Check,
  CheckCheck,
  Clock3,
  FileText,
  FolderKanban,
  FolderPlus,
  LayoutDashboard,
  ListTodo,
  Menu,
  MessageSquare,
  Plus,
  Pencil,
  RefreshCw,
  Trash2,
  Search,
  ShieldCheck,
  SlidersHorizontal,
  Users,
  X,
} from 'lucide-react'
import './App.css'
import type { Task, Priority } from './types/task.types'
import { createTask as createTaskRequest, fetchAssignableUsers, fetchDashboard, fetchTeamActivity, type AssignableUser, type EmployeeTaskInsight, type TeamActivity } from './services/dashboard'
import { Avatar, StatusBadge, PriorityLabel } from './components/TaskUI'
import { fetchAdminTaskDetails, updateAdminTask, deleteAdminTask, type AdminTaskDetails } from './services/taskManagement'
import type { AuthUser } from './services/auth'

const navGroups = [
  {
    label: 'WORKSPACE',
    items: [
      { label: 'Overview', icon: LayoutDashboard },
      { label: 'Tasks', icon: ListTodo },
      { label: 'Projects', icon: FolderKanban },
      { label: 'Team', icon: Users },
    ],
  },
  {
    label: 'MANAGEMENT',
    items: [
      { label: 'Approvals', icon: ShieldCheck },
      { label: 'Activity', icon: Activity },
      { label: 'Reports', icon: FileText },
    ],
  },
]

function App({ user, onSignOut }: { user: AuthUser; onSignOut: () => void }) {
  const [activeNav, setActiveNav] = useState('Overview')
  const [activeFilter, setActiveFilter] = useState('All tasks')
  const [taskPage, setTaskPage] = useState(1)
  const [projectProgressPage, setProjectProgressPage] = useState(1)
  const [employeeProgressPage, setEmployeeProgressPage] = useState(1)
  const [search, setSearch] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [projectCreateRequest, setProjectCreateRequest] = useState(0)
  const [showNotifications, setShowNotifications] = useState(false)
  const [mobileSidebar, setMobileSidebar] = useState(false)
  const [tasks, setTasks] = useState<Task[]>([])
  const [employeeStats, setEmployeeStats] = useState<EmployeeTaskInsight[]>([])
  const [activityItems, setActivityItems] = useState<TeamActivity[]>([])
  const [activityLoading, setActivityLoading] = useState(true)
  const [activityError, setActivityError] = useState('')
  const [activityPage, setActivityPage] = useState(1)
  const [projects, setProjects] = useState<import('./types/task.types').Project[]>([])
  const [dashboardLoading, setDashboardLoading] = useState(true)
  const [dashboardError, setDashboardError] = useState('')
  const [createError, setCreateError] = useState('')
  const [creatingTask, setCreatingTask] = useState(false)
  const [assignableUsers, setAssignableUsers] = useState<AssignableUser[]>([])
  const [selectedAssignees, setSelectedAssignees] = useState<string[]>([])
  const [assigneesLoading, setAssigneesLoading] = useState(false)
  const isAdmin = user.role === 'super_admin'
  const [editingTask, setEditingTask] = useState<AdminTaskDetails | null>(null)
  const [editTitle, setEditTitle] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [editProject, setEditProject] = useState('')
  const [editPriority, setEditPriority] = useState<AdminTaskDetails['priority']>('medium')
  const [editStatus, setEditStatus] = useState('todo')
  const [editStartDate, setEditStartDate] = useState('')
  const [editDueDate, setEditDueDate] = useState('')
  const [editAssignees, setEditAssignees] = useState<string[]>([])
  const [editUsers, setEditUsers] = useState<AssignableUser[]>([])
  const [editSaving, setEditSaving] = useState(false)
  const [editError, setEditError] = useState('')
  const [taskNotice, setTaskNotice] = useState('')
  const [selectedTaskId, setSelectedTaskId] = useState('')

  const openTaskFilter = (filter: string) => {
    setSearch('')
    setActiveFilter(filter)
    setActiveNav('Tasks')
  }

  const refreshActivity = useCallback(async () => {
    try {
      setActivityItems(await fetchTeamActivity())
      setActivityError('')
    } catch (error) {
      setActivityError(error instanceof Error ? error.message : 'Could not load team activity.')
    } finally {
      setActivityLoading(false)
    }
  }, [])

  const navGroupsWithCounts = navGroups.map(group => ({
    ...group,
    items: group.items.map(item => {
      if (item.label === 'Tasks') {
        return { ...item, count: dashboardLoading ? '…' : String(tasks.length) }
      }
      if (item.label === 'Approvals') {
        const pending = tasks.filter(task => task.status === 'In review').length
        return { ...item, count: dashboardLoading ? '…' : String(pending) }
      }
      return item
    }),
  }))

  useEffect(() => {
    let cancelled = false
    fetchDashboard().then((data) => {
      if (cancelled) return
      setTasks(data.tasks)
      setEmployeeStats(data.employeeStats)
      setProjects(data.projects)
      setDashboardError('')
    }).catch((error: unknown) => {
      if (!cancelled) setDashboardError(error instanceof Error ? error.message : 'Could not load dashboard data.')
    }).finally(() => { if (!cancelled) setDashboardLoading(false) })
    return () => { cancelled = true }
  }, [])

  useEffect(() => {
    const loadActivity = () => { void refreshActivity() }
    loadActivity()
    const interval = window.setInterval(loadActivity, 30_000)
    window.addEventListener('focus', loadActivity)
    return () => {
      window.clearInterval(interval)
      window.removeEventListener('focus', loadActivity)
    }
  }, [refreshActivity])

  useEffect(() => {
    if (activeNav !== 'Projects' && projectCreateRequest !== 0) setProjectCreateRequest(0)
  }, [activeNav, projectCreateRequest])

  const describeActivity = (item: TeamActivity) => {
    switch (item.action) {
      case 'task_created': return 'created a task'
      case 'task_assigned': return `assigned a task to ${item.targetName || 'a team member'}`
      case 'task_started': return 'started working on'
      case 'task_submitted': return 'submitted for review'
      case 'approved': return 'approved'
      case 'changes_requested': return 'requested changes on'
      case 'task_updated': return 'updated'
      default: return item.action.replaceAll('_', ' ')
    }
  }

  const relativeActivityTime = (value: string) => {
    const timestamp = new Date(value).getTime()
    if (!Number.isFinite(timestamp)) return ''
    const minutes = Math.max(0, Math.floor((Date.now() - timestamp) / 60000))
    if (minutes < 1) return 'Just now'
    if (minutes < 60) return `${minutes}m ago`
    const hours = Math.floor(minutes / 60)
    if (hours < 24) return `${hours}h ago`
    const days = Math.floor(hours / 24)
    return days < 7 ? `${days}d ago` : new Date(value).toLocaleDateString()
  }
  const activityPageCount = Math.max(1, Math.ceil(activityItems.length / 10))
  const currentActivityPage = Math.min(activityPage, activityPageCount)
  const visibleActivityItems = activityItems.slice((currentActivityPage - 1) * 10, currentActivityPage * 10)
  const projectProgressPageCount = Math.max(1, Math.ceil(projects.length / 10))
  const currentProjectProgressPage = Math.min(projectProgressPage, projectProgressPageCount)
  const visibleProjectProgress = projects.slice((currentProjectProgressPage - 1) * 10, currentProjectProgressPage * 10)
  const employeeProgressPageCount = Math.max(1, Math.ceil(employeeStats.length / 10))
  const visibleEmployeeStats = employeeStats.slice((employeeProgressPage - 1) * 10, employeeProgressPage * 10)
  const [newTaskTitle, setNewTaskTitle] = useState('')
  const [newTaskDescription, setNewTaskDescription] = useState('')
  const [newTaskProject, setNewTaskProject] = useState('')
  const [newTaskPriority, setNewTaskPriority] = useState<Priority>('Medium')
  const [newTaskDueDate, setNewTaskDueDate] = useState('')

  useEffect(() => {
    if (!newTaskProject && projects.length) setNewTaskProject(projects[0].name)
  }, [projects, newTaskProject])

  useEffect(() => {
    if (!newTaskProject || !showCreate) { setAssignableUsers([]); return }
    let cancelled = false
    setAssigneesLoading(true)
    fetchAssignableUsers(newTaskProject).then((users) => {
      if (!cancelled) {
        setAssignableUsers(users)
        setSelectedAssignees((current) => current.filter((id) => users.some((user) => user.id === id)))
      }
    }).catch((error: unknown) => {
      if (!cancelled) {
        setAssignableUsers([])
        setCreateError(error instanceof Error ? error.message : 'Could not load project employees.')
      }
    }).finally(() => { if (!cancelled) setAssigneesLoading(false) })
    return () => { cancelled = true }
  }, [newTaskProject, showCreate])

  const filteredTasks = useMemo(() => {
    return tasks.filter((task) => {
      const matchesSearch = `${task.title} ${task.project}`
        .toLowerCase()
        .includes(search.toLowerCase())

      const matchesFilter =
        activeFilter === 'All tasks' ||
        (activeFilter === 'To do' && task.status === 'To do') ||
        (activeFilter === 'In progress' && task.status === 'In progress') ||
        (activeFilter === 'In review' && task.status === 'In review') ||
        (activeFilter === 'Completed' && task.status === 'Completed') ||
        (activeFilter === 'Changes requested' && task.status === 'Changes requested')
        || (activeFilter === 'Blocked' && task.status === 'Blocked')
        || (activeFilter === 'Overdue' && task.overdue === true)

      const matchesPage =
        activeNav === 'Tasks'
          ? (isAdmin ? true : task.assignees.length > 0)
          : activeNav === 'Approvals'
            ? task.status === 'In review'
            : activeNav === 'Projects'
              ? true
              : activeNav === 'Overview'
                ? true
                : true

      return matchesSearch && matchesFilter && matchesPage
    })
  }, [tasks, search, activeFilter, activeNav, isAdmin])

  useEffect(() => { setTaskPage(1) }, [activeNav, activeFilter, search])
  useEffect(() => { setActivityPage(1) }, [activeNav])
  useEffect(() => { setProjectProgressPage(1) }, [activeNav])
  const taskPageCount = Math.max(1, Math.ceil(filteredTasks.length / 10))
  const currentTaskPage = Math.min(taskPage, taskPageCount)
  const displayedTasks = activeNav === 'Overview'
    ? filteredTasks.slice(0, 8)
    : activeNav === 'Tasks'
      ? filteredTasks.slice((currentTaskPage - 1) * 10, currentTaskPage * 10)
      : filteredTasks

  const createTask = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!newTaskTitle.trim() || !newTaskProject || creatingTask) return
    setCreatingTask(true)
    setCreateError('')
    try {
      await createTaskRequest({
        title: newTaskTitle.trim(),
        description: newTaskDescription.trim(),
        project: newTaskProject,
        priority: newTaskPriority,
        dueDate: newTaskDueDate,
        assignedUserIds: selectedAssignees,
      })
      const data = await fetchDashboard()
      setTasks(data.tasks)
      setEmployeeStats(data.employeeStats)
      setProjects(data.projects)
      setDashboardError('')
      void refreshActivity()
      setNewTaskTitle('')
      setNewTaskDescription('')
      setNewTaskPriority('Medium')
      setNewTaskDueDate('')
      setSelectedAssignees([])
      setShowCreate(false)
      setActiveNav('Overview')
      setActiveFilter('All tasks')
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : 'Could not save task.')
    } finally {
      setCreatingTask(false)
    }
  }

  const openEditTask = async (taskId: string) => {
    setEditError('')
    setTaskNotice('')
    try {
      const details = await fetchAdminTaskDetails(taskId)
      setEditingTask(details)
      setEditTitle(details.title)
      setEditDescription(details.description || '')
      setEditProject(details.project)
      setEditPriority(details.priority)
      setEditStatus(details.status)
      setEditStartDate(details.startDate || '')
      setEditDueDate(details.dueDate || '')
      setEditAssignees(details.assignees)
      setEditUsers([])
      try {
        setEditUsers(await fetchAssignableUsers(details.project))
      } catch (error) {
        setEditError(error instanceof Error ? error.message : 'Could not load project members. Existing assignees will be preserved.')
      }
    } catch (error) {
      setEditError(error instanceof Error ? error.message : 'Could not load task details.')
    }
  }

  const saveTaskEdits = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!editingTask || editSaving) return
    setEditSaving(true)
    setEditError('')
    try {
      await updateAdminTask(editingTask.id, { title: editTitle.trim(), description: editDescription, project: editProject, priority: editPriority, status: editStatus, startDate: editStartDate, dueDate: editDueDate, assignedUserIds: editAssignees })
      const data = await fetchDashboard()
      setTasks(data.tasks)
      setEmployeeStats(data.employeeStats)
      setProjects(data.projects)
      setEditingTask(null)
      setTaskNotice('Task updated successfully.')
      void refreshActivity()
    } catch (error) {
      setEditError(error instanceof Error ? error.message : 'Could not update task.')
    } finally {
      setEditSaving(false)
    }
  }

  const removeTask = async (task: Task) => {
    if (!isAdmin) return
    if (!window.confirm(`Remove “${task.title}” from active task lists? Its comments, submissions, and audit history will be retained. Tasks with subtasks cannot be deleted.`)) return
    setTaskNotice('')
    setDashboardError('')
    try {
      await deleteAdminTask(task.id)
      const data = await fetchDashboard()
      setTasks(data.tasks)
      setEmployeeStats(data.employeeStats)
      setProjects(data.projects)
      setTaskNotice(`“${task.title}” was deleted.`)
    } catch (error) {
      setDashboardError(error instanceof Error ? error.message : 'Could not delete task.')
    }
  }

  const pageTitle =
    activeNav === 'Overview'
      ? 'Workspace overview'
      : activeNav === 'Tasks'
        ? (isAdmin ? 'Task management' : 'My tasks')
        : activeNav === 'Approvals'
          ? 'Pending approvals'
          : activeNav

  const pageDescription =
    activeNav === 'Overview'
      ? "Here's what's happening across your workspace today."
      : activeNav === 'Tasks'
        ? (isAdmin ? 'Edit task details, manage assignments, and remove tasks when needed.' : 'Keep track of your assignments and upcoming deadlines.')
        : activeNav === 'Approvals'
          ? 'Review submitted work and keep projects moving.'
          : `Manage your workspace ${activeNav.toLowerCase()} in one place.`

  return (
    <div className="app-shell">
      
<Sidebar
          navGroups={navGroupsWithCounts}
          activeNav={activeNav}
          setActiveNav={label => { setSelectedTaskId(''); setActiveNav(label) }}
          setActiveFilter={setActiveFilter}
          mobileSidebar={mobileSidebar}
          setMobileSidebar={setMobileSidebar}
          AvatarComponent={Avatar}
          onSignOut={onSignOut}
          userName={user.full_name}
          userEmail={user.email}
          userRole="Administrator"
        />

      <main className="main-area">
        <header className="topbar">
          <div className="topbar-left">
            <button className="icon-button mobile-menu" onClick={() => setMobileSidebar(true)} aria-label="Open navigation">
              <Menu size={20} />
            </button>
            <div className="breadcrumbs"><span>Workspace</span><span className="breadcrumb-slash">/</span><strong>{activeNav}</strong></div>
          </div>
          <div className="topbar-actions">
            <span className="today-label"><CalendarDays size={15} /> {new Date().toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })}</span>
            <div className="topbar-divider" />
            <div className="notification-wrap">
              <button className={`icon-button notification-button ${showNotifications ? 'icon-button-active' : ''}`} onClick={() => setShowNotifications((value) => !value)} aria-label="Notifications">
                <Bell size={18} />
                <span className="notification-dot" />
              </button>
              {showNotifications && (
                <div className="notification-popover">
                  <div className="popover-heading"><strong>Notifications</strong><span>2 new</span></div>
                  <div className="notification-item"><span className="notification-icon"><ShieldCheck size={16} /></span><div><strong>Review needed</strong><p>One task is waiting for approval.</p><small>12 minutes ago</small></div></div>
                  <div className="notification-item"><span className="notification-icon notification-icon-blue"><MessageSquare size={16} /></span><div><strong>New comment</strong><p>Alex mentioned you on a task.</p><small>1 hour ago</small></div></div>
                </div>
              )}
            </div>
            <Avatar initials="AK" index={0} />
          </div>
        </header>

        <div className="page-content">
          {dashboardError && <div role="status" style={{ padding: '10px 14px', marginBottom: 12, borderRadius: 8, background: '#fff7ed', color: '#9a3412', fontSize: 13 }}>{dashboardError}</div>}
          <section className="welcome-row">
            <div>
              <div className="eyebrow"><span className="live-dot" /> YOUR WORKSPACE AT A GLANCE</div>
              <h1>{pageTitle}{activeNav === 'Overview' && <span className="wave">✳</span>}</h1>
              <p className="page-subtitle">{pageDescription}</p>
            </div>
            <div className="welcome-actions">
              {activeNav === 'Overview' && !selectedTaskId && <>
                {isAdmin && <button className="button button-primary" onClick={() => { setActiveNav('Projects'); setProjectCreateRequest((request) => request + 1) }}><FolderPlus size={16} /> Add project</button>}
                <button className="button button-primary" onClick={() => setShowCreate(true)}><Plus size={17} /> Create task</button>
              </>}
            </div>
          </section>

          {selectedTaskId ? <EmployeeTaskDetails taskId={selectedTaskId} onBack={() => setSelectedTaskId('')} adminMode onOpenTask={setSelectedTaskId} onEdit={() => void openEditTask(selectedTaskId)} onDelete={async () => {
            const task = tasks.find(item => item.id === selectedTaskId)
            if (!task || !window.confirm(`Remove “${task.title}” from active task lists? Its comments, submissions, and audit history will be retained. Tasks with subtasks cannot be deleted.`)) return
            try { await deleteAdminTask(task.id); setSelectedTaskId(''); const data = await fetchDashboard(); setTasks(data.tasks); setEmployeeStats(data.employeeStats); setProjects(data.projects); setTaskNotice(`“${task.title}” was deleted.`) }
            catch (error) { setDashboardError(error instanceof Error ? error.message : 'Could not delete task.') }
          }} /> : activeNav === 'Team' ? <EmployeeManagement /> : activeNav === 'Approvals' ? <AdminApprovals /> : activeNav === 'Projects' ? (
            <ProjectManagement createRequest={projectCreateRequest} onChanged={() => {
              fetchDashboard().then(data => { setTasks(data.tasks); setEmployeeStats(data.employeeStats); setProjects(data.projects); setDashboardError('') })
                .catch(error => setDashboardError(error instanceof Error ? error.message : 'Could not refresh workspace data.'))
            }} />
          ) : activeNav === 'Activity' ? (
            <section className="panel tasks-panel" style={{ marginTop: 20 }}><div className="panel-heading"><div><h2>Recent team activity</h2><p>Recent task updates from your workspace.</p></div><button className="button button-secondary button-small" onClick={() => void refreshActivity()}>Refresh</button></div>
              {activityError && <p role="status" className="activity-empty">{activityError}</p>}
              <div className="activity-list activity-page-list">{activityLoading ? <p className="activity-empty">Loading team activity…</p> : visibleActivityItems.map((item, index) => <div key={item.id} className="activity-item"><Avatar initials={item.actorName.split(/\s+/).map(part => part[0]).join('').slice(0, 2).toUpperCase()} index={index} /><div className="activity-copy"><p><strong>{item.actorName}</strong> {describeActivity(item)}</p><span>{item.taskTitle}</span><small>{item.projectName} · {relativeActivityTime(item.createdAt)}</small></div><span className={`activity-check ${item.action === 'approved' ? '' : 'activity-check-blue'}`}><Activity size={14} /></span></div>)}{!activityLoading && activityItems.length === 0 && !activityError && <p className="activity-empty">No team activity yet. Activity will appear here as tasks are created and updated.</p>}</div><PaginationControls page={currentActivityPage} pageCount={activityPageCount} onPageChange={setActivityPage}/></section>
          ) : activeNav === 'Reports' ? (
            <section className="stats-grid" style={{ marginTop: 20 }}><article className="stat-card"><div className="stat-top"><span>Total tasks</span><ListTodo size={18}/></div><div className="stat-value">{tasks.length}</div><p className="stat-foot">Current database tasks</p></article><article className="stat-card"><div className="stat-top"><span>To do</span><Clock3 size={18}/></div><div className="stat-value">{tasks.filter(t=>t.status==='To do').length}</div><p className="stat-foot">Not started</p></article><article className="stat-card"><div className="stat-top"><span>In progress</span><Activity size={18}/></div><div className="stat-value">{tasks.filter(t=>t.status==='In progress').length}</div><p className="stat-foot">Currently being worked on</p></article><article className="stat-card"><div className="stat-top"><span>Completed</span><CheckCheck size={18}/></div><div className="stat-value">{tasks.filter(t=>t.status==='Completed').length}</div><p className="stat-foot">Approved/completed tasks</p></article><article className="panel" style={{gridColumn:'1 / -1'}}><h2>Project progress</h2>{visibleProjectProgress.map(p=><div key={p.name} style={{margin:'14px 0'}}><div className="project-title-row"><strong>{p.name}</strong><span>{p.progress}%</span></div><div className="progress-track"><div className="progress-fill" style={{width:`${p.progress}%`,backgroundColor:p.color}}/></div></div>)}<PaginationControls page={currentProjectProgressPage} pageCount={projectProgressPageCount} onPageChange={setProjectProgressPage}/></article></section>
          ) : activeNav === 'Settings' || activeNav === 'Help & support' ? (
            <section className="panel" style={{ marginTop: 20 }}><h2>{activeNav}</h2><p>{activeNav === 'Settings' ? 'Workspace settings are limited to the current account and session.' : 'For help, check the task status, refresh your workspace, or contact the workspace administrator.'}</p><button className="button button-secondary" onClick={() => { setDashboardLoading(true); fetchDashboard().then(data => { setTasks(data.tasks); setEmployeeStats(data.employeeStats); setProjects(data.projects); setDashboardError(''); }).catch(error => setDashboardError(error instanceof Error ? error.message : 'Refresh failed')).finally(() => setDashboardLoading(false)) }}>Refresh workspace data</button></section>
          ) : <>
          {activeNav === 'Overview' && <>
          <section className="stats-grid" aria-label="Workspace statistics">
            <article className="stat-card stat-card-clickable" role="button" tabIndex={0} aria-label="Show all tasks" onClick={() => openTaskFilter('All tasks')} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openTaskFilter('All tasks') } }}>
              <div className="stat-top"><span>Total tasks</span><span className="stat-icon stat-icon-purple"><ListTodo size={18} /></span></div>
              <div className="stat-value">{tasks.length}</div>
              <p className="stat-foot">Current tasks in the database</p>
              <div className="mini-bars purple-bars"><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /></div>
            </article>
            <article className="stat-card stat-card-clickable" role="button" tabIndex={0} aria-label="Show tasks in progress" onClick={() => openTaskFilter('In progress')} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openTaskFilter('In progress') } }}>
              <div className="stat-top"><span>In progress</span><span className="stat-icon stat-icon-blue"><Activity size={18} /></span></div>
              <div className="stat-value">{tasks.filter((task) => task.status === 'In progress').length}</div>
              <p className="stat-foot">Tasks being worked on</p>
              <div className="mini-bars blue-bars"><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /></div>
            </article>
            <article className="stat-card stat-card-clickable" role="button" tabIndex={0} aria-label="Show tasks pending review" onClick={() => openTaskFilter('In review')} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openTaskFilter('In review') } }}>
              <div className="stat-top"><span>Pending reviews</span><span className="stat-icon stat-icon-orange"><ShieldCheck size={18} /></span></div>
              <div className="stat-value">{tasks.filter((task) => task.status === 'In review').length}<span className="stat-change stat-change-neutral"><span className="neutral-dot" /> Needs attention</span></div>
              <p className="stat-foot">Awaiting your approval</p>
              <div className="mini-bars orange-bars"><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /></div>
            </article>
            <article className="stat-card stat-card-clickable" role="button" tabIndex={0} aria-label="Show completed tasks" onClick={() => openTaskFilter('Completed')} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openTaskFilter('Completed') } }}>
              <div className="stat-top"><span>Completed</span><span className="stat-icon stat-icon-green"><CheckCheck size={18} /></span></div>
              <div className="stat-value">{tasks.filter((task) => task.status === 'Completed').length}</div>
              <p className="stat-foot">Tasks completed this month</p>
              <div className="mini-bars green-bars"><i /><i /><i /><i /><i /><i /><i /><i /><i /><i/><i /><i /></div>
            </article>
            <article className="stat-card stat-card-clickable" role="button" tabIndex={0} aria-label="Show to do tasks" onClick={() => openTaskFilter('To do')} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openTaskFilter('To do') } }}>
              <div className="stat-top"><span>To do</span><span className="stat-icon stat-icon-purple"><Clock3 size={18} /></span></div>
              <div className="stat-value">{tasks.filter((task) => task.status === 'To do').length}</div>
              <p className="stat-foot">Tasks ready to be started</p>
            </article>
            {isAdmin && <>
              <article className="stat-card stat-card-clickable" role="button" tabIndex={0} aria-label="Show overdue tasks" onClick={() => openTaskFilter('Overdue')} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openTaskFilter('Overdue') } }}><div className="stat-top"><span>Overdue</span><span className="stat-icon stat-icon-orange"><CalendarDays size={18}/></span></div><div className="stat-value">{tasks.filter(task => task.overdue).length}</div><p className="stat-foot">Past due and still active</p></article>
              <article className="stat-card stat-card-clickable" role="button" tabIndex={0} aria-label="Show blocked tasks" onClick={() => openTaskFilter('Blocked')} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); openTaskFilter('Blocked') } }}><div className="stat-top"><span>Blocked</span><span className="stat-icon stat-icon-orange"><CircleAlert size={18}/></span></div><div className="stat-value">{tasks.filter(task => task.status === 'Blocked').length}</div><p className="stat-foot">Tasks needing attention</p></article>
            </>}
          </section>

          <section className="middle-grid">
            <article className="panel progress-panel">
              <div className="panel-heading">
                <div><h2>Project overview</h2><p>Track progress across your active projects.</p></div>
                <button className="text-button" onClick={() => setActiveNav('Projects')}>All projects <ArrowRight size={15} /></button>
              </div>
              <div className="project-list">
                {visibleProjectProgress.map((project) => (
                  <div className="project-row" key={project.name}>
                    <div className={`project-symbol project-${project.color}`}>{project.code}</div>
                    <div className="project-main">
                      <div className="project-title-row"><strong>{project.name}</strong><span>{project.progress}%</span></div>
                      <div className="progress-track"><div className="progress-fill" style={{ width: `${project.progress}%`, backgroundColor: project.color }} /></div>
                      <div className="project-meta"><span>{project.completed} of {project.total} tasks completed</span><span>Due {project.due}</span></div>
                    </div>
                    <div className="avatar-stack">
                      {project.members.slice(0, 3).map((member, index) => <Avatar key={member} initials={member} index={index} />)}
                      {project.members.length > 3 && <span className="avatar avatar-more">+{project.members.length - 3}</span>}
                    </div>
                  </div>
                ))}
              </div>
              <PaginationControls page={currentProjectProgressPage} pageCount={projectProgressPageCount} onPageChange={setProjectProgressPage}/>
            </article>

            <article className="panel activity-panel">
              <div className="panel-heading">
                <div><h2>Team activity</h2><p>What's happening around you.</p></div>
                <button className="icon-button" aria-label="Refresh team activity" onClick={() => void refreshActivity()}><RefreshCw size={16} /></button>
              </div>
              <div className="activity-list">
                {activityLoading ? <p className="activity-empty">Loading team activity…</p> : activityItems.slice(0, 3).map((item, index) => <div key={item.id} className="activity-item"><Avatar initials={item.actorName.split(/\s+/).map(part => part[0]).join('').slice(0, 2).toUpperCase()} index={index} /><div className="activity-copy"><p><strong>{item.actorName}</strong> {describeActivity(item)}</p><span>{item.taskTitle}</span><small>{item.projectName} · {relativeActivityTime(item.createdAt)}</small></div><span className={`activity-check ${item.action === 'approved' ? '' : 'activity-check-blue'}`}><Activity size={14} /></span></div>)}
                {!activityLoading && activityItems.length === 0 && <p className="activity-empty">{activityError || 'No team activity yet.'}</p>}
                <button className="activity-footer" onClick={() => setActiveNav('Activity')}>View all activity <ArrowRight size={15} /></button>
              </div>
            </article>
          </section>
          {isAdmin && <section className="panel employee-insights-panel"><div className="panel-heading"><div><h2>Employee task progress</h2><p>Assignment and delivery indicators from active tasks.</p></div><span>{employeeStats.length} active employees</span></div>{employeeStats.length ? <><div className="employee-insights-list">{visibleEmployeeStats.map(person=><article className="employee-insight-row" key={person.id}><Avatar initials={person.name.split(/\s+/).map(part=>part[0]).slice(0,2).join('').toUpperCase()}/><div className="employee-insight-main"><div className="employee-insight-name"><strong>{person.name}</strong><span>{person.completed} of {person.total} completed</span></div><div className="progress-track"><div className="progress-fill" style={{width:`${person.total ? person.completed / person.total * 100 : 0}%`}}/></div><small>{person.inProgress} in progress · {person.inReview} awaiting review · {person.overdue} overdue</small></div></article>)}</div><PaginationControls page={employeeProgressPage} pageCount={employeeProgressPageCount} onPageChange={setEmployeeProgressPage}/></> : <div className="employee-empty">No active employee assignments yet.</div>}</section>}
          </>}

          {(activeNav === 'Overview' || activeNav === 'Tasks') && <section className={`panel tasks-panel ${activeNav === 'Tasks' ? 'my-tasks-panel' : ''}`}>
            <div className="panel-heading tasks-heading">
              <div><div className="task-heading-title"><h2>{activeNav === 'Tasks' ? (isAdmin ? 'All workspace tasks' : 'Your assigned tasks') : 'Recent tasks'}</h2>{activeNav === 'Tasks' && <span className="task-total-badge">{filteredTasks.length} {filteredTasks.length === 1 ? 'task' : 'tasks'}</span>}</div><p>{activeNav === 'Tasks' && isAdmin ? 'Edit task details, update assignees, or delete tasks.' : 'Stay on top of the work that matters most.'}</p></div>
              <div className="welcome-actions">
                {activeNav === 'Overview' && <button className="button button-secondary button-small" onClick={() => { setSearch(''); setActiveFilter('All tasks'); setActiveNav('Tasks') }}>View all tasks <ArrowRight size={14} /></button>}
                <button className="button button-primary tasks-add-button" onClick={() => { setCreateError(''); setShowCreate(true) }}><Plus size={17} /> Add task</button>
              </div>
            </div>
            <div className="task-toolbar">
              <div className="filter-tabs">
                {['All tasks', 'To do', 'In progress', 'In review', 'Changes requested', 'Completed', ...(isAdmin ? ['Overdue', 'Blocked'] : [])].map((filter) => (
                  <button key={filter} className={`filter-tab ${activeFilter === filter ? 'filter-tab-active' : ''}`} onClick={() => setActiveFilter(filter)}>
                    {filter}
                    {filter === 'In review' && <span className="filter-count">{tasks.filter(task => task.status === 'In review').length}</span>}
                  </button>
                ))}
              </div>
              <div className="task-tools">
                <label className="task-search"><Search size={15} /><input id="task-search" placeholder="Search tasks..." value={search} onChange={(event) => setSearch(event.target.value)} /><kbd>⌘ F</kbd></label>
                <button className="icon-button filter-button" aria-label="Filter tasks" onClick={()=> setActiveFilter(activeFilter === 'All tasks' ? 'In progress' : 'All tasks')}><SlidersHorizontal size={17} /></button>
              </div>
            </div>

            <div className="table-scroll">
              <table className="tasks-table">
                <thead>
                  <tr><th>Task name</th><th>Assignees</th><th>Due date</th><th>Priority</th><th>Status</th><th /></tr>
                </thead>
                <tbody>
                  {displayedTasks.map((task) => (
                    <tr key={task.id}>
                      <td><button type="button" className="task-name-cell task-name-open" onClick={() => isAdmin && setSelectedTaskId(task.id)} aria-label={`Open details for ${task.title}`}><span className={`task-checkbox-marker marker-${task.status.toLowerCase().replaceAll(' ', '-')}`}>{task.status === 'Completed' ? <Check size={12} /> : null}</span><span className="task-name-copy"><strong>{task.title}</strong><span><span className="project-mini-dot" style={{ backgroundColor: task.projectColor }} />{task.project}</span></span></button></td>
                      <td><div className="avatar-stack table-avatars">{task.assignees.map((member, index) => <Avatar key={member} initials={member} index={index} />)}</div></td>
                      <td><span className={`due-date ${task.due === 'Today' ? 'due-today' : ''}`}><CalendarDays size={14} />{task.due}</span></td>
                      <td><PriorityLabel priority={task.priority} /></td>
                      <td><StatusBadge status={task.status} /></td>
                      <td>{isAdmin ? <div style={{display:'flex',gap:6,justifyContent:'flex-end'}}><button className="icon-button row-more" aria-label={`Edit ${task.title}`} title="Edit task" onClick={() => void openEditTask(task.id)}><Pencil size={16} /></button><button className="icon-button row-more" aria-label={`Delete ${task.title}`} title="Delete task" onClick={() => void removeTask(task)}><Trash2 size={16} /></button></div> : <button className="icon-button row-more" aria-label={`Find ${task.title}`} title="Find this task" onClick={() => { setSearch(task.title); setActiveFilter('All tasks') }}><Search size={18} /></button>}</td>
                    </tr>
                  ))}
                  {displayedTasks.length === 0 && <tr><td colSpan={6} className="empty-state"><Search size={20} /><strong>No tasks found</strong><span>Try a different search or filter.</span></td></tr>}
                </tbody>
              </table>
            </div>
            {taskNotice && <div role="status" style={{marginTop:12,padding:10,borderRadius:8,background:'#ecfdf3',color:'#067647',fontSize:13}}>{taskNotice}</div>}
            <div className="table-footer"><span>{dashboardLoading ? 'Loading dashboard data…' : dashboardError ? dashboardError : activeNav === 'Tasks' ? <>Showing <strong>{filteredTasks.length === 0 ? 0 : (currentTaskPage - 1) * 10 + 1}–{Math.min(currentTaskPage * 10, filteredTasks.length)}</strong> of <strong>{filteredTasks.length}</strong> tasks</> : <>Showing <strong>{displayedTasks.length}</strong> of <strong>{tasks.length}</strong> database tasks</>}</span><div className="table-footer-actions">{activeNav === 'Tasks' && <div className="task-pagination" aria-label="Task pages"><button className="button button-secondary button-small" disabled={currentTaskPage <= 1} onClick={() => setTaskPage(page => Math.max(1, page - 1))}>Previous</button><span>Page {currentTaskPage} of {taskPageCount}</span><button className="button button-secondary button-small" disabled={currentTaskPage >= taskPageCount} onClick={() => setTaskPage(page => Math.min(taskPageCount, page + 1))}>Next page <ArrowRight size={14} /></button></div>}<button className="text-button" onClick={() => { setActiveFilter('All tasks'); setSearch(''); setActiveNav('Overview') }}>Reset filters <ArrowRight size={14} /></button></div></div>
          </section>}

          </>}

          <footer className="page-footer"><span>© 2026 Office Task Manager. Built for teams that get things done.</span><span><span className="footer-status-dot" /> All systems operational</span></footer>
        </div>
      </main>

      {editingTask && (
        <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) setEditingTask(null) }}>
          <form className="create-modal task-form-modal" onSubmit={saveTaskEdits}>
            <div className="modal-heading"><div><span className="modal-icon"><Pencil size={18} /></span><div><h2>Edit task</h2><p>{editingTask.project} · Status: {editingTask.status.replaceAll('_', ' ')}</p></div></div><button type="button" className="icon-button" onClick={() => setEditingTask(null)} aria-label="Close dialog"><X size={19} /></button></div>
            <label className="form-label">Task name<input required maxLength={250} value={editTitle} onChange={(event) => setEditTitle(event.target.value)} /></label>
            <label className="form-label">Description<textarea rows={4} maxLength={10000} value={editDescription} onChange={(event) => setEditDescription(event.target.value)} placeholder="Add task instructions, acceptance criteria, or context…" /></label>
            <label className="form-label">Project<select required value={editProject} onChange={async (event) => { const project = event.target.value; setEditProject(project); setEditAssignees([]); try { setEditUsers(await fetchAssignableUsers(project)) } catch (error) { setEditError(error instanceof Error ? error.message : 'Could not load project members.') } }}>{projects.map((project) => <option key={project.name} value={project.name}>{project.name}</option>)}</select></label>
            <label className="form-label">Priority<select value={editPriority} onChange={(event) => setEditPriority(event.target.value as AdminTaskDetails['priority'])}><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option><option value="urgent">Urgent</option></select></label>
            <label className="form-label">Status<select value={editStatus} onChange={(event) => setEditStatus(event.target.value)}><option value="todo">To do</option><option value="in_progress">In progress</option><option value="in_review">In review</option><option value="changes_requested">Changes requested</option><option value="approved">Completed</option><option value="blocked">Blocked</option><option value="cancelled">Cancelled</option></select></label>
            <label className="form-label">Start date<input type="date" value={editStartDate} onChange={(event) => setEditStartDate(event.target.value)} /></label>
            <label className="form-label">Due date<input type="date" value={editDueDate} onChange={(event) => setEditDueDate(event.target.value)} /></label>
            <label className="form-label">Assignees <span className="optional-label">(active members of this project)</span></label>
            <div className="assignee-picker" aria-label="Edit task assignees">
              {editUsers.length === 0 ? <p className="assignee-help">No active project members available. Add members in Project Management first.</p> : editUsers.map((user) => <label className="assignee-option" key={user.id}><input type="checkbox" checked={editAssignees.includes(user.id)} onChange={(event) => setEditAssignees((current) => event.target.checked ? [...current, user.id] : current.filter((id) => id !== user.id))} /><span className="assignee-avatar">{user.fullName.split(/\s+/).map((part) => part[0]).slice(0,2).join('').toUpperCase()}</span><span className="assignee-name"><strong>{user.fullName}</strong><small>{user.email}</small></span></label>)}
            </div>
            <p style={{fontSize:12,color:'#667085'}}>Choose Completed to mark this task approved. Use Approvals to record review feedback and decisions.</p>
            {editError && <div role="alert" className="create-error">{editError}</div>}
            <div className="modal-actions"><button type="button" className="button button-secondary" onClick={() => setEditingTask(null)}>Cancel</button><button type="submit" disabled={editSaving} className="button button-primary"><Check size={16} /> {editSaving ? 'Saving…' : 'Save changes'}</button></div>
          </form>
        </div>
      )}

      {showCreate && (
        <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) setShowCreate(false) }}>
          <form className="create-modal task-form-modal" onSubmit={createTask}>
            <div className="modal-heading"><div><span className="modal-icon"><Plus size={19} /></span><div><h2>Create a task</h2><p>Add a new task to your workspace.</p></div></div><button type="button" className="icon-button" onClick={() => setShowCreate(false)} aria-label="Close dialog"><X size={19} /></button></div>
            <label className="form-label">Task name<input autoFocus required maxLength={250} placeholder="e.g. Prepare project documentation" value={newTaskTitle} onChange={(event) => setNewTaskTitle(event.target.value)} /></label>
            <label className="form-label">Description <span className="optional-label">(optional)</span><textarea rows={4} maxLength={10000} value={newTaskDescription} onChange={(event) => setNewTaskDescription(event.target.value)} placeholder="Explain what needs to be done, add instructions, acceptance criteria, or useful context…" /></label>
            <label className="form-label">Project<select required value={newTaskProject} onChange={(event) => setNewTaskProject(event.target.value)}>{projects.map((project) => <option key={project.name} value={project.name}>{project.name}</option>)}</select></label>
            <label className="form-label">Priority<select value={newTaskPriority} onChange={(event)=> setNewTaskPriority(event.target.value as Priority)}><option>Low</option><option>Medium</option><option>High</option><option>Urgent</option></select></label>
            <label className="form-label">Assign to <span className="optional-label">(optional, select multiple)</span></label>
            <div className="assignee-picker" aria-label="Assign task to employees">
              {assigneesLoading ? <p className="assignee-help">Loading project employees…</p> : assignableUsers.length === 0 ? <p className="assignee-help">No active employees are assigned to this project.</p> : assignableUsers.map((user) => (
                <label className="assignee-option" key={user.id}>
                  <input type="checkbox" checked={selectedAssignees.includes(user.id)} onChange={(event) => setSelectedAssignees((current) => event.target.checked ? [...current, user.id] : current.filter((id) => id !== user.id))} />
                  <span className="assignee-avatar">{user.fullName.split(/\s+/).map((part) => part[0]).slice(0, 2).join('').toUpperCase()}</span>
                  <span className="assignee-name"><strong>{user.fullName}</strong><small>{user.email}</small></span>
                </label>
              ))}
            </div>
            <label className="form-label">Due date <span className="optional-label">(optional)</span><input type="date" value={newTaskDueDate} onChange={(event) => setNewTaskDueDate(event.target.value)} /></label>
            {createError && <div role="alert" className="create-error">{createError}</div>}
            <div className="modal-note"><Clock3 size={15} /> Selected employees will be saved to this task in the database.</div>
            <div className="modal-actions"><button type="button" className="button button-secondary" onClick={() => setShowCreate(false)}>Cancel</button><button type="submit" disabled={creatingTask || projects.length === 0} className="button button-primary"><Plus size={16} /> {creatingTask ? 'Saving…' : 'Create task'}</button></div>
          </form>
        </div>
      )}
    </div>
  )
}

export default App
