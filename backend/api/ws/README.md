# Realtime protocol

Version 1 uses `wss://api.kingsgambit2700.in/ws/v1` with the `kg2700.classroom.v1` subprotocol so the host-only session cookie is sent during the upgrade. The reverse proxy routes this path to the `realtime` container. Browser `Origin` must exactly match the configured web origin. The first frame must be `join` with a short-lived, single-use ticket created by `POST /api/v1/realtime/join-tickets`; the class-session ID is not put in the URL.

PostgreSQL is authoritative. A command is acknowledged only after its event and new revision commit atomically. Redis messages contain only `{sessionId, revision}` invalidations; a receiving node reloads state/events from PostgreSQL. Clients recover with `resume` after reconnecting and must discard speculative state on `revision_conflict`.
