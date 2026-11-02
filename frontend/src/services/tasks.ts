import { ApiRequestError } from "./auth"

const API_BASE_URL = (
  import.meta.env.VITE_API_BASE_URL || "http://localhost:8080"
).replace(/\/$/, "")

export async function performTaskAction(
  taskId: string,
  action: "start" | "submit",
  submission?: { summary: string; referenceLinks: string[]; attachmentIds?: string[] },
): Promise<{ message: string }> {
  let response: Response
  try {
    response = await fetch(
      `${API_BASE_URL}/api/tasks/${encodeURIComponent(taskId)}/${action}`,
      {
        method: "POST",
        credentials: "include",
        ...(action === "submit" ? { headers: { "Content-Type": "application/json" }, body: JSON.stringify(submission ?? {}) } : {}),
      },
    )
  } catch {
    throw new Error("Cannot connect to the server. Make sure the backend is running.")
  }

  const data = await response.json().catch(() => ({})) as { message?: string; error?: string }
  if (!response.ok) {
    throw new ApiRequestError(data.error || "Could not update task.", response.status)
  }
  return { message: data.message || "Task updated." }
}
