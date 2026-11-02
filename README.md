# Tally: live polls

Create a poll, share the link (or QR code), and watch the votes arrive live on every screen. You never need to refresh.

**Live demo:** _<add your deployed URL here>_

| Layer    | Tech                       | What it actually does |
|----------|----------------------------|-----------------------|
| Frontend | React 19 + TypeScript + Vite | SPA: auth, poll builder, voting, live results, present mode |
| Backend  | Go + Gin                   | REST API, validation, auth, vote orchestration, SSE streaming |
| Database | MongoDB                    | Durable source of truth: users, polls, every individual vote |
| Realtime | Redis                      | Live counters, atomic vote dedupe, pub/sub fan-out, presence, rate limiting |

```
/frontend   React app (src/pages, src/components, src/hooks, src/lib)
/backend    Go service
  cmd/server        entrypoint + graceful shutdown
  internal/api      HTTP layer: routes, middleware, handlers (thin)
  internal/service  business rules: how a vote flows through Redis + MongoDB
  internal/live     everything Redis: counter (Lua), hub (pub/sub), presence, rate limiter
  internal/store    everything MongoDB
  internal/validate all input normalisation + validation
  internal/auth     bcrypt + JWT
Dockerfile  builds both into one image
render.yaml one-click deploy on Render
```

## Features

