package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
	"github.com/kingsgambit2700/platform/backend/internal/domain/classroom"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/realtime/broadcast"
	"github.com/kingsgambit2700/platform/backend/internal/realtime/ticket"
)

const (
	actorIDKey      = "realtime_actor_id"
	actorRoleKey    = "realtime_actor_role"
	sessionTokenKey = "realtime_session_token"
)

type sessionAuthenticator interface {
	Authenticate(context.Context, string) (model.Principal, error)
}

type roomTickets interface {
	Consume(context.Context, string, string) (string, error)
	Authorize(context.Context, string, string) error
}

type Controller struct {
	service    *classroom.Service
	auth       sessionAuthenticator
	pool       *pgxpool.Pool
	bus        broadcast.Bus
	tickets    roomTickets
	logger     *slog.Logger
	cookieName string
	origins    []string
	hub        *hub
}

func New(service *classroom.Service, auth *authservice.Service, pool *pgxpool.Pool, bus broadcast.Bus, tickets *ticket.Service, logger *slog.Logger, cookieName string, origins []string) *Controller {
	return &Controller{service: service, auth: auth, pool: pool, bus: bus, tickets: tickets, logger: logger, cookieName: cookieName, origins: origins, hub: newHub()}
}

func (h *Controller) Register(app *fiber.App) {
	app.Get("/health/live", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	app.Get("/health/ready", h.ready)
	path := "/ws/v1"
	app.Use(path, h.authorizeUpgrade)
	app.Get(path, websocket.New(h.handle, websocket.Config{Origins: h.origins, AllowEmptyOrigin: false, Subprotocols: []string{"kg2700.classroom.v2", "kg2700.classroom.v1"}}))
}

func (h *Controller) RunSubscriber(ctx context.Context) error {
	return h.bus.Subscribe(ctx, func(_ context.Context, notification broadcast.Notification) error {
		h.hub.broadcast(notification.SessionID, fiber.Map{
			"type":      "revisionGap",
			"sessionId": notification.SessionID,
			"revision":  notification.Revision,
		})
		return nil
	})
}

func (h *Controller) ready(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "unavailable", "checks": fiber.Map{"postgres": "failed"}})
	}
	if err := h.bus.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "unavailable", "checks": fiber.Map{"postgres": "ok", "redis": "failed"}})
	}
	return c.JSON(fiber.Map{"status": "ok", "checks": fiber.Map{"postgres": "ok", "redis": "ok"}})
}

func (h *Controller) authorizeUpgrade(c fiber.Ctx) error {
	if !websocket.IsWebSocketUpgrade(c) {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusUpgradeRequired, "websocket_required", "WebSocket upgrade required", "Connect using the classroom WebSocket subprotocol"))
	}
	if !containsProtocol(c.Get("Sec-WebSocket-Protocol"), "kg2700.classroom.v1") && !containsProtocol(c.Get("Sec-WebSocket-Protocol"), "kg2700.classroom.v2") {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusBadRequest, "subprotocol_required", "WebSocket subprotocol required", "Use kg2700.classroom.v2"))
	}
	principal, err := h.auth.Authenticate(c.Context(), c.Cookies(h.cookieName))
	if err != nil {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusUnauthorized, "unauthenticated", "Authentication required", "Sign in before joining a classroom"))
	}
	role, ok := actorRole(principal.User.Roles)
	if !ok {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusForbidden, "classroom_forbidden", "Classroom access denied", "You are not assigned to this class session"))
	}
	c.Locals(actorIDKey, principal.User.ID)
	c.Locals(actorRoleKey, string(role))
	// Fiber request strings may reference reusable buffers; the socket outlives
	// the upgrade request and needs its own copy for later authentication.
	c.Locals(sessionTokenKey, strings.Clone(c.Cookies(h.cookieName)))
	return c.Next()
}

