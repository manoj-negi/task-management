import { useEffect, useMemo, useState } from "react"
import { Activity, CalendarDays, CheckCheck, CheckCircle2, Clock3, FolderKanban, LayoutDashboard, ListTodo, Menu, RefreshCw, ShieldCheck } from "lucide-react"
import Sidebar from "../layouts/Sidebar"
import PaginationControls from "./PaginationControls"
import EmployeeTaskDetails from "./EmployeeTaskDetails"
import { Avatar, PriorityLabel, StatusBadge } from "./TaskUI"
import { fetchDashboard } from "../services/dashboard"
import { performTaskAction } from "../services/tasks"
import type { Project, Task } from "../types/task.types"
import type { AuthUser } from "../services/auth"

const pageSize = 10
const navGroups = [{ label: "WORKSPACE", items: [
  { label: "Overview", icon: LayoutDashboard },
  { label: "Tasks", icon: ListTodo },
  { label: "Projects", icon: FolderKanban },
] }]

export default function EmployeeDashboard({ user, onSignOut }: { user: AuthUser; onSignOut: () => void }) {
  const [tasks, setTasks] = useState<Task[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [activeNav, setActiveNav] = useState("Overview")
  const [filter, setFilter] = useState("All")
  const [taskPage, setTaskPage] = useState(1)
  const [projectPage, setProjectPage] = useState(1)
  const [mobileSidebar, setMobileSidebar] = useState(false)
  const [busyTask, setBusyTask] = useState<string | null>(null)
  const [actionMessage, setActionMessage] = useState("")
  const [selectedTaskId, setSelectedTaskId] = useState("")

  async function load() {
    setLoading(true)
    setError("")
    try {
      const data = await fetchDashboard()
      setTasks(data.tasks)
      setProjects(data.projects)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load your work.")
    } finally {
      setLoading(false)
    }
  }

  async function handleTaskAction(task: Task, action: "start" | "submit") {
    setBusyTask(task.id)
    setError("")
    setActionMessage("")
    try {
      const result = await performTaskAction(task.id, action)
      setActionMessage(result.message)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not update task.")
    } finally {
      setBusyTask(null)
    }
  }

  useEffect(() => { void load() }, [])

  const filteredTasks = useMemo(() => {
    if (filter === "Due soon") return tasks.filter(task => task.due !== "No due date")
    if (filter === "All") return tasks
    return tasks.filter(task => task.status === filter)
  }, [tasks, filter])
  const inProgress = tasks.filter(task => task.status === "In progress").length
  const inReview = tasks.filter(task => task.status === "In review").length
  const completed = tasks.filter(task => task.status === "Completed").length
  const taskPageCount = Math.max(1, Math.ceil(filteredTasks.length / pageSize))
  const projectPageCount = Math.max(1, Math.ceil(projects.length / pageSize))
  const visibleTasks = filteredTasks.slice((taskPage - 1) * pageSize, taskPage * pageSize)
  const visibleProjects = projects.slice((projectPage - 1) * pageSize, projectPage * pageSize)
  const initials = user.full_name.split(/\s+/).map(part => part[0]).slice(0, 2).join("").toUpperCase()

  function navigate(label: string) {
    setSelectedTaskId("")
    setActiveNav(label)
    setTaskPage(1)
    setProjectPage(1)
    setMobileSidebar(false)
  }

  function openTaskDetails(task: Task) {
    setSelectedTaskId(task.id)
    setActiveNav("Tasks")
    setMobileSidebar(false)
  }

  function renderTask(task: Task) {
    return <article className="employee-task-card" key={task.id}>
      <div className="employee-task-main">
        <div className="employee-task-heading">
          <div className="employee-task-title-wrap"><button className="employee-task-title-button" onClick={() => openTaskDetails(task)}>{task.title}</button><span className="employee-task-project"><FolderKanban size={14}/>{task.project}</span></div>
          <StatusBadge status={task.status}/>
        </div>
        <div className="employee-task-meta"><PriorityLabel priority={task.priority}/><span><CalendarDays size={14}/>{task.due}</span></div>
        {task.reviewDecision === "changes_requested" && task.reviewFeedback && <div className="employee-feedback"><strong><ShieldCheck size={15}/> Admin feedback</strong><p>{task.reviewFeedback}</p></div>}
        {task.status !== "Completed" && <div className="employee-task-actions">
          {(task.myWorkStatus === "assigned" || !task.myWorkStatus) && <button className="button button-primary button-small" disabled={busyTask === task.id} onClick={() => void handleTaskAction(task, "start")}>{busyTask === task.id ? "Updating…" : "Start task"}</button>}
          {task.myWorkStatus === "in_progress" && <button className="button button-secondary button-small" onClick={() => openTaskDetails(task)}>Add summary &amp; submit</button>}
        </div>}
      </div>
    </article>
  }

  return <div className="app-shell employee-app-shell">
    <Sidebar navGroups={navGroups} activeNav={activeNav} setActiveNav={label => { setSelectedTaskId(""); setActiveNav(label) }} setActiveFilter={value => setFilter(current => { const next = typeof value === "function" ? value(current) : value; return next === "All tasks" ? "All" : next })} mobileSidebar={mobileSidebar} setMobileSidebar={setMobileSidebar} AvatarComponent={Avatar} onSignOut={onSignOut} userName={user.full_name} userEmail={user.email} userRole="Employee"/>
    <main className="main-area">
      <header className="topbar">
        <div className="topbar-left"><button className="icon-button mobile-menu" onClick={() => setMobileSidebar(true)} aria-label="Open navigation"><Menu size={20}/></button><div className="breadcrumbs"><span>Workspace</span><span className="breadcrumb-slash">/</span><strong>{activeNav}</strong></div></div>
        <div className="topbar-actions"><span className="today-label"><CalendarDays size={15}/>{new Date().toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" })}</span><div className="topbar-divider"/><Avatar initials={initials} index={1}/></div>
      </header>

      <div className="page-content">
        {error && <div className="employee-alert employee-alert-error" role="alert">{error}<button className="button button-secondary button-small" onClick={() => void load()}>Try again</button></div>}
        {actionMessage && <div className="employee-alert employee-alert-success" role="status"><CheckCircle2 size={17}/>{actionMessage}</div>}
        {selectedTaskId ? <EmployeeTaskDetails taskId={selectedTaskId} onBack={() => setSelectedTaskId("")}/> : <>
        <section className="welcome-row">
          <div><div className="eyebrow"><span className="live-dot"/>{activeNav === "Overview" ? "YOUR WORKSPACE AT A GLANCE" : `YOUR ${activeNav.toUpperCase()}`}</div><h1>{activeNav === "Overview" ? `Welcome, ${user.full_name.split(" ")[0]}` : activeNav === "Tasks" ? "My tasks" : activeNav === "Projects" ? "My projects" : activeNav}</h1><p className="page-subtitle">{activeNav === "Overview" ? "A clear view of your assignments, progress, and project work." : activeNav === "Tasks" ? "Keep track of assigned work, status, and deadlines." : activeNav === "Projects" ? "Projects you’re contributing to and their current progress." : activeNav === "Settings" ? "Your account details and secure workspace session." : "Find guidance for using your task workspace."}</p></div>
          <div className="welcome-actions"><button className="button button-secondary" onClick={() => void load()} disabled={loading}><RefreshCw size={16}/>{loading ? "Refreshing…" : "Refresh"}</button></div>
        </section>

        {activeNav === "Overview" && <>
          <section className="stats-grid employee-stats-grid" aria-label="My task summary">
            {[
              { label: "My tasks", value: tasks.length, icon: ListTodo, iconClass: "stat-icon-purple", filter: "All", note: "Assigned to your account" },
              { label: "In progress", value: inProgress, icon: Activity, iconClass: "stat-icon-blue", filter: "In progress", note: "Work underway" },
              { label: "In review", value: inReview, icon: Clock3, iconClass: "stat-icon-orange", filter: "In review", note: "Submitted for approval" },
              { label: "Completed", value: completed, icon: CheckCheck, iconClass: "stat-icon-green", filter: "Completed", note: "Approved tasks" },
            ].map(item => { const Icon = item.icon; return <button className="stat-card stat-card-clickable" key={item.label} onClick={() => { setFilter(item.filter); navigate("Tasks") }}><div className="stat-top"><span>{item.label}</span><span className={`stat-icon ${item.iconClass}`}><Icon size={18}/></span></div><div className="stat-value">{loading ? "—" : item.value}</div><p className="stat-foot">{item.note}</p></button> })}
          </section>
          <section className="employee-overview-grid">
            <div className="panel employee-panel"><div className="panel-heading"><div><h2>Recent tasks</h2><p>Your latest assignments and their current status.</p></div><button className="text-button" onClick={() => navigate("Tasks")}>View all tasks <span aria-hidden="true">→</span></button></div>
              {loading ? <div className="employee-empty">Loading your tasks…</div> : tasks.length ? <div className="employee-task-list">{tasks.slice(0, 5).map(renderTask)}</div> : <div className="employee-empty"><ListTodo size={28}/><strong>No tasks assigned yet</strong><span>New assignments will appear here.</span></div>}
            </div>
            <aside className="panel employee-panel"><div className="panel-heading"><div><h2>My projects</h2><p>Progress across your active work.</p></div><button className="icon-button" aria-label="View all projects" onClick={() => navigate("Projects")}><FolderKanban size={17}/></button></div>
              {loading ? <div className="employee-empty">Loading projects…</div> : projects.length ? <div className="employee-project-list">{projects.slice(0, 5).map(project => <article className="employee-project-card" key={project.name}><div className="employee-project-heading"><span className="employee-project-icon"><FolderKanban size={16}/></span><div><strong>{project.name}</strong><span>{project.completed} of {project.total} tasks approved</span></div></div><div className="progress-track"><div className="progress-fill" style={{ width: `${Math.max(0, Math.min(100, project.progress))}%` }}/></div><div className="employee-project-foot"><span>{project.progress}% complete</span><span>Due {project.due}</span></div></article>)}</div> : <div className="employee-empty"><FolderKanban size={27}/><strong>No projects yet</strong><span>Projects you join will appear here.</span></div>}
            </aside>
          </section>
        </>}

        {activeNav === "Tasks" && <section className="panel employee-panel employee-list-panel"><div className="panel-heading employee-list-heading"><div><h2>Assigned tasks</h2><p>{filteredTasks.length} {filteredTasks.length === 1 ? "task" : "tasks"} in this view</p></div><label className="employee-filter-label">Status<select value={filter} onChange={event => { setFilter(event.target.value); setTaskPage(1) }}><option>All</option><option>To do</option><option>In progress</option><option>In review</option><option>Completed</option><option>Changes requested</option><option>Due soon</option></select></label></div>
          {loading ? <div className="employee-empty">Loading your tasks…</div> : visibleTasks.length ? <div className="employee-task-list">{visibleTasks.map(renderTask)}</div> : <div className="employee-empty"><ListTodo size={28}/><strong>No tasks in this view</strong><span>Try another status filter or check back when work is assigned.</span></div>}
          <PaginationControls page={taskPage} pageCount={taskPageCount} onPageChange={setTaskPage}/>
        </section>}

        {activeNav === "Projects" && <section className="panel employee-panel employee-list-panel"><div className="panel-heading"><div><h2>Projects you’re on</h2><p>{projects.length} {projects.length === 1 ? "project" : "projects"} connected to your account</p></div></div>
          {loading ? <div className="employee-empty">Loading projects…</div> : visibleProjects.length ? <div className="employee-project-grid">{visibleProjects.map(project => <article className="employee-project-card" key={project.name}><div className="employee-project-heading"><span className="employee-project-icon"><FolderKanban size={17}/></span><div><strong>{project.name}</strong><span>{project.code} · Due {project.due}</span></div></div><p className="employee-project-summary">{project.completed} of {project.total} tasks approved</p><div className="progress-track"><div className="progress-fill" style={{ width: `${Math.max(0, Math.min(100, project.progress))}%` }}/></div><div className="employee-project-foot"><span>{project.progress}% complete</span><span>{project.members.length} team members</span></div></article>)}</div> : <div className="employee-empty"><FolderKanban size={28}/><strong>No projects yet</strong><span>You’ll see projects here when you’re added as a member.</span></div>}
          <PaginationControls page={projectPage} pageCount={projectPageCount} onPageChange={setProjectPage}/>
        </section>}

        {(activeNav === "Settings" || activeNav === "Help & support") && <section className="panel employee-panel employee-list-panel"><div className="panel-heading"><div><h2>{activeNav}</h2><p>{activeNav === "Settings" ? "Your current account information." : "A few quick pointers for your workspace."}</p></div></div>
          {activeNav === "Settings" ? <div className="employee-account-card"><Avatar initials={initials} index={1}/><div><strong>{user.full_name}</strong><span>{user.email}</span><small>Employee account</small></div></div> : <div className="employee-help-list"><article><strong>Update task progress</strong><span>Open Tasks and use Start task or Submit for review as you work.</span></article><article><strong>Check requested changes</strong><span>When a task needs edits, admin feedback appears directly on its task card.</span></article><article><strong>Need access or help?</strong><span>Contact your workspace administrator for project membership or account support.</span></article></div>}
        </section>}

        <footer className="employee-page-footer"><ShieldCheck size={15}/> Signed in as {user.email} · Your workspace data is scoped to your account.</footer>
        </>}
      </div>
    </main>
  </div>
}
