import Sidebar from './layouts/Sidebar'
import AdminApprovals from './components/AdminApprovals'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import {
  Activity,
  ArrowRight,
  ArrowUpRight,
  Bell,
  CalendarDays,
  Check,
  CheckCheck,
  Clock3,
  FileText,
  FolderKanban,
  LayoutDashboard,
  ListTodo,
  Menu,
  MessageSquare,
  MoreHorizontal,
  Plus,
  Search,
  ShieldCheck,
  SlidersHorizontal,
  Users,
  X,
} from 'lucide-react'
import './App.css'
import type { Task, Priority } from './types/task.types'
import { createTask as createTaskRequest, fetchAssignableUsers, fetchDashboard, type AssignableUser } from './services/dashboard'
import { Avatar, StatusBadge, PriorityLabel } from './components/TaskUI'

const navGroups = [
  {
    label: 'WORKSPACE',
    items: [
      { label: 'Overview', icon: LayoutDashboard },
      { label: 'My tasks', icon: ListTodo, count: '8' },
      { label: 'Projects', icon: FolderKanban },
      { label: 'Team', icon: Users },
    ],
  },
  {
    label: 'MANAGEMENT',
    items: [
      { label: 'Approvals', icon: ShieldCheck, count: '3' },
      { label: 'Activity', icon: Activity },
      { label: 'Reports', icon: FileText },
    ],
  },
]

