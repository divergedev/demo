# 🎬 Diverge Bank Demo — Recording Script

> 10-minute live demo script for KubeCon, screencasts, or developer advocacy.

## Pre-demo Checklist
- [ ] GKE cluster running with Diverge controller
- [ ] Baseline services deployed
- [ ] Terminal: large font, dark theme
- [ ] Browser: dashboard open at http://$GATEWAY_IP/
- [ ] Second terminal ready for diverge dev

---

## Act 1: The Problem (1:00)

**[TALKING POINT]** "Your team runs 50 microservices. A developer changes one service and opens a PR. To test it, you clone the entire staging environment — that's 15 minutes and $47/day per environment. There has to be a better way. Let's look at our baseline."

**[TERMINAL]**
```bash
# Show baseline — everything is v1
nix develop -c curl -s http://$GATEWAY_IP/api/payments | jq .
```

**[EXPECTED OUTPUT]**
```json
{
  "service": "payments-api",
  "version": "baseline",
  "payments": [
    { "id": "tx_123", "amount": 100 }
  ]
}
```

---

## Act 2: Create Preview (2:00)

**[TALKING POINT]** "Instead of cloning everything, Diverge uses a mesh-first approach. We'll create a `PreviewGroup` that overrides just the two services we changed: the frontend `payments-module` and the backend `payments-api`."

**[TERMINAL]**
```bash
nix develop -c diverge preview create --name fraud-detection \
  --service payments-api=ghcr.io/divergedev/demo-payments-api:preview-42 \
  --service payments-module=ghcr.io/divergedev/demo-payments-module:preview-42 \
  --header-key x-preview-id --header-value 42
```

**[EXPECTED OUTPUT]**
```
🚀 Created PreviewGroup fraud-detection
Routing: x-preview-id: 42
Services:
  - payments-api (ghcr.io/divergedev/demo-payments-api:preview-42)
  - payments-module (ghcr.io/divergedev/demo-payments-module:preview-42)
```

**[TALKING POINT]** "We only deployed two containers. The rest of the cluster is untouched."

---

## Act 3: Header Routing (2:00)

**[TALKING POINT]** "Let's see header-based routing in action. Normal traffic hits the baseline. If we pass the preview header, Diverge routes the request down the stack to our new code."

**[TERMINAL]**
```bash
# Without header
nix develop -c curl -s http://$GATEWAY_IP/api/payments | jq .version
```

**[EXPECTED OUTPUT]**
```json
"baseline"
```

**[TERMINAL]**
```bash
# With header
nix develop -c curl -s -H "x-preview-id: 42" http://$GATEWAY_IP/api/payments | jq .version
```

**[EXPECTED OUTPUT]**
```json
"preview-42"
```

**[TALKING POINT]** "In the browser, passing `x-preview-id: 42` dynamically loads the preview React module. You can see the new fraud detection banner and fee column!"

---

## Act 4: Local Dev + Hot Reload (2:00)

**[TALKING POINT]** "What if I want to debug my API locally, but still interact with the rest of the cluster? I can run `diverge dev` to route preview traffic right to my laptop."

**[TERMINAL]**
```bash
cd bank-demo/services/payments-api
nix develop -c diverge preview dev --service payments-api -- air
```

**[EXPECTED OUTPUT]**
```
Intercepting preview traffic for payments-api...
Starting air...
watching .
building...
running...
```

**[TALKING POINT]** "Now, when I hit the cluster with the preview header, it hits my local Go code. I can make an edit, Air hot-reloads it instantly, and the change is live."

---

## Act 5: DevSpace (1:00)

**[TALKING POINT]** "For our React frontend, we can use DevSpace to sync files directly into the cluster container, giving us instant Vite hot module replacement (HMR)."

**[TERMINAL]**
```bash
# In a second terminal
cd bank-demo/services/payments-module
nix develop -c diverge preview dev --service payments-module -- devspace dev
```

**[EXPECTED OUTPUT]**
```
Intercepting preview traffic for payments-module...
Starting devspace dev...
Syncing files to cluster...
Vite HMR ready.
```

**[TALKING POINT]** "Any edit to the React component immediately updates the browser UI, without rebuilding images."

---

## Act 6: Ops (1:00)

**[TALKING POINT]** "As a platform engineer, you have full visibility into these preview environments."

**[TERMINAL]**
```bash
nix develop -c diverge preview status
```

**[EXPECTED OUTPUT]**
```
NAME              HEADER             SERVICES                 AGE
fraud-detection   x-preview-id: 42   payments-api, module     5m
```

**[TERMINAL]**
```bash
nix develop -c diverge preview logs fraud-detection
```

**[EXPECTED OUTPUT]**
```
[payments-api] Starting server on :8080
[payments-api] Serving request /api/payments
```

---

## Act 7: Cleanup (30s)

**[TALKING POINT]** "Once the PR merges, Diverge cleans everything up automatically. Or we can delete it manually."

**[TERMINAL]**
```bash
nix develop -c diverge preview delete fraud-detection
```

**[EXPECTED OUTPUT]**
```
🗑️ Deleted PreviewGroup fraud-detection
```

---

## Act 8: Closing (30s)

**[TALKING POINT]** "We tested a full-stack cross-repo feature using a single header. Zero cloned databases, zero wasted environments. That's Diverge."
