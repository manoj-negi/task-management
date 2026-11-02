Office Task Manager — frontend login integration

Included files:
- frontend/src/services/auth.ts
- frontend/src/components/AuthGate.tsx
- frontend/src/components/Login.css
- frontend/src/main.tsx (replacement)
- frontend/.env.example

Apply the archive at ~/office-task-manager and merge/overwrite included files.
Keep App.tsx, App.css, Sidebar, task UI and mock data unchanged.

Create frontend/.env containing:
VITE_API_BASE_URL=http://localhost:8080

Then run:
cd ~/office-task-manager/frontend
npm run build
npm run dev

Open the Vite URL printed in the terminal and sign in using your admin account.

The JWT and user profile are stored in sessionStorage and removed when Sign out is clicked or the browser session ends. The dashboard still displays sample/mock task data; task CRUD and protected backend endpoints are separate next steps. Do not commit .env or share passwords/JWTs. Rotate credentials or tokens exposed in chat. For production, use HTTPS, reviewed token storage (prefer a backend-managed secure HttpOnly cookie where feasible), and server-side authorization on every protected API.
