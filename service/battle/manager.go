package battle

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gorilla/websocket"
)

const (
	messageTypeError    = "error"
	messageTypeInput    = "input"
	messageTypeJoined   = "joined"
	messageTypeSnapshot = "snapshot"

	eventTypeHit         = "hit"
	eventTypeKnockout    = "knockout"
	eventTypeQuotaPickup = "quota_pickup"
	eventTypeQuotaFailed = "quota_failed"

	playerRadius = 18
	bulletRadius = 6
)

var ErrBattleDisabled = errors.New("battle is disabled")

type Manager struct {
	mu    sync.Mutex
	rooms map[string]*Room
}

var DefaultManager = NewManager()

func NewManager() *Manager {
	return &Manager{
		rooms: make(map[string]*Room),
	}
}

func (m *Manager) Join(conn *websocket.Conn, userId int, username string, roomId string) {
	roomId = normalizeRoomId(roomId)
	for attempts := 0; attempts < 2; attempts++ {
		room := m.getOrCreateRoom(roomId)
		client := &Client{
			userId:   userId,
			username: username,
			conn:     conn,
			send:     make(chan []byte, 16),
			room:     room,
		}
		select {
		case room.register <- client:
			go client.writePump()
			client.readPump()
			return
		case <-room.done:
		}
	}

	data, _ := common.Marshal(map[string]any{
		"type":    messageTypeError,
		"message": "Connection failed",
	})
	_ = conn.WriteMessage(websocket.TextMessage, data)
	_ = conn.Close()
}

func (m *Manager) getOrCreateRoom(roomId string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	if room, ok := m.rooms[roomId]; ok {
		select {
		case <-room.done:
			delete(m.rooms, roomId)
		default:
			return room
		}
	}

	room := newRoom(roomId, m)
	m.rooms[roomId] = room
	go room.run()
	return room
}

func (m *Manager) removeRoom(roomId string, room *Room) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.rooms[roomId] == room {
		delete(m.rooms, roomId)
	}
}

func normalizeRoomId(roomId string) string {
	roomId = strings.TrimSpace(roomId)
	if roomId == "" {
		return "lobby"
	}
	if len(roomId) > 40 {
		roomId = roomId[:40]
	}
	var builder strings.Builder
	for _, ch := range roomId {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' {
			builder.WriteRune(ch)
		}
	}
	if builder.Len() == 0 {
		return "lobby"
	}
	return builder.String()
}

type Client struct {
	userId   int
	username string
	conn     *websocket.Conn
	send     chan []byte
	room     *Room
}

type ClientMessage struct {
	Type  string      `json:"type"`
	Seq   int64       `json:"seq,omitempty"`
	Input PlayerInput `json:"input"`
}

type PlayerInput struct {
	Up    bool    `json:"up"`
	Down  bool    `json:"down"`
	Left  bool    `json:"left"`
	Right bool    `json:"right"`
	Shoot bool    `json:"shoot"`
	AimX  float64 `json:"aim_x"`
	AimY  float64 `json:"aim_y"`
}

func (c *Client) readPump() {
	defer func() {
		select {
		case c.room.unregister <- c:
		case <-c.room.done:
		}
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(2048)
	_ = c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var message ClientMessage
		if err := common.Unmarshal(data, &message); err != nil {
			continue
		}
		if message.Type != messageTypeInput {
			continue
		}
		select {
		case c.room.inputs <- clientInput{
			userId: c.userId,
			seq:    message.Seq,
			input:  sanitizeInput(message.Input),
		}:
		default:
		}
	}
}

