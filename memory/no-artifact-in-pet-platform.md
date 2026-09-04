---
name: no-artifact-in-pet-platform
description: In pet-platform NEVER use the Artifact tool — just generate HTML files normally
metadata: 
  node_type: memory
  type: feedback
---

In the pet-platform project, do NOT publish claude.ai Artifacts (never call the Artifact tool). Just generate/write HTML files to disk normally (e.g. `demo/<feature>.html`) — user opens them locally.

**Why:** platform convention = `demo/` HTML mocks are the source of truth ([[demo-mock-source-of-truth]]); a claude.ai Artifact is a redundant hosted copy the user then has to manually delete. User asked to delete the one made for rainybox and to stop making them (2026-07-01).

**How to apply:** for any HTML deliverable in pet-platform, Write the file only. Skip the Artifact tool and the artifact-design skill's publish step (design fundamentals still fine to apply to the file itself).
