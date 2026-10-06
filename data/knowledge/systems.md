# Localoy Systems Architecture

## Partner Backend
The Partner Backend (running on port 4000) manages the experience catalogue, merchants, listing verification, and card resolution.
- Key endpoint for Loco: `POST /api/v1/internal/loco/cards` hydrates experience item IDs (`cm...`) into rich banner mini cards (title, area, price, rating, banner image).
- Secured with `LOCO_SERVICE_TOKEN`.
- Holds the single source of truth for dining, events, offers, and stores.

## Auth Server
The Auth Server (running on port 8003) issues and validates consumer access tokens.
- Issues 60-minute HS256 JWT tokens.
- Secret: `CONSUMER_AUTH_JWT_SECRET` (Auth Server's `SECRET_KEY`).
- Token payload contains `sub` (User UUID), `type: access`, and role.
- Provides `GET /api/v1/admin/users/:sub` for user profile, name, and area lookup with `AUTH_ADMIN_SERVICE_TOKEN`.

## Loco AI Upstream Engine
Loco AI connects to an upstream n8n workflow (`https://n8n.caprover-internal.localoy.app/webhook/loco-ai`).
- Requests are cryptographically signed using HMAC-SHA256 into the `X-Localoy-Signature: t=...,v1=...` header using `LOCO_WEBHOOK_SIGNING_SECRET`.
- Time budget: 25 seconds.
- Returns conversational replies, experience recommendations, and multi-step curated itineraries.

## Cubicle Deployment Platform
Cubicle is Localoy's self-hosted container deployment platform.
- Builds Docker images with pinned Alpine Go runners.
- Manages reverse proxying, health checks via `GET /healthz`, environment variables, and auto-scaling.
- Injects managed Redis (`REDIS_URL`) and PostgreSQL (`DATABASE_URL`).

## Localoy Flutter App and Web
- **Flutter App**: Cross-platform mobile app for iOS and Android providing experiences, Loco chat, itineraries, and saved plans.
- **Consumer Web**: Next.js frontend at `localoy-front` serving the web platform and `/loco` chat.
