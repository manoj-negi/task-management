import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react"
import { ArrowLeft, CalendarDays, CheckCircle2, CircleAlert, Clock3, Download, FileText, Link2, LoaderCircle, MessageSquare, Paperclip, Send, ShieldCheck, Upload, UserRound, Workflow } from "lucide-react"
import PaginationControls from "./PaginationControls"
import { Avatar } from "./TaskUI"
import { addTaskComment, downloadTaskAttachment, fetchTaskActivity, fetchTaskComments, fetchTaskDetails, reviewAdminTask, updateSubtaskStatus, updateTaskStatus, uploadTaskAttachment, type TaskAttachment, type TaskComment, type TaskDetailsResponse, type TaskTimelineEvent } from "../services/taskDetails"
import { performTaskAction } from "../services/tasks"
import { createAdminSubtask } from "../services/taskManagement"
import { fetchAssignableUsers, type AssignableUser } from "../services/dashboard"

type Tab = "Overview" | "Subtasks" | "Comments" | "Submissions" | "Activity"
const pageSize = 10

const labelStatus = (status: string) => ({ todo: "To do", in_progress: "In progress", in_review: "Submitted for review", changes_requested: "Changes requested", approved: "Completed", blocked: "Blocked", cancelled: "Cancelled" }[status] ?? status)
const readableAction = (action: string) => ({ task_created: "created this task", task_assigned: "assigned this task", task_started: "started working", task_submitted: "submitted work for review", task_status_changed: "changed the task status", task_details_updated: "updated task details or assignees", task_deleted: "removed this task", subtask_created: "added a subtask", subtask_status_changed: "updated a subtask", comment_added: "commented", attachment_uploaded: "uploaded an attachment", approved: "approved the work", changes_requested: "requested changes" }[action] ?? action.replaceAll("_", " "))
const formatDate = (value: string) => value ? new Date(value).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }) : "Not set"
const formatSize = (size: number) => size < 1024 * 1024 ? `${Math.max(1, Math.round(size / 1024))} KB` : `${(size / (1024 * 1024)).toFixed(1)} MB`