- **Sign up / log in** (bcrypt + JWT in an HttpOnly cookie). An account is needed to create or manage polls. Voting never needs one.
- **Poll builder** with 2–10 options, an optional description and an optional auto-close timer (5 min to 1 week).
- **Live results**: animated bars, a vote counter and a "N viewers" presence indicator, all pushed from the server.
- **One vote per person**, enforced atomically on the server (see below). Your own choice is highlighted afterwards.
- **Share** with a copy-link button, the native share sheet on mobile, and a QR code.
- **Present mode** (`/p/:code/present`): a full-screen projector view with giant live bars, a big QR code and a total counter. It is built for a speaker asking a room.
- **Owner controls**: close or reopen voting (everyone's screen updates instantly) and delete the poll.
- **Dashboard** listing your polls with vote totals.
- Dark/light theme (follows the OS), mobile-friendly, keyboard-accessible.

## How a vote flows (the interesting part)

```
Browser ──POST /vote──▶ Go
                         │ 1. validate poll is open + option belongs to poll
                         │ 2. Redis Lua script (atomic):
                         │      SADD voters  → already there? reject "already voted"
                         │      HINCRBY counts[option] +1
                         │      INCR seq      → version number for this snapshot
                         │ 3. INSERT vote into MongoDB (unique index on pollId+voterKey)
                         │      failed? → run revert Lua script, return error
                         │ 4. PUBLISH {counts, seq} on lp:events:<pollId>
                         ▼
        every Go instance's Hub (PSUBSCRIBE lp:events:*)
                         │
                         ▼
        every open browser tab (Server-Sent Events) → bars animate
```

### Key decisions

**Redis does the live work. MongoDB is the source of truth.**
Each vote is stored in MongoDB as its own document, so the data is durable and can be audited. Counting thousands of documents on every page view, or racing on read-modify-write, would be slow and wrong. Redis holds the hot state: a hash of counts, a set of who already voted, and a version number. Reads are O(options), not O(votes).

**Atomic dedupe with a Lua script.**
"Has this person voted? If not, count it" must happen as one step, or two simultaneous clicks would both get counted. The Lua script runs atomically inside Redis. I tested this by firing 10 parallel requests from the same browser: exactly one returns `201` and the other nine return `409`. MongoDB's unique index on `(pollId, voterKey)` backs it up as a second line of defence.

**Redis and MongoDB can't silently drift.**
The vote goes to Redis first (the fast, contended path) and then to MongoDB. If the MongoDB write fails, a revert script undoes the Redis increment. Viewers only get the update once the vote is durable.

**Redis state rebuilds itself.**
Per-poll keys expire after 72h of inactivity, so Redis memory only holds polls that are active. When a request finds no live state (it expired, Redis restarted, or a flush happened), the counts and voter set are rebuilt from MongoDB. A short `SET NX` lock makes sure only one request rebuilds. While rebuilding, the vote script refuses ballots (`ready` flag missing), so no vote can slip between the MongoDB read and the Redis write. I tested this by running `FLUSHALL` mid-poll: counts came back correct, and earlier voters still could not vote twice.

**Why SSE and not WebSockets?**
Updates only flow server → browser, and votes are ordinary POSTs. Server-Sent Events are plain HTTP. They pass through every proxy and load balancer, reconnect automatically, and need no extra library. A heartbeat every 20 seconds keeps idle proxies from cutting the connection.

**Ordering and the subscribe/snapshot race.**
Every counts message carries a monotonically increasing `seq`, and the browser ignores anything older than what it shows. The stream handler subscribes *before* it reads the initial snapshot. A vote landing in between therefore shows up in both places, and the client drops the duplicate. Reading first and subscribing second would lose it.

**Horizontal scaling works.**
No state lives in Go memory except "which sockets are connected to me". A vote handled by instance A reaches viewers on instance B through Redis pub/sub. Presence (viewer count) is a Redis sorted set scored by heartbeat time, so connections lost in a crash age out instead of counting forever. If a browser can't keep up, the hub drops it instead of blocking everyone, and the browser reconnects and resyncs from a fresh snapshot.

**One deploy, one origin.**
Go serves the built React app and the API from the same domain. Session cookies stay first-party (`HttpOnly`, `Secure`, `SameSite=Lax`), so no CORS configuration is needed and there is no cross-site cookie trouble.

### Security and validation

- Every request body is decoded strictly (unknown fields are rejected, 32 KB limit), then normalised and validated in `internal/validate`. That covers lengths, control characters, email format, unique options, the duration range, the slug and option-id formats, and whether the option belongs to the poll. All of this happens before anything touches MongoDB.
- Passwords are hashed with bcrypt (cost 12). Login gives the same error and takes the same time for "unknown email" and "wrong password", which prevents account enumeration.
- JWTs are HS256 only (`alg=none` and algorithm swapping are rejected), with expiry required. They live in an HttpOnly cookie that JavaScript cannot read.
- `SameSite=Lax` cookies block cross-site POST/PATCH/DELETE (CSRF).
- Ownership is checked server-side for close, reopen and delete.
- Redis-backed per-IP rate limits apply to signup, login, poll creation, voting and streams. They are shared across instances. Limits are generous for voting so that a classroom behind one NAT IP still works.
- Security headers include CSP, `X-Frame-Options`, `nosniff` and `Referrer-Policy`. The login redirect (`?next=`) only accepts same-site paths.

### Known limitations (honest)

- Anonymous voters are identified by a random HttpOnly cookie. Someone determined can clear cookies or use incognito to vote again. Real one-person-one-vote needs logins or verified identity, which would hurt a "scan the QR and vote" flow. The per-IP rate limit caps the damage.
- Results are public to anyone with the link. Hiding results until a person votes is only a UX choice in the frontend.
- On Render's free tier, the service sleeps after 15 minutes idle. The first request then takes about 30–60 seconds.

## Running locally

Prerequisites: Go 1.26+, Node 20+, and MongoDB and Redis running locally (or free cloud instances, see below).

```bash
# 1. backend (http://localhost:8080)
cd backend
export MONGO_URI="mongodb://localhost:27017"
export REDIS_URL="redis://localhost:6379"
export JWT_SECRET="$(openssl rand -hex 32)"
go run ./cmd/server

# 2. frontend (http://localhost:5173, proxies /api to :8080)
cd frontend
npm install
npm run dev
```

Quick infra with Docker: `docker run -d -p 27017:27017 mongo:8` and `docker run -d -p 6379:6379 redis:7`.

### Tests

```bash
cd backend && go test ./...
```

The tests cover validation rules, JWT tampering, expiry and `alg=none`, concurrent duplicate votes (200 racing requests), rebuilding state from the store after expiry, seq ordering, revert, pub/sub fan-out across two simulated instances, slow-consumer eviction, presence and the rate limiter. Redis behaviour runs against an in-process Redis (miniredis).

## Deploying (free tier)

1. **MongoDB Atlas**: create a free M0 cluster and a database user. Under Network Access, allow `0.0.0.0/0`. Copy the `mongodb+srv://...` URI.
2. **Upstash Redis**: create a free database and copy the `rediss://default:<password>@<host>:6379` URL. TLS is used automatically. Pub/sub and Lua both work.
3. **Render**: push this repo to GitHub, then choose **New → Blueprint** and pick the repo. `render.yaml` builds the Dockerfile. Paste `MONGO_URI` and `REDIS_URL`. `JWT_SECRET` is generated for you.
4. Open the URL and check `/api/healthz`, which should return `{"status":"ok"}`.

Environment variables: `MONGO_URI`, `REDIS_URL`, `JWT_SECRET` (≥32 chars) are required. `APP_ENV=production` turns on secure cookies and JSON logs. `MONGO_DB`, `PORT`, `STATIC_DIR` and `TRUSTED_PROXIES` are optional.

## API

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/auth/signup` | – | create account, start session |
| POST | `/api/auth/login` | – | start session |
| POST | `/api/auth/logout` | – | end session |
| GET | `/api/auth/me` | optional | current user or `null` |
| POST | `/api/polls` | ✔ | create poll |
| GET | `/api/polls/mine` | ✔ | my polls + totals |
| GET | `/api/polls/:slug` | – | poll, results, my vote, isOwner |
| POST | `/api/polls/:slug/vote` | – | cast a vote |
| PATCH | `/api/polls/:slug` | owner | `{"status":"open"\|"closed"}` |
| DELETE | `/api/polls/:slug` | owner | delete poll and its votes |
| GET | `/api/polls/:slug/stream` | – | SSE: `snapshot`, `counts`, `viewers`, `status`, `deleted` |
| GET | `/api/healthz` | – | MongoDB + Redis health |