func (h *Controller) handle(conn *websocket.Conn) {
	conn.SetReadLimit(64 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	actor := classroom.Actor{ID: localString(conn, actorIDKey), Role: classroom.ActorRole(localString(conn, actorRoleKey))}
	rawSession := localString(conn, sessionTokenKey)
	join, err := readCommand(conn)
	if err != nil || join.Type != "join" || join.Ticket == "" {
		response := fiber.Map{"type": "error", "code": "invalid_ticket", "detail": "The first frame must contain a valid join ticket"}
		if platformid.Valid(join.CommandID) {
			response["commandId"] = join.CommandID
		}
		_ = conn.WriteJSON(response)
		return
	}
	joinCtx, joinCancel := context.WithTimeout(context.Background(), 3*time.Second)
	sessionID, err := h.tickets.Consume(joinCtx, join.Ticket, actor.ID)
	if err == nil {
		actor, err = h.authorizeCommand(joinCtx, rawSession, actor.ID, sessionID)
	}
	joinCancel()
	if err != nil {
		_ = conn.WriteJSON(fiber.Map{"type": "error", "commandId": join.CommandID, "code": "invalid_ticket", "detail": "The join ticket is invalid, expired, or already used"})
		return
	}
	client := h.hub.add(sessionID, conn)
	defer h.hub.remove(sessionID, client)
	loadCtx, loadCancel := context.WithTimeout(context.Background(), 3*time.Second)
	state, err := h.service.Reconnect(loadCtx, sessionID, 0)
	loadCancel()
	if err != nil {
		_ = client.write(rejected(join.CommandID, err))
		return
	}
	if err := client.write(fiber.Map{"type": "snapshot", "protocolVersion": 2, "commandId": join.CommandID, "snapshot": state.Snapshot, "events": state.Events}); err != nil {
		return
	}
	for {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		message, err := readCommand(conn)
		if err != nil {
			response := fiber.Map{"type": "error", "code": "invalid_message", "detail": "The command does not match the negotiated classroom protocol"}
			if platformid.Valid(message.CommandID) {
				response["commandId"] = message.CommandID
			}
			if err := client.write(response); err != nil {
				return
			}
			return
		}
		commandCtx, commandCancel := context.WithTimeout(context.Background(), 5*time.Second)
		actor, err = h.authorizeCommand(commandCtx, rawSession, actor.ID, sessionID)
		if err != nil {
			commandCancel()
			_ = client.write(fiber.Map{"type": "rejected", "commandId": message.CommandID, "code": "unauthorized", "detail": "Classroom access has expired or changed. Sign in and rejoin the class."})
			return
		}
		response, accepted := h.execute(commandCtx, sessionID, actor, message)
		commandCancel()
		if err := client.write(response); err != nil {
			return
		}
		if accepted != nil {
			publishCtx, publishCancel := context.WithTimeout(context.Background(), time.Second)
			err := h.bus.Publish(publishCtx, broadcast.Notification{SessionID: sessionID, Revision: accepted.State.Revision})
			publishCancel()
			if err != nil {
				h.logger.Warn("classroom invalidation publish failed", "session_id", sessionID, "revision", accepted.State.Revision, "error", err)
			}
		}
	}
}

func (h *Controller) authorizeCommand(ctx context.Context, rawSession, userID, sessionID string) (classroom.Actor, error) {
	principal, err := h.auth.Authenticate(ctx, rawSession)
	if err != nil {
		return classroom.Actor{}, err
	}
	if principal.User.ID != userID {
		return classroom.Actor{}, ticket.ErrForbidden
	}
	role, ok := actorRole(principal.User.Roles)
	if !ok {
		return classroom.Actor{}, ticket.ErrForbidden
	}
	if err := h.tickets.Authorize(ctx, userID, sessionID); err != nil {
		return classroom.Actor{}, err
	}
	return classroom.Actor{ID: userID, Role: role}, nil
}

type command struct {
	Version          int                    `json:"-"`
	InitialFEN       string                 `json:"initialFen"`
	Annotations      []classroom.Annotation `json:"annotations"`
	Title            string                 `json:"title"`
	CheckpointID     string                 `json:"checkpointId"`
	Seconds          int                    `json:"seconds"`
	Label            string                 `json:"label"`
	Type             string                 `json:"type"`
	CommandID        string                 `json:"commandId"`
	ExpectedRevision uint64                 `json:"expectedRevision"`
	AfterRevision    uint64                 `json:"afterRevision"`
	Move             string                 `json:"move"`
	FEN              string                 `json:"fen"`
	Moves            []string               `json:"moves"`
	Mode             classroom.Mode         `json:"mode"`
	ActiveActorID    string                 `json:"activeActorId"`
	Ticket           string                 `json:"ticket"`
	Payload          map[string]any         `json:"payload"`
}

func (h *Controller) execute(ctx context.Context, sessionID string, actor classroom.Actor, message command) (any, *classroom.Accepted) {
	if message.Type == "ping" {
		return fiber.Map{"type": "accepted", "commandId": message.CommandID, "operation": "ping"}, nil
	}
	if message.Type == "resume" {
		recovery, err := h.service.Reconnect(ctx, sessionID, message.AfterRevision)
		if err != nil {
			return rejected(message.CommandID, err), nil
		}
		return fiber.Map{"type": "snapshot", "protocolVersion": 2, "commandId": message.CommandID, "snapshot": recovery.Snapshot, "events": recovery.Events}, nil
	}
	now := time.Now().UTC()
	var accepted classroom.Accepted
	var err error
	switch message.Type {
	case "move":
		accepted, err = h.service.SubmitMove(ctx, classroom.MoveCommand{EventID: message.CommandID, SessionID: sessionID, Actor: actor, Move: message.Move, ExpectedRevision: message.ExpectedRevision, OccurredAt: now})
	case "setMode":
		accepted, err = h.service.SetMode(ctx, classroom.ModeCommand{EventID: message.CommandID, SessionID: sessionID, Actor: actor, Mode: message.Mode, ActiveActorID: message.ActiveActorID, ExpectedRevision: message.ExpectedRevision, OccurredAt: now})
	case "loadPosition":
		accepted, err = h.service.LoadPosition(ctx, classroom.PositionCommand{EventID: message.CommandID, SessionID: sessionID, Actor: actor, FEN: message.FEN, InitialFEN: message.InitialFEN, Moves: message.Moves, ExpectedRevision: message.ExpectedRevision, OccurredAt: now})
	case "closeBoard":
		accepted, err = h.service.Close(ctx, classroom.CloseCommand{EventID: message.CommandID, SessionID: sessionID, Actor: actor, ExpectedRevision: message.ExpectedRevision, OccurredAt: now})
	case "annotation", "undo", "checkpoint", "restoreCheckpoint", "deleteCheckpoint", "countdown":
		if message.Version < 2 {
			return disabledCommand(message.CommandID), nil
		}
		accepted, err = h.service.UpdateLesson(ctx, classroom.LessonCommand{EventID: message.CommandID, SessionID: sessionID, Actor: actor, ExpectedRevision: message.ExpectedRevision, OccurredAt: now, Operation: message.Type, Annotations: message.Annotations, Title: message.Title, CheckpointID: message.CheckpointID, Seconds: message.Seconds, Label: message.Label})
	case "exerciseAssign", "privateResponse", "exerciseReveal", "presence", "providerState":
		return fiber.Map{"type": "rejected", "commandId": message.CommandID, "code": "not_implemented", "detail": "This classroom feature is disabled in the current release"}, nil
	default:
		err = classroom.ErrInvalidBoard
	}
	if err != nil {
		return rejected(message.CommandID, err), nil
	}
	return fiber.Map{"type": "accepted", "commandId": message.CommandID, "state": accepted.State, "event": accepted.Event}, &accepted
}

func readCommand(conn *websocket.Conn) (command, error) {
	_, payload, err := conn.ReadMessage()
	if err != nil {
		return command{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var message command
	if err := decoder.Decode(&message); err != nil {
		return message, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return message, errors.New("invalid command envelope")
	}
	if conn.Subprotocol() == "kg2700.classroom.v2" {
		message.Version = 2
		if !validV2Envelope(payload, message) {
			return message, errors.New("invalid v2 command envelope")
		}
	}
	if !platformid.Valid(message.CommandID) || !validCommand(message) {
		return message, errors.New("invalid command envelope")
	}
	return message, nil
}

func validCommand(message command) bool {
	switch message.Type {
	case "join":
		return message.Ticket != ""
	case "resume", "ping", "closeBoard":
		return true
	case "move":
		return message.Move != ""
	case "setMode":
		return message.Mode != classroom.ModeUnknown
	case "loadPosition":
		return message.FEN != "" && message.Moves != nil
	case "restoreCheckpoint", "deleteCheckpoint", "countdown":
		return message.Version == 2
	case "annotation", "undo", "checkpoint", "exerciseAssign", "privateResponse", "exerciseReveal", "presence", "providerState":
		return true
	default:
		return false
	}
}

func containsProtocol(header, expected string) bool {
	for _, value := range strings.Split(header, ",") {
		if strings.TrimSpace(value) == expected {
			return true
		}
	}
	return false
}

func rejected(commandID string, err error) fiber.Map {
	code, detail := "internal_error", "The command could not be completed"
	switch {
	case errors.Is(err, classroom.ErrRevisionConflict):
		code, detail = "revision_conflict", "Reload canonical state and retry"
	case errors.Is(err, classroom.ErrIllegalMove):
		code, detail = "illegal_move", "The move is not legal in the current position"
	case errors.Is(err, classroom.ErrMoveNotAuthorized), errors.Is(err, classroom.ErrControlNotAuthorized):
		code, detail = "unauthorized", "This actor cannot perform the command"
	case errors.Is(err, classroom.ErrBoardClosed):
		code, detail = "board_closed", "The board is closed"
	case errors.Is(err, classroom.ErrInvalidBoard):
		code, detail = "invalid_message", "The message is invalid"
	}
	if code == "internal_error" {
		return fiber.Map{"type": "error", "commandId": commandID, "code": code, "detail": detail}
	}
	response := fiber.Map{"type": "rejected", "commandId": commandID, "code": code, "detail": detail}
	var conflict *classroom.RevisionConflictError
	if errors.As(err, &conflict) {
		response["currentRevision"] = conflict.Current
	}
	return response
}

func actorRole(roles []model.Role) (classroom.ActorRole, bool) {
	priority := []model.Role{model.RoleSuperAdmin, model.RoleOperationsAdmin, model.RoleAcademicManager, model.RoleCoach, model.RoleStudent}
	for _, wanted := range priority {
		for _, role := range roles {
			if role == wanted {
				return classroom.ActorRole(role), true
			}
		}
	}
	return classroom.ActorRoleUnknown, false
}

func localString(conn *websocket.Conn, key string) string {
	value, _ := conn.Locals(key).(string)
	return value
}

type client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *client) write(value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if c.conn.Subprotocol() != "kg2700.classroom.v2" {
		value = legacyFrame(value)
	}
	return c.conn.WriteJSON(value)
}

type hub struct {
	mu       sync.RWMutex
	sessions map[string]map[*client]struct{}
}

func newHub() *hub { return &hub{sessions: make(map[string]map[*client]struct{})} }

func (h *hub) add(sessionID string, conn *websocket.Conn) *client {
	value := &client{conn: conn}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[sessionID] == nil {
		h.sessions[sessionID] = make(map[*client]struct{})
	}
	h.sessions[sessionID][value] = struct{}{}
	return value
}

func (h *hub) remove(sessionID string, value *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions[sessionID], value)
	if len(h.sessions[sessionID]) == 0 {
		delete(h.sessions, sessionID)
	}
}

func (h *hub) broadcast(sessionID string, value any) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.sessions[sessionID]))
	for client := range h.sessions[sessionID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		_ = client.write(value)
	}
}
