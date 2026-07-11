# Namespaces and their purpose

The cluster runs the `web` application in three namespaces. **What a namespace is for governs how
carefully you touch it.**

- **`dev`** — a throwaway development namespace. No real users, no data of value. Scale it, restart
  it, delete pods in it freely; this is where experiments and demos run. Downtime here costs nothing.
- **`staging`** — the pre-production mirror. It validates releases before prod. Treat it like a
  rehearsal: routine ops are fine, but coordinate destructive changes because QA may be mid-run.
- **`prod`** — **live customer traffic.** Every pod here is serving real users. Any change to prod
  (scale, restart, delete) is customer-visible and must be treated as a production change: it
  requires approval, and it is never routine.

Rule of thumb: **dev is a sandbox, prod is a hospital.** The further right you go (dev → staging →
prod), the more a mistake costs and the more oversight an action needs.