function App() {
  const [activeNav, setActiveNav] = useState('Overview')
  const [activeFilter, setActiveFilter] = useState('All tasks')
  const [search, setSearch] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [showNotifications, setShowNotifications] = useState(false)
  const [mobileSidebar, setMobileSidebar] = useState(false)
  const [tasks, setTasks] = useState<Task[]>([])
  const [projects, setProjects] = useState<import('./types/task.types').Project[]>([])
  const [dashboardLoading, setDashboardLoading] = useState(true)
  const [dashboardError, setDashboardError] = useState('')
  const [createError, setCreateError] = useState('')
  const [creatingTask, setCreatingTask] = useState(false)
  const [assignableUsers, setAssignableUsers] = useState<AssignableUser[]>([])
  const [selectedAssignees, setSelectedAssignees] = useState<string[]>([])
  const [assigneesLoading, setAssigneesLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    const token = sessionStorage.getItem('office_task_manager_token')
    if (!token) { setDashboardLoading(false); setDashboardError('Please sign in to load dashboard data.'); return }
    fetchDashboard(token).then((data) => {
      if (cancelled) return
      setTasks(data.tasks)
      setProjects(data.projects)
      setDashboardError('')
    }).catch((error: unknown) => {
      if (!cancelled) setDashboardError(error instanceof Error ? error.message : 'Could not load dashboard data.')
    }).finally(() => { if (!cancelled) setDashboardLoading(false) })
    return () => { cancelled = true }
  }, [])
  const [newTaskTitle, setNewTaskTitle] = useState('')
  const [newTaskProject, setNewTaskProject] = useState('')
  const [newTaskPriority, setNewTaskPriority] = useState<Priority>('Medium')
  const [newTaskDueDate, setNewTaskDueDate] = useState('')

  useEffect(() => {
    if (!newTaskProject && projects.length) setNewTaskProject(projects[0].name)
  }, [projects, newTaskProject])

  useEffect(() => {
    const token = sessionStorage.getItem('office_task_manager_token')
    if (!token || !newTaskProject || !showCreate) { setAssignableUsers([]); return }
    let cancelled = false
    setAssigneesLoading(true)
    fetchAssignableUsers(token, newTaskProject).then((users) => {
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
      const matchesSearch = `${task.title} ${task.project} ${task.id}`
        .toLowerCase()
        .includes(search.toLowerCase())

      const matchesFilter =
        activeFilter === 'All tasks' ||
        (activeFilter === 'In progress' && task.status === 'In progress') ||
        (activeFilter === 'In review' && task.status === 'In review') ||
        (activeFilter === 'Completed' && task.status === 'Completed')

      const matchesPage =
        activeNav === 'My tasks'
          ? task.assignees.length > 0
          : activeNav === 'Approvals'
            ? task.status === 'In review'
            : activeNav === 'Projects'
              ? true
              : activeNav === 'Overview'
                ? true
                : true

      return matchesSearch && matchesFilter && matchesPage
    })
  }, [tasks, search, activeFilter, activeNav])

  const createTask = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!newTaskTitle.trim() || !newTaskProject || creatingTask) return
    const token = sessionStorage.getItem('office_task_manager_token')
    if (!token) { setCreateError('Please sign in again before creating a task.'); return }
    setCreatingTask(true)
    setCreateError('')
    try {
      await createTaskRequest(token, {
        title: newTaskTitle.trim(),
        project: newTaskProject,
        priority: newTaskPriority,
        dueDate: newTaskDueDate,
        assignedUserIds: selectedAssignees,
      })
      const data = await fetchDashboard(token)
      setTasks(data.tasks)
      setProjects(data.projects)
      setDashboardError('')
      setNewTaskTitle('')
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

  const pageTitle =
    activeNav === 'Overview'
      ? 'Good morning, Alex'
      : activeNav === 'My tasks'
        ? 'My tasks'
        : activeNav === 'Approvals'
          ? 'Pending approvals'
          : activeNav

  const pageDescription =
    activeNav === 'Overview'
      ? "Here's what's happening across your workspace today."
      : activeNav === 'My tasks'
        ? 'Keep track of your assignments and upcoming deadlines.'
        : activeNav === 'Approvals'
          ? 'Review submitted work and keep projects moving.'
          : `Manage your workspace ${activeNav.toLowerCase()} in one place.`

  return (
    <div className="app-shell">
      
<Sidebar
          navGroups={navGroups}
          activeNav={activeNav}
          setActiveNav={setActiveNav}
          setActiveFilter={setActiveFilter}
          mobileSidebar={mobileSidebar}
          setMobileSidebar={setMobileSidebar}
          AvatarComponent={Avatar}
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
            <span className="today-label"><CalendarDays size={15} /> Wednesday, Oct 21</span>
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
              <button className="button button-secondary" onClick={() => setActiveNav('Projects')}><FolderKanban size={16} /> View projects</button>
              <button className="button button-primary" onClick={() => setShowCreate(true)}><Plus size={17} /> Create task</button>
            </div>
          </section>

          {activeNav === 'Approvals' ? <AdminApprovals /> : <>
          <section className="stats-grid" aria-label="Workspace statistics">
            <article className="stat-card">
              <div className="stat-top"><span>Total tasks</span><span className="stat-icon stat-icon-purple"><ListTodo size={18} /></span></div>
              <div className="stat-value">{tasks.length}<span className="stat-change"><ArrowUpRight size={14} /> 12.8%</span></div>
              <p className="stat-foot">Compared to last month</p>
              <div className="mini-bars purple-bars"><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /></div>
            </article>
            <article className="stat-card">
              <div className="stat-top"><span>In progress</span><span className="stat-icon stat-icon-blue"><Activity size={18} /></span></div>
              <div className="stat-value">{tasks.filter((task) => task.status === 'In progress').length}<span className="stat-change"><ArrowUpRight size={14} /> 8.2%</span></div>
              <p className="stat-foot">Tasks being worked on</p>
              <div className="mini-bars blue-bars"><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /></div>
            </article>
            <article className="stat-card">
              <div className="stat-top"><span>Pending reviews</span><span className="stat-icon stat-icon-orange"><ShieldCheck size={18} /></span></div>
              <div className="stat-value">{tasks.filter((task) => task.status === 'In review').length}<span className="stat-change stat-change-neutral"><span className="neutral-dot" /> Needs attention</span></div>
              <p className="stat-foot">Awaiting your approval</p>
              <div className="mini-bars orange-bars"><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /><i /></div>
            </article>
            <article className="stat-card">
              <div className="stat-top"><span>Completed</span><span className="stat-icon stat-icon-green"><CheckCheck size={18} /></span></div>
              <div className="stat-value">{tasks.filter((task) => task.status === 'Completed').length}<span className="stat-change"><ArrowUpRight size={14} /> 18.4%</span></div>
              <p className="stat-foot">Tasks completed this month</p>
              <div className="mini-bars green-bars"><i /><i /><i /><i /><i /><i /><i /><i /><i /><i/><i /><i /></div>
            </article>
          </section>

          <section className="middle-grid">
            <article className="panel progress-panel">
              <div className="panel-heading">
                <div><h2>Project overview</h2><p>Track progress across your active projects.</p></div>
                <button className="text-button" onClick={() => setActiveNav('Projects')}>All projects <ArrowRight size={15} /></button>
              </div>
              <div className="project-list">
                {projects.map((project) => (
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
            </article>

            <article className="panel activity-panel">
              <div className="panel-heading">
                <div><h2>Team activity</h2><p>What's happening around you.</p></div>
                <button className="icon-button" aria-label="More activity options"><MoreHorizontal size={19} /></button>
              </div>
              <div className="activity-list">
                <div className="activity-item">
                  <Avatar initials="RM" index={1} />
                  <div className="activity-copy"><p><strong>Rahul Mehta</strong> completed a task</p><span>Updated API documentation</span><small>10 minutes ago</small></div>
                  <span className="activity-check"><Check size={14} /></span>
                </div>
                <div className="activity-item">
                  <Avatar initials="SK" index={2} />
                  <div className="activity-copy"><p><strong>Sneha Kapoor</strong> submitted work</p><span>Dashboard UI implementation</span><small>35 minutes ago</small></div>
                  <span className="activity-check activity-check-blue"><ShieldCheck size={14} /></span>
                </div>
                <div className="activity-item">
                  <Avatar initials="NP" index={3} />
                  <div className="activity-copy"><p><strong>Neha Patel</strong> added a comment</p><span>“Ready for the final review.”</span><small>1 hour ago</small></div>
                  <span className="activity-comment"><MessageSquare size={14} /></span>
                </div>
                <button className="activity-footer" onClick={() => setActiveNav('Activity')}>View all activity <ArrowRight size={15} /></button>
              </div>
            </article>
          </section>

          <section className="panel tasks-panel">
            <div className="panel-heading tasks-heading">
              <div><h2>{activeNav === 'Approvals' ? 'Tasks awaiting review' : activeNav === 'My tasks' ? 'Your assigned tasks' : 'Recent tasks'}</h2><p>Stay on top of the work that matters most.</p></div>
              <button className="button button-secondary button-small" onClick={() => setShowCreate(true)}><Plus size={15} /> Add task</button>
            </div>
            <div className="task-toolbar">
              <div className="filter-tabs">
                {['All tasks', 'In progress', 'In review', 'Completed'].map((filter) => (
                  <button key={filter} className={`filter-tab ${activeFilter === filter ? 'filter-tab-active' : ''}`} onClick={() => setActiveFilter(filter)}>
                    {filter}
                    {filter === 'In review' && <span className="filter-count">3</span>}
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
                  <tr><th><input type="checkbox" aria-label="Select all tasks" /></th><th>Task name</th><th>Assignees</th><th>Due date</th><th>Priority</th><th>Status</th><th /></tr>
                </thead>
                <tbody>
                  {filteredTasks.map((task) => (
                    <tr key={task.id}>
                      <td><input type="checkbox" aria-label={`Select ${task.title}`} /></td>
                      <td><div className="task-name-cell"><span className={`task-checkbox-marker marker-${task.status.toLowerCase().replaceAll(' ', '-')}`}>{task.status === 'Completed' ? <Check size={12} /> : null}</span><div className="task-name-copy"><strong>{task.title}</strong><span><span className="project-mini-dot" style={{ backgroundColor: task.projectColor }} />{task.project}<span className="task-id">{task.id}</span></span></div></div></td>
                      <td><div className="avatar-stack table-avatars">{task.assignees.map((member, index) => <Avatar key={member} initials={member} index={index} />)}</div></td>
                      <td><span className={`due-date ${task.due === 'Today' ? 'due-today' : ''}`}><CalendarDays size={14} />{task.due}</span></td>
                      <td><PriorityLabel priority={task.priority} /></td>
                      <td><StatusBadge status={task.status} /></td>
                      <td><button className="icon-button row-more" aria-label={`Options for ${task.title}`}><MoreHorizontal size={18} /></button></td>
                    </tr>
                  ))}
                  {filteredTasks.length === 0 && <tr><td colSpan={7} className="empty-state"><Search size={20} /><strong>No tasks found</strong><span>Try a different search or filter.</span></td></tr>}
                </tbody>
              </table>
            </div>
            <div className="table-footer"><span>{dashboardLoading ? 'Loading dashboard data…' : dashboardError ? dashboardError : <>Showing <strong>{filteredTasks.length}</strong> of <strong>{tasks.length}</strong> database tasks</>}</span><button className="text-button" onClick={() => { setActiveFilter('All tasks'); setSearch(''); setActiveNav('Overview') }}>Reset filters <ArrowRight size={14} /></button></div>
          </section>

          </>}

          <footer className="page-footer"><span>© 2026 Workboard. Built for teams that get things done.</span><span><span className="footer-status-dot" /> All systems operational</span></footer>
        </div>
      </main>

      {showCreate && (
        <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) setShowCreate(false) }}>
          <form className="create-modal" onSubmit={createTask}>
            <div className="modal-heading"><div><span className="modal-icon"><Plus size={19} /></span><div><h2>Create a task</h2><p>Add a new task to your workspace.</p></div></div><button type="button" className="icon-button" onClick={() => setShowCreate(false)} aria-label="Close dialog"><X size={19} /></button></div>
            <label className="form-label">Task name<input autoFocus required placeholder="e.g. Prepare project documentation" value={newTaskTitle} onChange={(event) => setNewTaskTitle(event.target.value)} /></label>
            <label className="form-label">Project<select value={newTaskProject} onChange={(event) => setNewTaskProject(event.target.value)}>{projects.map((project) => <option key={project.name}>{project.name}</option>)}</select></label>
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