func (c *Client) writePump() {
	pingTicker := time.NewTicker(25 * time.Second)
	defer func() {
		pingTicker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case data, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-pingTicker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) sendJSON(payload any) {
	data, err := common.Marshal(payload)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

type Room struct {
	id         string
	manager    *Manager
	register   chan *Client
	unregister chan *Client
	inputs     chan clientInput
	clients    map[int]*Client
	players    map[int]*player
	bullets    map[string]*bullet
	drops      map[string]*drop
	events     []BattleEvent
	rng        *rand.Rand
	nextId     int64
	idleSince  time.Time
	done       chan struct{}
}

type clientInput struct {
	userId int
	seq    int64
	input  PlayerInput
}

type player struct {
	UserId    int
	Username  string
	X         float64
	Y         float64
	HP        int
	Alive     bool
	RespawnAt time.Time
	LastShot  time.Time
	LastAimX  float64
	LastAimY  float64
	Input     PlayerInput
	InputSeq  int64
	Score     int
	Deaths    int
	RoundLoss int
	RoundGain int
}

type bullet struct {
	Id        string
	OwnerId   int
	X         float64
	Y         float64
	VX        float64
	VY        float64
	Damage    int
	ExpiresAt time.Time
}

type drop struct {
	Id         string
	FromUserId int
	Quota      int
	X          float64
	Y          float64
	ExpiresAt  time.Time
}

type BattleEvent struct {
	Id           string `json:"id"`
	Type         string `json:"type"`
	UserId       int    `json:"user_id,omitempty"`
	TargetUserId int    `json:"target_user_id,omitempty"`
	Quota        int    `json:"quota,omitempty"`
	CreatedAt    int64  `json:"created_at"`
}

type Snapshot struct {
	Type        string           `json:"type"`
	RoomId      string           `json:"room_id"`
	Me          int              `json:"me"`
	AckSeq      int64            `json:"ack_seq"`
	ServerTime  int64            `json:"server_time"`
	MapWidth    int              `json:"map_width"`
	MapHeight   int              `json:"map_height"`
	PlayerSpeed int              `json:"player_speed"`
	Players    []PlayerSnapshot `json:"players"`
	Bullets    []BulletSnapshot `json:"bullets"`
	Drops      []DropSnapshot   `json:"drops"`
	Events     []BattleEvent    `json:"events"`
}

type PlayerSnapshot struct {
	UserId    int     `json:"user_id"`
	Username  string  `json:"username"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	HP        int     `json:"hp"`
	Alive     bool    `json:"alive"`
	Score     int     `json:"score"`
	Deaths    int     `json:"deaths"`
	RoundLoss int     `json:"round_loss"`
	RoundGain int     `json:"round_gain"`
}

type BulletSnapshot struct {
	Id      string  `json:"id"`
	OwnerId int     `json:"owner_id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
}

type DropSnapshot struct {
	Id         string  `json:"id"`
	FromUserId int     `json:"from_user_id"`
	Quota      int     `json:"quota"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
}

func newRoom(id string, manager *Manager) *Room {
	return &Room{
		id:         id,
		manager:    manager,
		register:   make(chan *Client, 8),
		unregister: make(chan *Client, 64),
		inputs:     make(chan clientInput, 128),
		clients:    make(map[int]*Client),
		players:    make(map[int]*player),
		bullets:    make(map[string]*bullet),
		drops:      make(map[string]*drop),
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		done:       make(chan struct{}),
	}
}

func (r *Room) run() {
	settings := normalizedSettings()
	ticker := time.NewTicker(time.Second / time.Duration(settings.TickRate))
	defer func() {
		ticker.Stop()
		r.manager.removeRoom(r.id, r)
		close(r.done)
	}()

	lastTick := time.Now()
	for {
		select {
		case client := <-r.register:
			r.handleRegister(client)
		case client := <-r.unregister:
			r.handleUnregister(client)
		case input := <-r.inputs:
			if p := r.players[input.userId]; p != nil {
				if input.seq > 0 {
					if input.seq < p.InputSeq {
						continue
					}
					p.InputSeq = input.seq
				}
				p.Input = input.input
			}
		case now := <-ticker.C:
			settings = normalizedSettings()
			if !settings.Enabled {
				r.closeAll(messageTypeError, "battle disabled")
				return
			}
			dt := now.Sub(lastTick).Seconds()
			lastTick = now
			r.step(dt, now, settings)
			if len(r.clients) == 0 {
				if r.idleSince.IsZero() {
					r.idleSince = now
				}
				if now.Sub(r.idleSince) > time.Duration(settings.IdleRoomTTLSeconds)*time.Second {
					return
				}
			} else {
				r.idleSince = time.Time{}
			}
		}
	}
}

func (r *Room) handleRegister(client *Client) {
	settings := normalizedSettings()
	if !settings.Enabled {
		client.sendJSON(map[string]any{"type": messageTypeError, "message": "battle disabled"})
		close(client.send)
		return
	}
	if _, ok := r.clients[client.userId]; !ok && len(r.clients) >= settings.MaxPlayersPerRoom {
		client.sendJSON(map[string]any{"type": messageTypeError, "message": "room full"})
		close(client.send)
		return
	}
	if old := r.clients[client.userId]; old != nil {
		close(old.send)
		_ = old.conn.Close()
	}

	r.clients[client.userId] = client
	p := r.players[client.userId]
	if p == nil {
		p = &player{
			UserId:   client.userId,
			Username: client.username,
			Alive:    true,
			HP:       100,
			LastAimX: 1,
			LastAimY: 0,
		}
		r.placePlayer(p, settings)
		r.players[client.userId] = p
	} else {
		p.Username = client.username
	}
	p.Input = PlayerInput{AimX: 1, AimY: 0}
	p.InputSeq = 0
	client.sendJSON(map[string]any{"type": messageTypeJoined, "room_id": r.id})
}

func (r *Room) handleUnregister(client *Client) {
	if current := r.clients[client.userId]; current != client {
		return
	}
	delete(r.clients, client.userId)
	delete(r.players, client.userId)
	close(client.send)
}

func (r *Room) closeAll(messageType string, message string) {
	for userId, client := range r.clients {
		client.sendJSON(map[string]any{"type": messageType, "message": message})
		close(client.send)
		_ = client.conn.Close()
		delete(r.clients, userId)
	}
	r.players = make(map[int]*player)
	r.bullets = make(map[string]*bullet)
	r.drops = make(map[string]*drop)
}

func (r *Room) step(dt float64, now time.Time, settings operation_setting.BattleSetting) {
	if dt <= 0 || dt > 0.2 {
		dt = 1.0 / float64(settings.TickRate)
	}

	for _, p := range r.players {
		r.updatePlayer(p, dt, now, settings)
	}
	r.updateBullets(now, dt, settings)
	r.updateDrops(now, settings)
	r.handlePickups(now, settings)
	r.broadcastSnapshot(now, settings)
}

func (r *Room) updatePlayer(p *player, dt float64, now time.Time, settings operation_setting.BattleSetting) {
	if !p.Alive {
		if !p.RespawnAt.IsZero() && now.After(p.RespawnAt) {
			p.Alive = true
			p.HP = 100
			r.placePlayer(p, settings)
		}
		return
	}

	dx := boolToFloat(p.Input.Right) - boolToFloat(p.Input.Left)
	dy := boolToFloat(p.Input.Down) - boolToFloat(p.Input.Up)
	length := math.Hypot(dx, dy)
	if length > 0 {
		dx /= length
		dy /= length
		p.X = clampFloat(p.X+dx*float64(settings.PlayerSpeed)*dt, playerRadius, float64(settings.MapWidth-playerRadius))
		p.Y = clampFloat(p.Y+dy*float64(settings.PlayerSpeed)*dt, playerRadius, float64(settings.MapHeight-playerRadius))
	}

	if isFiniteVector(p.Input.AimX, p.Input.AimY) {
		aimLength := math.Hypot(p.Input.AimX, p.Input.AimY)
		if aimLength > 0.01 {
			p.LastAimX = p.Input.AimX / aimLength
			p.LastAimY = p.Input.AimY / aimLength
		}
	}

	if p.Input.Shoot && now.Sub(p.LastShot) >= time.Duration(settings.FireCooldownMs)*time.Millisecond {
		p.LastShot = now
		r.spawnBullet(p, now, settings)
	}
}

func (r *Room) updateBullets(now time.Time, dt float64, settings operation_setting.BattleSetting) {
	for id, b := range r.bullets {
		b.X += b.VX * dt
		b.Y += b.VY * dt
		if now.After(b.ExpiresAt) || b.X < 0 || b.X > float64(settings.MapWidth) || b.Y < 0 || b.Y > float64(settings.MapHeight) {
			delete(r.bullets, id)
			continue
		}
		for _, target := range r.players {
			if target.UserId == b.OwnerId || !target.Alive {
				continue
			}
			if distance(b.X, b.Y, target.X, target.Y) > playerRadius+bulletRadius {
				continue
			}
			delete(r.bullets, id)
			r.handleHit(b, target, now, settings)
			break
		}
	}
}

func (r *Room) updateDrops(now time.Time, _ operation_setting.BattleSetting) {
	for id, d := range r.drops {
		if now.After(d.ExpiresAt) {
			delete(r.drops, id)
		}
	}
}

func (r *Room) handlePickups(now time.Time, settings operation_setting.BattleSetting) {
	for dropId, d := range r.drops {
		for _, p := range r.players {
			if !p.Alive || p.UserId == d.FromUserId {
				continue
			}
			if distance(p.X, p.Y, d.X, d.Y) > float64(settings.DropPickupRadius) {
				continue
			}
			quota, err := r.settleDrop(d, p, settings)
			if err != nil {
				if errors.Is(err, model.ErrBattleQuotaInsufficient) {
					delete(r.drops, dropId)
				} else {
					d.ExpiresAt = now.Add(time.Second)
				}
				r.addEvent(eventTypeQuotaFailed, p.UserId, d.FromUserId, 0)
				break
			}
			if quota > 0 {
				r.addEvent(eventTypeQuotaPickup, p.UserId, d.FromUserId, quota)
				if d.Quota <= 0 {
					delete(r.drops, dropId)
					break
				}
			}
		}
	}
}

func (r *Room) handleHit(b *bullet, target *player, now time.Time, settings operation_setting.BattleSetting) {
	attacker := r.players[b.OwnerId]
	target.HP -= b.Damage
	r.createDrop(target, now, settings)
	r.addEvent(eventTypeHit, b.OwnerId, target.UserId, 0)

	if target.HP <= 0 {
		target.HP = 0
		target.Alive = false
		target.Deaths++
		target.RespawnAt = now.Add(time.Duration(settings.RespawnSeconds) * time.Second)
		if attacker != nil {
			attacker.Score++
		}
		r.addEvent(eventTypeKnockout, b.OwnerId, target.UserId, 0)
	}
}

func (r *Room) createDrop(target *player, now time.Time, settings operation_setting.BattleSetting) {
	remaining := settings.MaxRoundLossQuota - target.RoundLoss
	if remaining <= 0 {
		return
	}
	balance, err := model.GetUserQuota(target.UserId, true)
	if err != nil {
		r.addEvent(eventTypeQuotaFailed, 0, target.UserId, 0)
		return
	}
	remaining = minPositive(remaining, balance)
	if remaining <= 0 {
		return
	}

	dailyStart := model.BattleDailyUsageStart()
	usage, err := model.GetBattleQuotaUsageSince(target.UserId, dailyStart)
	if err != nil {
		r.addEvent(eventTypeQuotaFailed, 0, target.UserId, 0)
		return
	}
	remaining = minPositive(remaining, settings.MaxDailyLossQuota-usage.Lost)
	if remaining <= 0 {
		return
	}

	quota := settings.MinDropQuota
	if settings.MaxDropQuota > settings.MinDropQuota {
		quota += r.rng.Intn(settings.MaxDropQuota - settings.MinDropQuota + 1)
	}
	if quota > remaining {
		quota = remaining
	}
	if quota <= 0 {
		return
	}

	id := newBattleObjectId("drop")
	_, err = model.DebitBattleQuota(model.BattleQuotaMutationParams{
		RoomId:  r.id,
		EventId: id + "-debit",
		UserId:  target.UserId,
		Quota:   quota,
		Reason:  "drop",
		UsageLimit: &model.BattleQuotaLimit{
			Since: dailyStart,
			Max:   settings.MaxDailyLossQuota,
		},
	})
	if err != nil {
		r.addEvent(eventTypeQuotaFailed, 0, target.UserId, 0)
		return
	}
	target.RoundLoss += quota
	r.drops[id] = &drop{
		Id:         id,
		FromUserId: target.UserId,
		Quota:      quota,
		X:          clampFloat(target.X+r.rng.Float64()*50-25, 20, float64(settings.MapWidth-20)),
		Y:          clampFloat(target.Y+r.rng.Float64()*50-25, 20, float64(settings.MapHeight-20)),
		ExpiresAt:  now.Add(time.Duration(settings.DropExpireSeconds) * time.Second),
	}
}

func (r *Room) settleDrop(d *drop, picker *player, settings operation_setting.BattleSetting) (int, error) {
	if d.Quota <= 0 {
		return 0, nil
	}

	dailyStart := model.BattleDailyUsageStart()
	toUsage, err := model.GetBattleQuotaUsageSince(picker.UserId, dailyStart)
	if err != nil {
		return 0, err
	}

	amount := d.Quota
	amount = minPositive(amount, settings.MaxRoundGainQuota-picker.RoundGain)
	amount = minPositive(amount, settings.MaxDailyGainQuota-toUsage.Won)
	if amount <= 0 {
		return 0, nil
	}

	_, err = model.CreditBattleQuota(model.BattleQuotaMutationParams{
		RoomId:  r.id,
		EventId: newBattleObjectId("pickup"),
		UserId:  picker.UserId,
		Quota:   amount,
		Reason:  "pickup",
		UsageLimit: &model.BattleQuotaLimit{
			Since: dailyStart,
			Max:   settings.MaxDailyGainQuota,
		},
	})
	if err != nil {
		if errors.Is(err, model.ErrBattleQuotaLimitExceeded) {
			return 0, nil
		}
		return 0, err
	}

	picker.RoundGain += amount
	d.Quota -= amount
	return amount, nil
}

func (r *Room) spawnBullet(p *player, now time.Time, settings operation_setting.BattleSetting) {
	id := newBattleObjectId("bullet")
	vx := p.LastAimX * float64(settings.BulletSpeed)
	vy := p.LastAimY * float64(settings.BulletSpeed)
	r.bullets[id] = &bullet{
		Id:        id,
		OwnerId:   p.UserId,
		X:         p.X + p.LastAimX*(playerRadius+bulletRadius),
		Y:         p.Y + p.LastAimY*(playerRadius+bulletRadius),
		VX:        vx,
		VY:        vy,
		Damage:    settings.BulletDamage,
		ExpiresAt: now.Add(1400 * time.Millisecond),
	}
}

func (r *Room) placePlayer(p *player, settings operation_setting.BattleSetting) {
	p.X = float64(playerRadius) + r.rng.Float64()*float64(settings.MapWidth-playerRadius*2)
	p.Y = float64(playerRadius) + r.rng.Float64()*float64(settings.MapHeight-playerRadius*2)
}

func (r *Room) broadcastSnapshot(now time.Time, settings operation_setting.BattleSetting) {
	base := Snapshot{
		Type:        messageTypeSnapshot,
		RoomId:      r.id,
		ServerTime:  now.UnixMilli(),
		MapWidth:    settings.MapWidth,
		MapHeight:   settings.MapHeight,
		PlayerSpeed: settings.PlayerSpeed,
		Players:     make([]PlayerSnapshot, 0, len(r.players)),
		Bullets:     make([]BulletSnapshot, 0, len(r.bullets)),
		Drops:       make([]DropSnapshot, 0, len(r.drops)),
		Events:      append(make([]BattleEvent, 0, len(r.events)), r.events...),
	}
	for _, p := range r.players {
		base.Players = append(base.Players, PlayerSnapshot{
			UserId:    p.UserId,
			Username:  p.Username,
			X:         p.X,
			Y:         p.Y,
			HP:        p.HP,
			Alive:     p.Alive,
			Score:     p.Score,
			Deaths:    p.Deaths,
			RoundLoss: p.RoundLoss,
			RoundGain: p.RoundGain,
		})
	}
	for _, b := range r.bullets {
		base.Bullets = append(base.Bullets, BulletSnapshot{
			Id:      b.Id,
			OwnerId: b.OwnerId,
			X:       b.X,
			Y:       b.Y,
		})
	}
	for _, d := range r.drops {
		base.Drops = append(base.Drops, DropSnapshot{
			Id:         d.Id,
			FromUserId: d.FromUserId,
			Quota:      d.Quota,
			X:          d.X,
			Y:          d.Y,
		})
	}

	for userId, client := range r.clients {
		snapshot := base
		snapshot.Me = userId
		if p := r.players[userId]; p != nil {
			snapshot.AckSeq = p.InputSeq
		}
		client.sendJSON(snapshot)
	}
}

func (r *Room) addEvent(eventType string, userId int, targetUserId int, quota int) {
	r.nextId++
	event := BattleEvent{
		Id:           fmt.Sprintf("%s-event-%d", r.id, r.nextId),
		Type:         eventType,
		UserId:       userId,
		TargetUserId: targetUserId,
		Quota:        quota,
		CreatedAt:    time.Now().UnixMilli(),
	}
	r.events = append(r.events, event)
	if len(r.events) > 16 {
		r.events = r.events[len(r.events)-16:]
	}
}

func normalizedSettings() operation_setting.BattleSetting {
	s := *operation_setting.GetBattleSetting()
	s.MinDropQuota = clampInt(s.MinDropQuota, 0, 100000000)
	s.MaxDropQuota = clampInt(s.MaxDropQuota, s.MinDropQuota, 100000000)
	s.MaxRoundLossQuota = clampInt(s.MaxRoundLossQuota, 0, 100000000)
	s.MaxRoundGainQuota = clampInt(s.MaxRoundGainQuota, 0, 100000000)
	s.MaxDailyLossQuota = clampInt(s.MaxDailyLossQuota, 0, 100000000)
	s.MaxDailyGainQuota = clampInt(s.MaxDailyGainQuota, 0, 100000000)
	s.MaxPlayersPerRoom = clampInt(s.MaxPlayersPerRoom, 2, 32)
	s.TickRate = clampInt(s.TickRate, 10, 60)
	s.MapWidth = clampInt(s.MapWidth, 600, 4000)
	s.MapHeight = clampInt(s.MapHeight, 400, 2400)
	s.PlayerSpeed = clampInt(s.PlayerSpeed, 80, 900)
	s.BulletSpeed = clampInt(s.BulletSpeed, 100, 1800)
	s.BulletDamage = clampInt(s.BulletDamage, 1, 100)
	s.FireCooldownMs = clampInt(s.FireCooldownMs, 80, 2000)
	s.RespawnSeconds = clampInt(s.RespawnSeconds, 1, 30)
	s.DropPickupRadius = clampInt(s.DropPickupRadius, 8, 160)
	s.DropExpireSeconds = clampInt(s.DropExpireSeconds, 3, 120)
	s.IdleRoomTTLSeconds = clampInt(s.IdleRoomTTLSeconds, 5, 600)
	return s
}

func sanitizeInput(input PlayerInput) PlayerInput {
	if !isFiniteVector(input.AimX, input.AimY) {
		input.AimX = 1
		input.AimY = 0
	}
	input.AimX = clampFloat(input.AimX, -1, 1)
	input.AimY = clampFloat(input.AimY, -1, 1)
	return input
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func distance(x1 float64, y1 float64, x2 float64, y2 float64) float64 {
	return math.Hypot(x1-x2, y1-y2)
}

func isFiniteVector(x float64, y float64) bool {
	return !math.IsNaN(x) && !math.IsNaN(y) && !math.IsInf(x, 0) && !math.IsInf(y, 0)
}

func clampFloat(value float64, minValue float64, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minPositive(value int, limit int) int {
	if limit <= 0 {
		return 0
	}
	if value > limit {
		return limit
	}
	return value
}

func newBattleObjectId(prefix string) string {
	return prefix + "-" + common.GetUUID()
}
