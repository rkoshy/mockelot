# Scenarios: The Power of Client-Side Interception

This document illustrates specific scenarios where Mockelot's **SOCKS5 Overlay** and **Container-as-Proxy** patterns provide simplicity and capability that is difficult to achieve with traditional Kubernetes tools (Telepresence, Mirrord) or heavy local environments.

## 1. The "Surgical Strike" (Partial Mocking of Remote Services)

**The Challenge:** You need to test an edge case (e.g., a specific error code) on a remote service that works 99% of the time.

### The K8s/Service-Mesh Way
Requires replacing the entire pod/service. You must replicate the full environment locally.

```
[ Local Machine ]                                [ Remote Cluster ]
+------------------+                             +-----------------------+
|  My App          |  (Interception)             |  Inventory Service    |
|                  | --------------------------> |  (Telepresence Agent) |
|  User Service    | <-------------------------- |  (Real DB Connection) |
|  (Local Clone)   |                             |                       |
+------------------+                             +-----------------------+
       ^ Requires DB, Env Vars, Dependencies
```

### The Mockelot Way
You overlay *one path* on top of the real service.

```
[ Local Machine ]
+------------------------------------------+
|  My App                                  |
|     ↓ (HTTPS Request)                    |
|  [ Mockelot SOCKS5 Proxy ]               |
|     │                                    |
|     ├─ GET /sku/999 ────> [ 503 ERROR ]  | (Mock Intercept)
|     │                                    |
|     └─ All Other paths ─┐                |
+-------------------------│----------------+
                          │ (VPN Tunnel)
                          ▼
                 [ Remote Cluster ]
                 +-------------------+
                 | Inventory Service |
                 +-------------------+
```

**Advantage:** Zero cluster modification. You don't need the database or env vars locally.

---

## 2. The "3rd Party API" Trap (Mocking Salesforce/Stripe)

**The Challenge:** You need to mock an external SaaS provider (e.g., `api.stripe.com`) to test rate limits or errors.

### The Standard Way (Config Hacking)
You must change your application code/config to point to localhost.

```
[ Application Code ]
- const API_URL = "https://api.stripe.com"
+ const API_URL = "http://localhost:8080"  <-- Config Drift / Security Risk!
```

### The Mockelot Way (Domain Takeover)
Your code remains untouched. Mockelot intercepts the connection at the network layer.

```
[ Application Code ]
  GET https://api.stripe.com  (Standard Config)
       │
       ▼
[ Mockelot SOCKS5 ]
  Does domain match "api.stripe.com"?
  ├── YES: Generate Cert & Serve Mock ──> [ 402 Payment Required ]
  └── NO:  Tunnel to Internet ──────────> [ Real Stripe API ]
```

**Advantage:** No config drift. Your code executes exactly as it would in production (HTTPS, Host headers).

---

## 3. The "Unprivileged" Developer

**The Challenge:** A frontend developer needs to fix a UI bug but the backend is in a locked-down Kubernetes cluster. They have VPN access but **no `kubectl` permissions**.

### The K8s Way
Blocked. Cannot install agents. Cannot port-forward without permissions.

```
[ Developer Laptop ]      [ Secure Cluster ]
+------------------+      +----------------+
|  Frontend App    | -X-> |  Backend API   |
+------------------+      +----------------+
      (Blocked: No Cluster Access)
```

### The Mockelot Way
Client-side only.

```
[ Developer Laptop ]
+-----------------------+
|  Browser / App        |
|  (SOCKS5 Configured)  |
|          ↓            |
|  [ Mockelot ]         |
|    1. Mocks Login     | (Local Override)
|    2. Proxies Data    | (Over VPN to Secure Cluster)
+----------│------------+
           │
           ▼
   [ Secure Cluster ]
   (No Agents Required)
```

**Advantage:** Works in strictly regulated environments where devs are consumers, not admins.

---

## 4. The "Container Latency" Test (Chaos Engineering)

**The Challenge:** You want to see if your app handles a 5-second database timeout correctly. The DB is a heavy local Docker container.

### The Traditional Way
Requires complex Linux networking commands or a service mesh.

```bash
# Complex setup inside container
docker exec -it my-db tc qdisc add dev eth0 root netem delay 5000ms
```

### The Mockelot Way
Treats the container as a proxy endpoint.

```
[ Mockelot UI ]
Endpoint: "My Database"
Type:     Container (postgres:14)
Delay:    5000ms  <-- [X] Enabled

[ Flow ]
App -> Mockelot -> (Wait 5s) -> Docker Container (Port 32768)
```

**Advantage:** Point-and-click chaos engineering. Toggle it on/off instantly without restarting the container.
