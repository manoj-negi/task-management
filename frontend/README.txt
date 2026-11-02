# Office Task Manager — Frontend refactor batch 1

This batch refactors the current working dashboard without changing `App.css`.

## Files included
- `src/App.tsx` — updated to import the extracted types, mock data, reusable task UI components, and Sidebar.
- `src/types/task.types.ts` — task and project types.
- `src/data/mockData.ts` — current demo tasks and projects moved out of App.tsx.
- `src/components/TaskUI.tsx` — Avatar, StatusBadge, PriorityLabel.
- `src/layouts/Sidebar.tsx` — extracted sidebar.

## Apply
From the `frontend` directory, back up the current files first:
```bash
cp src/App.tsx src/App.tsx.backup
cp src/layouts/Sidebar.tsx src/layouts/Sidebar.tsx.backup 2>/dev/null || true
```
Then copy the files from this zip into the `frontend` directory, preserving the `src/` folder structure. Keep your existing `src/App.css`, `src/index.css`, and other files.

## Verify
```bash
npm run dev
```
Check that the dashboard renders, the sidebar navigation works, and task creation still works. Then run `npm run build` to catch TypeScript issues.
