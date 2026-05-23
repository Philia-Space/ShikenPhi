# ShikenPhi (試験)

**Exam Session Service for JLPT Practice**

ShikenPhi is the exam engine microservice for the Philia-Space ecosystem. It manages exam session lifecycles, answer tracking, scoring, results, leaderboards, and user statistics for JLPT (Japanese Language Proficiency Test) practice exams.

## Overview

| Attribute | Value |
|-----------|-------|
| **Name** | ShikenPhi (試験 = "exam/test") |
| **Port** | 8088 |
| **Database** | MongoDB |
| **Depends On** | MondaiPhi (question bank) |
| **Stack** | Go + standard lib HTTP + `phi-core` DDD |

## Features

- **Session Lifecycle**: Create → Answer → Submit → Results
- **Exact-Fit Algorithm**: Deterministic question selection using subset-sum DP (guarantees exact question counts)
- **Atomic Units**: Questions from same passage/source group are kept together
- **Option Shuffling**: Per-session randomization (fixed after creation)
- **Real-Time Answers**: Save individual answers during exam
- **Scoring & Results**: Automatic scoring with section breakdowns
- **Leaderboards**: Weekly/monthly/all-time rankings per level
- **User Stats**: Aggregated performance, streaks, XP/rank system
- **24h Expiry**: Sessions auto-expire (enforced by TTL index)

## Architecture

```
ShikenPhi/
├── main.go                          # Entry point
├── config/                          # Environment configuration
├── handlers/                        # HTTP handlers
│   ├── session.go                   # Session routes (create, load, answer, submit)
│   └── result.go                    # Results, leaderboard, profile routes
├── internal/                        # Domain + Application layers
│   ├── domain/                      # Aggregates, entities, repositories
│   │   ├── session.go               # Session aggregate root
│   │   ├── result.go                # Result entity
│   │   ├── stats.go                 # UserStats entity
│   │   ├── streak.go                # UserStreak value object
│   │   ├── leaderboard.go           # LeaderboardEntry read model
│   │   └── repository.go            # Repository interfaces
│   └── application/                 # Command handlers
│       └── create_session.go
├── repositories/
│   ├── mongo/                       # MongoDB implementation
│   │   └── client.go                # DB connection
│   └── memory/                      # In-memory fake for tests
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/sessions` | Create exam session |
| `GET` | `/sessions/:id` | Load session with hydrated questions |
| `POST` | `/sessions/:id/answers` | Save single answer |
| `POST` | `/sessions/:id/submit` | Submit exam → score → results |
| `GET` | `/results` | User's result history |
| `GET` | `/results/:id` | Single result |
| `GET` | `/leaderboard` | Rankings (`?period=weekly&level=N3`) |
| `GET` | `/profile/stats` | Aggregated user stats |
| `GET` | `/profile/streaks` | Daily streak calendar |

## Session Algorithm

1. Fetch template from MondaiPhi (e.g., `balanced_75`)
2. For each section:
   - Query eligible questions from MondaiPhi
   - Build atomic units (passage groups, source_group_key groups)
   - Shuffle units (seeded for reproducibility)
   - **Subset-sum DP** to pick units summing to exactly `targetCount`
3. Flatten question IDs across sections
4. Generate option orders (Fisher-Yates per question)
5. Create session with 24h expiry
6. Enforce: **one active session per user**

## Data Model

### Session Status
- `active` — In progress
- `completed` — Submitted and scored
- `expired` — Past 24h deadline
- `abandoned` — Explicitly abandoned

### ID Prefixes
- `ssn_` — Session
- `rst_` — Result

## Configuration

```env
SHIKENPHI_PORT=8088
SHIKENPHI_ENVIRONMENT=development
SHIKENPHI_MONGO_URL=mongodb://localhost:27018/shikenphi
SHIKENPHI_MONGO_DB=shikenphi
SHIKENPHI_AUTH_JWKS_URL=http://localhost:8080/.well-known/jwks.json
SHIKENPHI_MONDAIPHI_URL=http://localhost:8087
SHIKENPHI_LEADERBOARD_REFRESH_INTERVAL=5m
```

## Getting Started

```bash
# Install dependencies
cd services/ShikenPhi
go mod download

# Start server
make run
# or
go run main.go
```

## Related Services

- **MondaiPhi** (問題) — Question bank service (provides questions)
- **AuthPhi** — Authentication & JWT provider
- **LyraPhi** — Frontend exam application

## License

ISC
