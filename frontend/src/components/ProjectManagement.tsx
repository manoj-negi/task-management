import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { Archive, ArrowLeft, FolderPlus, Pencil, Plus, RefreshCw, Trash2, Users, X } from 'lucide-react'
import { getCurrentUser } from '../services/auth'
import PaginationControls from './PaginationControls'
import {
  addProjectMember, archiveProject, deleteProject, fetchActiveEmployees, fetchProjectMembers,
  fetchProjects, removeProjectMember, saveProject,
  type ManagedProject, type ProjectMember,
} from '../services/projects'

type Employee = { id: string; full_name: string; email: string; is_active: boolean; role: string }
const emptyForm = { name: '', description: '', status: 'planned', startDate: '', dueDate: '' }

export default function ProjectManagement({ onChanged, createRequest }: { onChanged: () => void; createRequest?: number }) {
  const [projects, setProjects] = useState<ManagedProject[]>([])
  const [employees, setEmployees] = useState<Employee[]>([])
  const [members, setMembers] = useState<ProjectMember[]>([])
  const [selected, setSelected] = useState('')
  const [showProjectDetails, setShowProjectDetails] = useState(false)
  const [form, setForm] = useState(emptyForm)
  const [editing, setEditing] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [memberToAdd, setMemberToAdd] = useState('')
  const [selectedMemberIds, setSelectedMemberIds] = useState<string[]>([])
  const [isAdmin, setIsAdmin] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [projectPage, setProjectPage] = useState(1)
  const [memberPage, setMemberPage] = useState(1)

  const refresh = useCallback(async (preferredId?: string) => {
    setLoading(true)
    try {
      const [projectRows, user] = await Promise.all([fetchProjects(), getCurrentUser()])
      setProjects(projectRows)
      setIsAdmin(user.role === 'super_admin')
      const nextId = preferredId || ''
      setSelected(nextId)
      const activeEmployees = user.role === 'super_admin' ? await fetchActiveEmployees() : []
      setEmployees(activeEmployees)
      if (nextId) setMembers(await fetchProjectMembers(nextId))
      else setMembers([])
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load projects.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void refresh() }, [refresh])

  const chooseProject = async (id: string) => {
    setSelected(id)
    setMemberPage(1)
    setShowProjectDetails(true)
    setError('')
    try { setMembers(id ? await fetchProjectMembers(id) : []) }
    catch (e) { setError(e instanceof Error ? e.message : 'Could not load members.') }
  }

  const startCreate = useCallback(() => {
    setEditing('')
    setForm(emptyForm)
    setSelectedMemberIds([])
    setShowProjectDetails(false)
    setSelected('')
    setShowForm(true)
    setError('')
    setNotice('')
  }, [])

  useEffect(() => {
    if (createRequest && isAdmin) startCreate()
  }, [createRequest, isAdmin, startCreate])

  const startEdit = (p: ManagedProject) => {
    setEditing(p.id)
    setForm({ name:p.name, description:p.description, status:p.status, startDate:p.startDate, dueDate:p.dueDate })
    setSelectedMemberIds([])
    setShowForm(true)
    setError('')
    setNotice('')
  }

  const submitProject = async (event: FormEvent) => {
    event.preventDefault()
    if (saving) return
    setSaving(true); setError(''); setNotice('')
    try {
      const saved = await saveProject({ ...form, id: editing || undefined })
      if (!editing && selectedMemberIds.length) {
        const membershipResults = await Promise.allSettled(
          selectedMemberIds.map(userId => addProjectMember(saved.id, userId)),
        )
        const failedMembers = membershipResults.filter(result => result.status === 'rejected').length
        await refresh(saved.id)
        onChanged()
        setShowForm(false)
        setNotice(failedMembers
          ? `Project created, but ${failedMembers} member${failedMembers === 1 ? '' : 's'} could not be added.`
          : 'Project created with selected members.')
        return
      }
      setShowForm(false)
      setNotice(editing ? 'Project updated.' : 'Project created.')
      await refresh(saved.id)
      onChanged()
    } catch (e) { setError(e instanceof Error ? e.message : 'Could not save project.') }
    finally { setSaving(false) }
  }

  const addMember = async () => {
    if (!selected || !memberToAdd) return
    setError(''); setNotice('')
    try {
      await addProjectMember(selected, memberToAdd)
      setMemberToAdd('')
      setMembers(await fetchProjectMembers(selected))
      await refresh(selected)
      onChanged()
      setNotice('Employee added to project.')
    } catch (e) { setError(e instanceof Error ? e.message : 'Could not add member.') }
  }

  const removeMember = async (userId: string) => {
    if (!selected || !window.confirm('Remove this employee from the project? Their existing task history will be preserved.')) return
    setError(''); setNotice('')
    try {
      await removeProjectMember(selected, userId)
      setMembers(await fetchProjectMembers(selected))
      await refresh(selected)
      onChanged()
      setNotice('Employee removed from project.')
    } catch (e) { setError(e instanceof Error ? e.message : 'Could not remove member.') }
  }

  const archive = async (p: ManagedProject) => {
    if (!window.confirm(`Archive "${p.name}"? It will no longer be available for new tasks.`)) return
    try {
      await archiveProject(p.id)
      await refresh(p.id); onChanged(); setNotice('Project archived.')
    } catch (e) { setError(e instanceof Error ? e.message : 'Could not archive project.') }
  }

  const removeProject = async (p: ManagedProject) => {
    if (!window.confirm(`Delete "${p.name}"? Projects with tasks cannot be deleted; archive those instead.`)) return
    try {
      await deleteProject(p.id)
      await refresh(); onChanged(); setNotice('Project deleted.')
    } catch (e) { setError(e instanceof Error ? e.message : 'Could not delete project.') }
  }

  const activeProject = projects.find(p => p.id === selected)
  const projectPageCount = Math.max(1, Math.ceil(projects.length / 10))
  const currentProjectPage = Math.min(projectPage, projectPageCount)
  const visibleProjects = projects.slice((currentProjectPage - 1) * 10, currentProjectPage * 10)
  const memberPageCount = Math.max(1, Math.ceil(members.length / 10))
  const currentMemberPage = Math.min(memberPage, memberPageCount)
  const visibleMembers = members.slice((currentMemberPage - 1) * 10, currentMemberPage * 10)

  return <section className="panel progress-panel project-management-panel" style={{ marginTop:20 }}>
    <div className="panel-heading">
      <div><h2>Project management</h2><p>Create projects, manage membership and track task progress.</p></div>
      {isAdmin && <button className="button button-primary button-small" onClick={startCreate}><FolderPlus size={15}/> New project</button>}
    </div>
    {error && <div role="alert" className="create-error" style={{marginBottom:12}}>{error}</div>}
    {notice && <p role="status" style={{color:'#047857',fontSize:14}}>{notice}</p>}
    {showForm && isAdmin && <form onSubmit={submitProject} className="create-modal project-create-form">
      <div className="modal-heading"><div><span className="modal-icon"><FolderPlus size={18}/></span><div><h2>{editing ? 'Edit project' : 'Create project'}</h2><p>Project details and timeline.</p></div></div><button type="button" className="icon-button" onClick={()=>setShowForm(false)} aria-label="Close form"><X size={18}/></button></div>
      <label className="form-label">Project name<input required minLength={2} maxLength={160} value={form.name} onChange={e=>setForm({...form,name:e.target.value})}/></label>
      <label className="form-label">Description<textarea rows={3} maxLength={5000} value={form.description} onChange={e=>setForm({...form,description:e.target.value})}/></label>
      <label className="form-label">Status<select value={form.status} onChange={e=>setForm({...form,status:e.target.value})}><option value="planned">Planned</option><option value="active">Active</option><option value="on_hold">On hold</option><option value="completed">Completed</option></select></label>
      {!editing && <fieldset className="project-member-picker">
        <legend>Project members</legend>
        <p>Select active employees to add when this project is created.</p>
        {employees.length === 0
          ? <span className="project-member-empty">No active employees available.</span>
          : <div className="project-member-options">{employees.map(employee => <label key={employee.id} className="project-member-option">
            <input type="checkbox" checked={selectedMemberIds.includes(employee.id)} onChange={event => setSelectedMemberIds(current => event.target.checked ? [...current, employee.id] : current.filter(id => id !== employee.id))}/>
            <span><strong>{employee.full_name}</strong><small>{employee.email}</small></span>
          </label>)}</div>}
      </fieldset>}
      <div className="project-date-fields">
        <label className="form-label">Start date<input type="date" value={form.startDate} onChange={e=>setForm({...form,startDate:e.target.value})}/></label>
        <label className="form-label">Due date<input type="date" value={form.dueDate} min={form.startDate || undefined} onChange={e=>setForm({...form,dueDate:e.target.value})}/></label>
      </div>
      <div className="modal-actions"><button type="button" className="button button-secondary" onClick={()=>setShowForm(false)}>Cancel</button><button className="button button-primary" disabled={saving}>{saving?'Saving…':editing?'Save changes':'Create project'}</button></div>
    </form>}
    {loading ? <p className="empty-state">Loading projects…</p> : projects.length === 0 ? <div className="empty-state"><FolderPlus size={20}/><strong>No projects yet</strong><span>{isAdmin?'Create your first project to start organizing tasks.':'No projects have been shared with you yet.'}</span></div> : showProjectDetails && activeProject ? <div className="project-detail-page">
      <button className="button button-secondary project-detail-back" onClick={()=>{setShowProjectDetails(false);setSelected('');setMembers([])}}><ArrowLeft size={16}/> All projects</button>
      <div className="project-detail-card">
        <div className="project-detail-heading"><div><span className="project-detail-eyebrow"><FolderPlus size={14}/> PROJECT DETAILS</span><h2>{activeProject.name}</h2><p>{activeProject.description || 'No description provided.'}</p></div>{isAdmin && <div className="project-actions"><button className="icon-button" title="Edit project" aria-label="Edit project" onClick={()=>startEdit(activeProject)}><Pencil size={16}/></button>{activeProject.status!=='archived'&&<button className="icon-button" title="Archive project" aria-label="Archive project" onClick={()=>void archive(activeProject)}><Archive size={16}/></button>}<button className="icon-button" title="Delete project" aria-label="Delete project" onClick={()=>void removeProject(activeProject)}><Trash2 size={16}/></button></div>}</div>
        <div className="project-detail-meta"><span className={`project-status-pill project-status-${activeProject.status}`}><i/> {activeProject.status.replace('_',' ')}</span><span><strong>Due date</strong>{activeProject.dueDate || 'Not set'}</span><span><strong>Tasks</strong>{activeProject.completedTasks} of {activeProject.totalTasks} complete</span></div>
        <div className="project-members-heading"><div><h2><Users size={17}/> Members <span>{members.length}</span></h2><p>Only active project members can be assigned its tasks.</p></div><button className="icon-button" title="Refresh members" aria-label="Refresh members" onClick={()=>void refresh(selected)}><RefreshCw size={15}/></button></div>
        {isAdmin && activeProject.status!=='archived' && <div className="project-add-member"><select aria-label="Employee to add" value={memberToAdd} onChange={e=>setMemberToAdd(e.target.value)}><option value="">Select active employee…</option>{employees.filter(e=>!members.some(m=>m.id===e.id)).map(e=><option key={e.id} value={e.id}>{e.full_name} · {e.email}</option>)}</select><button className="button button-primary button-small" disabled={!memberToAdd} onClick={()=>void addMember()}><Plus size={15}/> Add member</button></div>}
        <div className="project-member-list">{visibleMembers.map(m=><div className="project-member-row" key={m.id}><div className="assignee-avatar">{m.fullName.split(/\s+/).map(p=>p[0]).slice(0,2).join('').toUpperCase()}</div><div className="project-main"><strong>{m.fullName}</strong><div className="project-meta"><span>{m.email}</span></div></div><span className={`member-account-status ${m.isActive?'member-account-active':''}`}>{m.isActive?'Active':'Inactive'}</span>{isAdmin&&activeProject.status!=='archived'&&<button className="icon-button" title={`Remove ${m.fullName}`} aria-label={`Remove ${m.fullName}`} onClick={()=>void removeMember(m.id)}><X size={15}/></button>}</div>)}{members.length===0&&<p className="project-empty-members">No members yet. Add employees to enable task assignment.</p>}</div>
        <PaginationControls page={currentMemberPage} pageCount={memberPageCount} onPageChange={setMemberPage}/>
      </div>
    </div> : <div className="project-overview">
      <div className="project-list-heading"><div><h3>All projects</h3><p>Select a project to view its details and members.</p></div><span>{projects.length} {projects.length === 1 ? 'project' : 'projects'}</span></div>
      <div className="project-cards-grid">
        {visibleProjects.map(p=><button key={p.id} onClick={()=>void chooseProject(p.id)} className="project-row project-select-card" aria-label={`Open ${p.name}`}>
          <div className={`project-symbol ${p.status==='archived'?'project-symbol-archived':''}`}>{p.name.split(/\s+/).slice(0,3).map(w=>w[0]).join('').toUpperCase()}</div>
          <div className="project-main"><div className="project-title-row"><strong>{p.name}</strong><span>{p.status.replace('_',' ')}</span></div><p className="project-card-description">{p.description || 'No description provided.'}</p><div className="progress-track"><div className="progress-fill" style={{width:`${p.totalTasks?Math.round(p.completedTasks/p.totalTasks*100):0}%`}}/></div><div className="project-meta"><span>{p.completedTasks}/{p.totalTasks} tasks complete</span><span>{p.memberCount} members</span></div></div>
        </button>)}
      </div>
      <PaginationControls page={currentProjectPage} pageCount={projectPageCount} onPageChange={setProjectPage}/>
    </div>}
  </section>
}
