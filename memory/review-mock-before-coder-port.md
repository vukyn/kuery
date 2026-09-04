---
name: review-mock-before-coder-port
description: Always show user the demo/ mock for review BEFORE dispatching coder to port to React
metadata: 
  node_type: memory
  type: feedback
---

After the designer builds/updates a demo HTML mock, STOP and let the user review it before dispatching the coder to port it to React. Do not chain designer → coder automatically.

**Why:** The mock is the design source of truth ([[demo-mock-source-of-truth]]). The user wants to approve the visual design before any React code is written — porting first wastes work if the design needs changes. The user has flagged this omission twice now ("lại quên cho tôi review mock trước khi cho coder làm").

**How to apply:** designer finishes → present the mock to the user (how to open in browser + how to trigger each state + the key design choices) → wait for explicit approval or change requests → only then dispatch coder. If the user requests changes, update the mock first, re-show, then port.
