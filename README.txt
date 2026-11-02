Office Task Manager — UI Foundation Patch

Replace these two files in your local project:
- frontend/src/index.css
- frontend/src/App.css

Before replacing, back up or commit your current files. This patch updates shared design tokens and app-shell styling only. It does not change API services, backend code, or authentication behavior.

After copying, run from the frontend directory:
  npm run build
  npm run dev

If the build reports an issue, send the complete error output before making further changes.
