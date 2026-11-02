export interface AuthUser {
  id: string
  full_name: string
  email: string
  role: string
  can_assign_tasks: boolean
}

export interface LoginResponse {
  user: AuthUser
}

interface ApiError {
  error?: string | { message?: string }
}

export class ApiRequestError extends Error {
  public status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = "ApiRequestError"
    this.status = status
  }
}

const API_BASE_URL = (
  import.meta.env.VITE_API_BASE_URL || "http://localhost:8080"
).replace(/\/$/, "")

async function readResponse(response: Response) {
  const data = await response.json().catch(() => ({}))

  if (!response.ok) {
    const errorData = data as ApiError
    const message =
      typeof errorData.error === "string"
        ? errorData.error
        : errorData.error?.message || "Request failed."

    throw new ApiRequestError(message, response.status)
  }

  return data
}

export async function login(
  email: string,
  password: string,
): Promise<LoginResponse> {
  let response: Response

  try {
    response = await fetch(`${API_BASE_URL}/api/auth/login`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: email.trim(),
        password,
      }),
    })
  } catch {
    throw new Error(
      "Cannot connect to the server. Make sure the Go backend is running.",
    )
  }

  const data = (await readResponse(response)) as LoginResponse

  if (!data.user) {
    throw new Error("The server returned an unexpected login response.")
  }

  return data
}

export async function getCurrentUser(): Promise<AuthUser> {
  let response: Response

  try {
    response = await fetch(`${API_BASE_URL}/api/auth/me`, {
      method: "GET",
      credentials: "include",
    })
  } catch {
    throw new Error(
      "Cannot connect to the server. Check that the backend is running.",
    )
  }

  const data = (await readResponse(response)) as { user: AuthUser }

  if (!data.user?.id) {
    throw new Error("The server returned an unexpected user response.")
  }

  return data.user
}

export async function logout(): Promise<void> {
  try {
    await fetch(`${API_BASE_URL}/api/auth/logout`, {
      method: "POST",
      credentials: "include",
    })
  } catch {
    // Clear local UI state even if the server cannot be reached.
  }
}