export default function EmployeeTaskDetails({ taskId, onBack, backLabel, adminMode = false, onEdit, onDelete, onOpenTask }: { taskId: string; onBack: () => void; backLabel?: string; adminMode?: boolean; onEdit?: () => void; onDelete?: () => void; onOpenTask?: (taskId: string) => void }) {
  const [data, setData] = useState<TaskDetailsResponse | null>(null)
  const [comments, setComments] = useState<TaskComment[]>([])
  const [activity, setActivity] = useState<TaskTimelineEvent[]>([])
  const [tab, setTab] = useState<Tab>("Overview")
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [notice, setNotice] = useState("")
  const [busy, setBusy] = useState("")
  const [comment, setComment] = useState("")
  const [replyTo, setReplyTo] = useState<TaskComment | null>(null)
  const [summary, setSummary] = useState("")
  const [referenceLinks, setReferenceLinks] = useState("")
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [submissionAttachmentIds, setSubmissionAttachmentIds] = useState<string[]>([])
  const [commentPage, setCommentPage] = useState(1)
  const [activityPage, setActivityPage] = useState(1)
  const [subtaskDraftID, setSubtaskDraftID] = useState("")
  const [subtaskSummary, setSubtaskSummary] = useState("")
  const [reviewFeedback, setReviewFeedback] = useState("")
  const [showSubtaskForm, setShowSubtaskForm] = useState(false)
  const [subtaskTitle, setSubtaskTitle] = useState("")
  const [subtaskDescription, setSubtaskDescription] = useState("")
  const [subtaskDueDate, setSubtaskDueDate] = useState("")
  const [subtaskPriority, setSubtaskPriority] = useState<"low" | "medium" | "high" | "urgent">("medium")
  const [projectUsers, setProjectUsers] = useState<AssignableUser[]>([])
  const [subtaskAssignees, setSubtaskAssignees] = useState<string[]>([])

  const refresh = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      // Task details are the primary view. A comments/activity endpoint issue
      // should not make the whole task page appear unavailable.
      const [detailsResult, commentsResult, activityResult] = await Promise.allSettled([
        fetchTaskDetails(taskId),
        fetchTaskComments(taskId),
        fetchTaskActivity(taskId),
      ])
      if (detailsResult.status === "rejected") throw detailsResult.reason

      setData(detailsResult.value)
      setComments(commentsResult.status === "fulfilled" ? commentsResult.value : [])
      setActivity(activityResult.status === "fulfilled" ? activityResult.value : [])

      const unavailableSections = [
        commentsResult.status === "rejected" ? "comments" : "",
        activityResult.status === "rejected" ? "activity" : "",
      ].filter(Boolean)
      if (unavailableSections.length) {
        setError(`Task loaded, but ${unavailableSections.join(" and ")} could not be loaded. You can still use the task details.`)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load task details.")
    } finally {
      setLoading(false)
    }
  }, [taskId])

  useEffect(() => { void refresh() }, [refresh])
  useEffect(() => {
    if (!adminMode || !data?.task.project) return
    fetchAssignableUsers(data.task.project).then(setProjectUsers).catch(err => setError(err instanceof Error ? err.message : "Could not load project members."))
  }, [adminMode, data?.task.project])

  const pagedComments = useMemo(() => comments.slice((commentPage - 1) * pageSize, commentPage * pageSize), [comments, commentPage])
  const pagedActivity = useMemo(() => activity.slice((activityPage - 1) * pageSize, activityPage * pageSize), [activity, activityPage])
  const task = data?.task
  const dueTime = task?.dueDate ? new Date(task.dueDate).getTime() : NaN
  const now = Date.now()
  const overdue = Number.isFinite(dueTime) && dueTime < now && task?.status !== "approved" && task?.status !== "cancelled"
  const dueSoon = Number.isFinite(dueTime) && dueTime >= now && dueTime - now <= 3 * 24 * 60 * 60 * 1000
  const canSubmit = !!task && task.myWorkStatus === "in_progress" && (task.status === "in_progress" || task.status === "changes_requested")

  async function changeStatus(status: "blocked" | "in_progress") {
    if (!data) return
    if (status === "blocked" && !window.confirm("Mark this task as blocked? Your team will see the status change.")) return
    setBusy("status"); setError(""); setNotice("")
    try { const result = await updateTaskStatus(taskId, status); setNotice(result.message); await refresh() }
    catch (err) { setError(err instanceof Error ? err.message : "Could not change task status.") }
    finally { setBusy("") }
  }

  async function saveComment(event: FormEvent) {
    event.preventDefault()
    if (!comment.trim()) return
    setBusy("comment"); setError(""); setNotice("")
    try { const result = await addTaskComment(taskId, comment, replyTo?.id); setComment(""); setReplyTo(null); setNotice(result.message); await refresh() }
    catch (err) { setError(err instanceof Error ? err.message : "Could not add your comment.") }
    finally { setBusy("") }
  }

  async function uploadFile() {
    if (!selectedFile) return
    setBusy("upload"); setError(""); setNotice("")
    try { const result = await uploadTaskAttachment(taskId, selectedFile); setSubmissionAttachmentIds(current => [...current, result.attachment.id]); setSelectedFile(null); setNotice("Attachment uploaded."); await refresh() }
    catch (err) { setError(err instanceof Error ? err.message : "Could not upload this file.") }
    finally { setBusy("") }
  }

  async function download(attachment: TaskAttachment) {
    setBusy(`download:${attachment.id}`); setError("")
    try { await downloadTaskAttachment(taskId, attachment) }
    catch (err) { setError(err instanceof Error ? err.message : "Could not download this file.") }
    finally { setBusy("") }
  }

  async function submitWork(event: FormEvent) {
    event.preventDefault()
    if (!summary.trim()) { setError("Add a work summary before submitting for review."); return }
    if (!canSubmit) return
    if (!window.confirm("Submit this work summary for admin review?")) return
    setBusy("submit"); setError(""); setNotice("")
    try {
      const attachmentIds = [...submissionAttachmentIds]
      if (selectedFile) {
        const uploaded = await uploadTaskAttachment(taskId, selectedFile)
        attachmentIds.push(uploaded.attachment.id)
        setSelectedFile(null)
      }
      const links = referenceLinks.split(/[\n,]+/).map(value => value.trim()).filter(Boolean)
      const result = await performTaskAction(taskId, "submit", { summary: summary.trim(), referenceLinks: links, attachmentIds })
      setSummary(""); setReferenceLinks(""); setSubmissionAttachmentIds([]); setNotice(result.message); await refresh()
    } catch (err) { setError(err instanceof Error ? err.message : "Could not submit this work.") }
    finally { setBusy("") }
  }

  async function changeSubtask(id: string, status: string, workSummary?: string) {
    if (status === "blocked" && !window.confirm("Mark this subtask as blocked?")) return
    setBusy(`subtask:${id}`); setError("")
    try { const result = await updateSubtaskStatus(taskId, id, status, workSummary); setSubtaskDraftID(""); setSubtaskSummary(""); setNotice(result.message); await refresh() }
    catch (err) { setError(err instanceof Error ? err.message : "Could not update subtask.") }
    finally { setBusy("") }
  }

  async function decideReview(decision: "approved" | "changes_requested") {
    if (!adminMode) return
    if (decision === "changes_requested" && !reviewFeedback.trim()) { setError("Feedback is required when requesting changes."); return }
    if (!window.confirm(decision === "approved" ? "Approve this task submission?" : "Return this task to the employee with your feedback?")) return
    setBusy("review"); setError(""); setNotice("")
    try { const result = await reviewAdminTask(taskId, decision, reviewFeedback); setNotice(result.message); setReviewFeedback(""); await refresh() }
    catch (err) { setError(err instanceof Error ? err.message : "Could not save the review decision.") }
    finally { setBusy("") }
  }

  async function saveSubtask(event: FormEvent) {
    event.preventDefault()
    if (!adminMode || !subtaskTitle.trim()) return
    setBusy("create-subtask"); setError(""); setNotice("")
    try {
      const result = await createAdminSubtask(taskId, { title: subtaskTitle.trim(), description: subtaskDescription, priority: subtaskPriority, dueDate: subtaskDueDate, assignedUserIds: subtaskAssignees })
      setSubtaskTitle(""); setSubtaskDescription(""); setSubtaskDueDate(""); setSubtaskAssignees([]); setShowSubtaskForm(false); setNotice(result.message); await refresh()
    } catch (err) { setError(err instanceof Error ? err.message : "Could not create subtask.") }
    finally { setBusy("") }
  }

  if (loading && !data) return <section className="panel employee-detail-loading"><LoaderCircle size={24} className="employee-spinner"/><strong>Loading task details…</strong><button className="button button-secondary button-small" onClick={onBack}><ArrowLeft size={15}/> {backLabel || "Back to tasks"}</button></section>
  if (!data || !task) return <section className="panel employee-detail-loading"><CircleAlert size={26}/><strong>Task details unavailable</strong><p>{error || "This task could not be found or you no longer have access."}</p><div><button className="button button-secondary" onClick={onBack}><ArrowLeft size={15}/> {backLabel || "Back to tasks"}</button><button className="button button-primary" onClick={() => void refresh()}>Try again</button></div></section>

  const tabs: Tab[] = adminMode ? ["Overview", "Subtasks", "Comments", "Submissions", "Activity"] : ["Overview", "Subtasks", "Comments", "Activity"]
  return <section className="employee-task-details">
    <button className="employee-back-button" onClick={onBack}><ArrowLeft size={16}/> {backLabel || (adminMode ? "Back to tasks" : "Back to my tasks")}</button>
    {error && <div className="employee-alert employee-alert-error" role="alert">{error}<button className="button button-secondary button-small" onClick={() => void refresh()}>Retry</button></div>}
    {notice && <div className="employee-alert employee-alert-success" role="status"><CheckCircle2 size={17}/>{notice}</div>}

    <header className="employee-detail-hero panel">
      <div className="employee-detail-title-row"><div className="employee-detail-title"><span className="employee-detail-project"><Workflow size={15}/>{task.project}</span><h1>{task.title}</h1></div><div className="employee-detail-badges"><span className={`employee-status-pill employee-status-${task.status}`}>{labelStatus(task.status)}</span><span className={`employee-priority-pill employee-priority-${task.priority.toLowerCase()}`}>{task.priority} priority</span></div></div>
      <p className="employee-detail-summary">{task.description || "No task description was provided."}</p>
      {(overdue || dueSoon || task.status === "blocked" || task.status === "in_review" || task.status === "changes_requested") && <div className={`employee-deadline-alert ${overdue || task.status === "blocked" ? "is-alert" : ""}`}><CircleAlert size={17}/><span>{overdue ? "This task is overdue." : task.status === "blocked" ? "This task is blocked. Update its status when work can resume." : task.status === "in_review" ? "Work is awaiting admin review." : task.status === "changes_requested" ? "The reviewer requested changes. See the feedback below." : "This task is due within the next three days."}</span></div>}
      {task.reviewFeedback && task.reviewDecision === "changes_requested" && <div className="employee-review-feedback"><strong><ShieldCheck size={16}/> Admin feedback</strong><p>{task.reviewFeedback}</p></div>}
      <div className="employee-detail-progress"><div><span>Progress</span><strong>{task.progress}%</strong></div><div className="progress-track"><div className="progress-fill" style={{ width: `${task.progress}%` }}/></div>{task.subtaskCount > 0 && <small>{task.subtasksDone} of {task.subtaskCount} subtasks completed</small>}</div>
      <div className="employee-detail-actions">
        {adminMode && <><button className="button button-secondary" onClick={onEdit}>Edit task</button><button className="button button-secondary" onClick={onDelete}>Delete task</button></>}
        {!adminMode && (task.status === "todo" || task.status === "changes_requested") && task.myWorkStatus !== "ready_for_review" && <button className="button button-primary" disabled={busy === "start"} onClick={() => { setBusy("start");setError("");performTaskAction(taskId,"start").then(result=>{setNotice(result.message);return refresh()}).catch(err=>setError(err instanceof Error?err.message:"Could not start task.")).finally(()=>setBusy("")) }}>{task.myWorkStatus === "in_progress" ? "Continue task" : "Start task"}</button>}
        {!adminMode && task.status === "blocked" && <button className="button button-primary" disabled={busy === "status"} onClick={() => void changeStatus("in_progress")}>Resume work</button>}
        {task.myWorkStatus === "ready_for_review" && task.status !== "in_review" && <span className="employee-complete-label"><Clock3 size={17}/> Your work is submitted; waiting for other assignees</span>}
        {task.status === "approved" && <span className="employee-complete-label"><CheckCircle2 size={17}/> Approved by reviewer</span>}
      </div>
    </header>

    <nav className="employee-detail-tabs" role="tablist" aria-label="Task details sections">{tabs.map(name => <button key={name} role="tab" aria-selected={tab === name} className={tab === name ? "active" : ""} onClick={() => setTab(name)}>{name}{name === "Subtasks" && task.subtaskCount > 0 ? <span>{data.subtasks.length}</span> : name === "Comments" && comments.length > 0 ? <span>{comments.length}</span> : null}</button>)}</nav>

    {tab === "Overview" && <div className="employee-detail-layout">
      <div className="employee-detail-primary">
        <section className="panel employee-detail-panel"><div className="panel-heading"><div><h2>Task overview</h2><p>Assignment details and current status.</p></div></div><div className="employee-task-info-grid">
          <Info label="Project" value={task.project} icon={<Workflow size={16}/>} />{task.parentTask && <Info label="Parent task" value={task.parentTask} icon={<Workflow size={16}/>} />}<Info label="Start date" value={formatDate(task.startDate)} icon={<CalendarDays size={16}/>} /><Info label="Due date" value={formatDate(task.dueDate)} icon={<Clock3 size={16}/>} /><Info label="Created" value={formatDate(task.createdAt)} icon={<CalendarDays size={16}/>} /><Info label="Assigned by" value={task.assignedBy || task.createdBy} icon={<UserRound size={16}/>} /><Info label="Assigned on" value={formatDate(task.assignedAt)} icon={<CalendarDays size={16}/>} /><Info label="Current progress" value={task.subtaskCount ? `${task.subtasksDone} of ${task.subtaskCount} subtasks done` : `${task.progress}%`} icon={<CheckCircle2 size={16}/>} />
        </div><div className="employee-assignee-list"><h3>Assigned employees</h3>{data.assignees.map(person => <div className="employee-assignee-row" key={person.id}><Avatar initials={person.fullName.split(/\s+/).map(part => part[0]).slice(0,2).join("").toUpperCase()}/><div><strong>{person.fullName}</strong><span>{person.email}</span></div><span className="employee-assignee-work">{labelStatus(person.workStatus === "assigned" ? "todo" : person.workStatus === "ready_for_review" ? "in_review" : "in_progress")}</span></div>)}</div></section>

        <section className="panel employee-detail-panel"><div className="panel-heading"><div><h2>Attachments</h2><p>Files shared with this task.</p></div><label className="button button-secondary button-small employee-upload-label"><Upload size={15}/> Add file<input type="file" accept="image/png,image/jpeg,image/gif,image/webp,application/pdf,text/plain,text/csv,.doc,.docx,.xls,.xlsx,.ppt,.pptx" onChange={event => setSelectedFile(event.target.files?.[0] ?? null)}/></label></div>
          {selectedFile && <div className="employee-selected-file"><span><Paperclip size={15}/>{selectedFile.name} · {formatSize(selectedFile.size)}</span><button className="button button-primary button-small" disabled={busy === "upload"} onClick={() => void uploadFile()}>{busy === "upload" ? "Uploading…" : "Upload"}</button></div>}
          {data.attachments.length ? <div className="employee-attachment-list">{data.attachments.map(file => <div className="employee-attachment-row" key={file.id}><span className="employee-file-icon"><FileText size={17}/></span><div className="employee-attachment-copy"><strong>{file.name}</strong><span>{file.uploader} · {formatDate(file.uploadedAt)} · {formatSize(file.size)}{data.latestSubmission.attachmentIds.includes(file.id) ? " · Included in latest submission" : ""}</span></div><button className="icon-button" aria-label={`Download ${file.name}`} disabled={busy === `download:${file.id}`} onClick={() => void download(file)}><Download size={16}/></button></div>)}</div> : <div className="employee-inline-empty">No files attached to this task yet.</div>}
        </section>

        {canSubmit && <form className="panel employee-detail-panel employee-submission-form" onSubmit={submitWork}><div className="panel-heading"><div><h2>Submit work for review</h2><p>Include a short summary of what you completed.</p></div></div><label>Work summary<textarea required minLength={1} maxLength={10000} value={summary} onChange={event => setSummary(event.target.value)} placeholder="Describe the work completed, decisions made, or anything the reviewer should know."/></label><label>Reference links <span>Optional · one URL per line</span><textarea value={referenceLinks} onChange={event => setReferenceLinks(event.target.value)} placeholder="https://…" rows={3}/></label><div className="employee-submit-footer"><span>Optional attachments can be added above.</span><button className="button button-primary" disabled={busy === "submit"}>{busy === "submit" ? "Submitting…" : <><Send size={15}/> Submit for review</>}</button></div></form>}
        {data.latestSubmission.summary && <section className="panel employee-detail-panel"><div className="panel-heading"><div><h2>Latest work summary</h2><p>Submitted {formatDate(data.latestSubmission.submittedAt)}</p></div></div><p className="employee-last-summary">{data.latestSubmission.summary}</p>{data.latestSubmission.referenceLinks.map(link => <a className="employee-reference-link" href={link} target="_blank" rel="noreferrer" key={link}><Link2 size={14}/>{link}</a>)}</section>}
      </div>
      <aside className="employee-detail-secondary">{adminMode ? <section className="panel employee-detail-panel employee-status-controls"><div className="panel-heading"><div><h2>Review submission</h2><p>{task.status === "in_review" ? "Employee work is awaiting your decision." : "Review controls are available while work is in review."}</p></div></div>{task.status === "in_review" && <><label>Feedback<textarea value={reviewFeedback} onChange={event => setReviewFeedback(event.target.value)} maxLength={5000} placeholder="Required when requesting changes…"/></label><button className="button button-primary" disabled={busy === "review"} onClick={() => void decideReview("approved")}>Approve work</button><button className="button button-secondary" disabled={busy === "review" || !reviewFeedback.trim()} onClick={() => void decideReview("changes_requested")}>Request changes</button></>}</section> : <section className="panel employee-detail-panel employee-status-controls"><div className="panel-heading"><div><h2>My progress</h2><p>Update your task state.</p></div></div>{task.status === "blocked" && <button className="button button-primary" disabled={busy === "status"} onClick={() => void changeStatus("in_progress")}>Resume work</button>}{task.myWorkStatus === "in_progress" && (task.status === "in_progress" || task.status === "changes_requested") && <button className="button button-secondary" disabled={busy === "status"} onClick={() => void changeStatus("blocked")}>Mark as blocked</button>}<small>Task completion is recorded by the reviewer after approval.</small></section>}
        <section className="panel employee-detail-panel"><div className="panel-heading"><div><h2>Review state</h2><p>Approval workflow</p></div></div><div className="employee-review-state"><span className={`employee-review-state-icon ${task.status === "approved" ? "complete" : task.status === "in_review" ? "review" : ""}`}>{task.status === "approved" ? <CheckCircle2 size={17}/> : task.status === "in_review" ? <Clock3 size={17}/> : <ShieldCheck size={17}/>}</span><div><strong>{task.status === "approved" ? "Approved" : task.status === "in_review" ? "Waiting for review" : task.status === "changes_requested" ? "Changes requested" : "Review after submission"}</strong><span>{task.status === "approved" ? "Your task is complete." : task.status === "in_review" ? "The admin will review your submitted work." : "Submit your work when it is ready."}</span></div></div></section>
      </aside>
    </div>}

    {tab === "Subtasks" && <section className="panel employee-detail-panel employee-tab-panel"><div className="panel-heading"><div><h2>Subtasks</h2><p>{adminMode ? "Manage subtasks and their project assignments." : "Update progress for subtasks assigned to you."}</p></div><div className="employee-subtask-admin-tools"><strong className="employee-subtask-progress">{task.subtasksDone}/{task.subtaskCount} complete</strong>{adminMode&&<button className="button button-primary button-small" onClick={()=>setShowSubtaskForm(value=>!value)}>{showSubtaskForm?"Cancel":"Add subtask"}</button>}</div></div>{showSubtaskForm&&adminMode&&<form className="employee-admin-subtask-form" onSubmit={saveSubtask}><label>Subtask title<input required maxLength={250} value={subtaskTitle} onChange={event=>setSubtaskTitle(event.target.value)}/></label><label>Description<textarea maxLength={10000} value={subtaskDescription} onChange={event=>setSubtaskDescription(event.target.value)}/></label><div className="employee-admin-subtask-fields"><label>Priority<select value={subtaskPriority} onChange={event=>setSubtaskPriority(event.target.value as typeof subtaskPriority)}><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option><option value="urgent">Urgent</option></select></label><label>Due date<input type="date" value={subtaskDueDate} onChange={event=>setSubtaskDueDate(event.target.value)}/></label></div><fieldset><legend>Assign project members</legend>{projectUsers.length?projectUsers.map(person=><label className="employee-subtask-member" key={person.id}><input type="checkbox" checked={subtaskAssignees.includes(person.id)} onChange={event=>setSubtaskAssignees(current=>event.target.checked?[...current,person.id]:current.filter(id=>id!==person.id))}/><span>{person.fullName} · {person.email}</span></label>):<small>No active project members are available.</small>}</fieldset><button className="button button-primary" disabled={busy==="create-subtask"}>{busy==="create-subtask"?"Creating…":"Create subtask"}</button></form>}{data.subtasks.length ? <div className="employee-subtask-list">{data.subtasks.map(subtask => <article className="employee-subtask-row" key={subtask.id}><span className={`employee-subtask-check ${subtask.status === "approved" ? "done" : ""}`}>{subtask.status === "approved" ? <CheckCircle2 size={18}/> : <Workflow size={17}/>}</span><div className="employee-subtask-copy"><strong>{subtask.title}</strong><span>{subtask.workStatus === "ready_for_review" ? "Submitted for review" : labelStatus(subtask.status)}{subtask.dueDate ? ` · Due ${formatDate(subtask.dueDate)}` : ""}</span></div>{adminMode&&<button className="button button-secondary button-small" onClick={()=>onOpenTask?.(subtask.id)}>Manage</button>}{subtask.canUpdate && subtask.status !== "approved" && subtask.status !== "in_review" && subtask.workStatus !== "ready_for_review" && <div className="employee-subtask-actions">{subtask.status === "blocked" ? <button className="button button-primary button-small" disabled={busy === `subtask:${subtask.id}`} onClick={() => void changeSubtask(subtask.id,"in_progress")}>Resume</button> : subtask.status === "todo" ? <button className="button button-primary button-small" disabled={busy === `subtask:${subtask.id}`} onClick={() => void changeSubtask(subtask.id,"in_progress")}>Start</button> : <><button className="button button-secondary button-small" disabled={busy === `subtask:${subtask.id}`} onClick={() => void changeSubtask(subtask.id,"blocked")}>Block</button><button className="button button-primary button-small" onClick={() => {setSubtaskDraftID(current=>current===subtask.id?"":subtask.id);setSubtaskSummary("")}}>Submit for review</button></>}</div>}{subtaskDraftID===subtask.id && <form className="employee-subtask-submit" onSubmit={event=>{event.preventDefault();if(!subtaskSummary.trim()){setError("Add a summary before submitting this subtask.");return}void changeSubtask(subtask.id,"in_review",subtaskSummary)}}><textarea required value={subtaskSummary} onChange={event=>setSubtaskSummary(event.target.value)} placeholder="Summarize the completed subtask…"/><button className="button button-primary button-small" disabled={busy===`subtask:${subtask.id}`}>Confirm submission</button></form>}</article>)}</div> : !showSubtaskForm&&<div className="employee-inline-empty">This task has no subtasks assigned yet.</div>}</section>}

    {tab === "Comments" && <section className="panel employee-detail-panel employee-tab-panel"><div className="panel-heading"><div><h2>Comments</h2><p>Discuss task details with the team.</p></div></div><form className="employee-comment-form" onSubmit={saveComment}>{replyTo && <div className="employee-replying"><span>Replying to {replyTo.author}: “{replyTo.comment.slice(0,80)}{replyTo.comment.length>80?"…":""}”</span><button type="button" onClick={() => setReplyTo(null)}>Cancel reply</button></div>}<textarea value={comment} onChange={event => setComment(event.target.value)} maxLength={5000} placeholder="Write a comment…"/><div><small>{comment.length}/5000</small><button className="button button-primary button-small" disabled={!comment.trim() || busy === "comment"}>{busy === "comment" ? "Posting…" : <><MessageSquare size={14}/> Add comment</>}</button></div></form>{comments.length ? <div className="employee-comment-list">{pagedComments.map(item => <article className="employee-comment" key={item.id}><Avatar initials={item.author.split(/\s+/).map(part => part[0]).slice(0,2).join("").toUpperCase()}/><div className="employee-comment-body"><div className="employee-comment-meta"><strong>{item.author}</strong><span className="employee-comment-role">{item.role.replaceAll("_", " ")}</span><time dateTime={item.createdAt}>{formatDate(item.createdAt)}</time></div>{item.parentId && <span className="employee-reply-label">Reply</span>}<p>{item.comment}</p><button className="employee-reply-button" onClick={() => { setReplyTo(item); document.querySelector<HTMLTextAreaElement>(".employee-comment-form textarea")?.focus() }}>Reply</button></div></article>)}</div> : <div className="employee-inline-empty"><MessageSquare size={20}/>No comments yet. Start the conversation.</div>}<PaginationControls page={commentPage} pageCount={Math.max(1,Math.ceil(comments.length/pageSize))} onPageChange={setCommentPage}/></section>}

    {tab === "Submissions" && adminMode && <section className="panel employee-detail-panel employee-tab-panel"><div className="panel-heading"><div><h2>Work submissions</h2><p>Submission summaries, links, and review decisions.</p></div></div>{data.submissions?.length ? <div className="employee-submission-history">{data.submissions.map(submission => <article className="employee-submission-history-item" key={submission.id}><div className="employee-comment-meta"><strong>{submission.submittedBy}</strong><time>{formatDate(submission.submittedAt)}</time></div><p>{submission.summary}</p>{submission.referenceLinks.map(link => <a className="employee-reference-link" href={link} target="_blank" rel="noreferrer" key={link}><Link2 size={14}/>{link}</a>)}{data.attachments.filter(file => submission.attachmentIds.includes(file.id)).map(file => <button className="employee-reference-link" key={file.id} onClick={() => void download(file)}><Download size={14}/>{file.name}</button>)}</article>)}</div> : <div className="employee-inline-empty">No work has been submitted yet.</div>}{data.reviews?.length ? <><h3>Review history</h3><div className="employee-timeline">{data.reviews.map((review,index)=><article className="employee-timeline-item" key={`${review.createdAt}-${index}`}><span className="employee-timeline-dot"/><div><div className="employee-timeline-heading"><strong>{review.reviewer}</strong><span>{review.decision === "approved" ? "approved the work" : "requested changes"}</span></div>{review.feedback&&<p>{review.feedback}</p>}<time>{formatDate(review.createdAt)}</time></div></article>)}</div></>:null}</section>}
    {tab === "Activity" && <section className="panel employee-detail-panel employee-tab-panel"><div className="panel-heading"><div><h2>Activity timeline</h2><p>Task events in chronological order.</p></div></div>{activity.length ? <div className="employee-timeline">{pagedActivity.map(item => <article className="employee-timeline-item" key={item.id}><span className="employee-timeline-dot"/><div><div className="employee-timeline-heading"><strong>{item.actor}</strong><span>{readableAction(item.action)}</span></div>{item.detail && <p>{item.detail}</p>}<time dateTime={item.createdAt}>{formatDate(item.createdAt)}</time></div></article>)}</div> : <div className="employee-inline-empty">No activity has been recorded yet.</div>}<PaginationControls page={activityPage} pageCount={Math.max(1,Math.ceil(activity.length/pageSize))} onPageChange={setActivityPage}/></section>}
  </section>
}

function Info({ label, value, icon }: { label: string; value: string; icon: ReactNode }) {
  return <div className="employee-task-info"><span className="employee-info-icon">{icon}</span><div><small>{label}</small><strong>{value || "Not set"}</strong></div></div>
}
