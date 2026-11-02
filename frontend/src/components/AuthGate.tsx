import { useEffect, useState, type FormEvent } from "react"
import {
  ArrowRight,
  Eye,
  EyeOff,
  LockKeyhole,
  ShieldCheck,
} from "lucide-react"
import App from "../App"
import EmployeeDashboard from "./EmployeeDashboard"
import {
  login,
  getCurrentUser,
  logout,
  ApiRequestError,
  type AuthUser,
} from "../services/auth"
import "./Login.css"

export default function AuthGate() {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [checkingSession, setCheckingSession] = useState(true)
  const [retryCount, setRetryCount] = useState(0)

  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [sessionError, setSessionError] = useState("")

  useEffect(() => {
    // Remove JWTs saved by older frontend versions; authentication now uses an HttpOnly cookie.
    sessionStorage.removeItem("office_task_manager_token")
  }, [])

  useEffect(() => {
    let cancelled = false

    async function validateSession() {
      setCheckingSession(true)
      setSessionError("")

      try {
        const currentUser = await getCurrentUser()

        if (cancelled) return

        setUser(currentUser)
      } catch (err) {
        if (cancelled) return

        if (err instanceof ApiRequestError && err.status === 401) {
          setUser(null)
          setError("")
        } else {
          setSessionError(
            err instanceof Error
              ? err.message
              : "Unable to verify your session.",
          )
        }
      } finally {
        if (!cancelled) setCheckingSession(false)
      }
    }

    void validateSession()

    return () => {
      cancelled = true
    }
  }, [retryCount])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError("")
    setSessionError("")
    setLoading(true)

    try {
      const result = await login(email, password)
      setUser(result.user)
      setPassword("")
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Unable to sign in. Please try again.",
      )
    } finally {
      setLoading(false)
    }
  }

  function handleLogout() {
    void logout()
    setUser(null)
    setPassword("")
    setError("")
    setSessionError("")
    setCheckingSession(false)
  }

  if (checkingSession) {
    return (
      <main className="login-page">
        <div className="login-form-wrap">
          <ShieldCheck size={28} />
          <h2>Verifying your session…</h2>
          <p>Please wait while we securely verify your account.</p>
        </div>
      </main>
    )
  }

  if (sessionError) {
    return (
      <main className="login-page">
        <div className="login-form-wrap">
          <ShieldCheck size={28} />
          <h2>Unable to verify your session</h2>
          <p className="login-error" role="alert">{sessionError}</p>
          <button
            className="login-submit"
            type="button"
            onClick={() => setRetryCount((count) => count + 1)}
          >
            Try again
          </button>
          <button
            type="button"
            onClick={handleLogout}
          >
            Sign out
          </button>
        </div>
      </main>
    )
  }

  if (user) {
    return (
      user.role === "employee" ? (
        <EmployeeDashboard user={user} onSignOut={handleLogout} />
      ) : <App user={user} onSignOut={handleLogout} />
    )
  }

  return (
    <main className="login-page">
      <section className="login-visual" aria-label="Office Task Manager">
        <div className="login-brand">
          <span className="login-brand-mark">
            <LockKeyhole size={21} />
          </span>
          <span>
            Workboard<span className="login-brand-period">.</span>
          </span>
        </div>

        <div className="login-visual-copy">
          <span className="login-eyebrow">
            <span /> YOUR WORK, IN SYNC
          </span>
          <h1>Make great work<br />happen together.</h1>
          <p>
            A clearer view of your tasks, projects, and team progress — all in
            one workspace.
          </p>
        </div>

        <div className="login-visual-footer">
          <ShieldCheck size={16} />
          <span>Secure access for your team</span>
        </div>

        <div className="login-orb login-orb-one" />
        <div className="login-orb login-orb-two" />
      </section>

      <section className="login-panel">
        <div className="login-form-wrap">
          <div className="login-mobile-brand">
            <span className="login-brand-mark">
              <LockKeyhole size={19} />
            </span>
            <strong>
              Workboard<span className="login-brand-period">.</span>
            </strong>
          </div>

          <span className="login-form-eyebrow">WELCOME BACK</span>
          <h2>Sign in to your workspace</h2>
          <p className="login-form-subtitle">
            Enter your work account details to continue.
          </p>

          <form className="login-form" onSubmit={handleSubmit}>
            <label>
              Work email
              <input
                type="email"
                name="email"
                autoComplete="username"
                placeholder="you@company.com"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
                autoFocus
              />
            </label>

            <label>
              Password
              <span className="login-password-wrap">
                <input
                  type={showPassword ? "text" : "password"}
                  name="password"
                  autoComplete="current-password"
                  placeholder="Enter your password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  required
                />
                <button
                  className="login-password-toggle"
                  type="button"
                  onClick={() => setShowPassword((visible) => !visible)}
                  aria-label={showPassword ? "Hide password" : "Show password"}
                >
                  {showPassword ? <EyeOff size={17} /> : <Eye size={17} />}
                </button>
              </span>
            </label>

            {error && <div className="login-error" role="alert">{error}</div>}

            <button className="login-submit" type="submit" disabled={loading}>
              {loading ? "Signing in…" : "Sign in"}
              {!loading && <ArrowRight size={17} />}
            </button>
          </form>

          <div className="login-security-note">
            <LockKeyhole size={14} />
            <span>Your connection is used only to authenticate your account.</span>
          </div>
        </div>

        <p className="login-copyright">
          © 2026 Workboard. For authorized team members only.
        </p>
      </section>
    </main>
  )
}
