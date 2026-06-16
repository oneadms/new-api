package battle

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
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

	eventTypeHit              = "hit"
	eventTypeCapSettlement    = "cap_settlement"
	eventTypeSettlementFailed = "settlement_failed"

	playerWidth  = 46.0
	playerHeight = 70.0
	capWidth     = 42.0
	capHeight    = 28.0

	gravity              = 1800.0
	jumpVelocity         = -760.0
	maxFallSpeed         = 980.0
	fastFallAcceleration = 900.0
	capThrowUpVelocity   = -430.0
	capGravity           = 1450.0
	capStackSpacing      = 12.0
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
	Jump  bool    `json:"jump,omitempty"`
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
	id          string
	manager     *Manager
	register    chan *Client
	unregister  chan *Client
	inputs      chan clientInput
	clients     map[int]*Client
	players     map[int]*player
	bullets     map[string]*bullet
	roundLosses map[int]int
	roundGains  map[int]int
	events      []BattleEvent
	rng         *rand.Rand
	nextId      int64
	idleSince   time.Time
	done        chan struct{}
}

type clientInput struct {
	userId int
	seq    int64
	input  PlayerInput
}

type player struct {
	UserId     int
	Username   string
	X          float64
	Y          float64
	VX         float64
	VY         float64
	Alive      bool
	LastShot   time.Time
	LastJump   bool
	LastShoot  bool
	LastAimX   float64
	LastAimY   float64
	Direction  int
	OnGround   bool
	Input      PlayerInput
	InputSeq   int64
	RoundLoss  int
	RoundGain  int
	CapStack   int
	CapSources map[int]int
}

type bullet struct {
	Id        string
	OwnerId   int
	X         float64
	Y         float64
	VX        float64
	VY        float64
	ExpiresAt time.Time
}

type platform struct {
	Id     string
	X      float64
	Y      float64
	W      float64
	H      float64
	OneWay bool
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
	Type        string             `json:"type"`
	RoomId      string             `json:"room_id"`
	Me          int                `json:"me"`
	AckSeq      int64              `json:"ack_seq"`
	ServerTime  int64              `json:"server_time"`
	MapWidth    int                `json:"map_width"`
	MapHeight   int                `json:"map_height"`
	PlayerSpeed int                `json:"player_speed"`
	Players     []PlayerSnapshot   `json:"players"`
	Bullets     []BulletSnapshot   `json:"bullets"`
	Platforms   []PlatformSnapshot `json:"platforms"`
	Events      []BattleEvent      `json:"events"`
}

type PlayerSnapshot struct {
	UserId    int     `json:"user_id"`
	Username  string  `json:"username"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	VX        float64 `json:"vx"`
	VY        float64 `json:"vy"`
	Alive     bool    `json:"alive"`
	Direction int     `json:"direction"`
	OnGround  bool    `json:"on_ground"`
	RoundLoss int     `json:"round_loss"`
	RoundGain int     `json:"round_gain"`
	CapStack  int     `json:"cap_stack"`
}

type BulletSnapshot struct {
	Id      string  `json:"id"`
	OwnerId int     `json:"owner_id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	VX      float64 `json:"vx"`
	VY      float64 `json:"vy"`
}

type PlatformSnapshot struct {
	Id     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
	OneWay bool    `json:"one_way"`
}

type capSettlement struct {
	UserId    int
	CapCount  int
	Amount    int
	Remainder int
}

func newRoom(id string, manager *Manager) *Room {
	return &Room{
		id:          id,
		manager:     manager,
		register:    make(chan *Client, 8),
		unregister:  make(chan *Client, 64),
		inputs:      make(chan clientInput, 128),
		clients:     make(map[int]*Client),
		players:     make(map[int]*player),
		bullets:     make(map[string]*bullet),
		roundLosses: make(map[int]int),
		roundGains:  make(map[int]int),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		done:        make(chan struct{}),
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
			UserId:     client.userId,
			Username:   client.username,
			Alive:      true,
			LastAimX:   1,
			LastAimY:   0,
			Direction:  1,
			RoundLoss:  r.roundLosses[client.userId],
			RoundGain:  r.roundGains[client.userId],
			CapSources: make(map[int]int),
		}
		r.placePlayer(p, settings)
		r.players[client.userId] = p
	} else {
		p.Username = client.username
		if p.CapSources == nil {
			p.CapSources = make(map[int]int)
		}
	}
	p.Input = PlayerInput{AimX: 1, AimY: 0}
	p.InputSeq = 0
	client.sendJSON(map[string]any{"type": messageTypeJoined, "room_id": r.id})
}

func (r *Room) handleUnregister(client *Client) {
	if current := r.clients[client.userId]; current != client {
		return
	}
	if p := r.players[client.userId]; p != nil {
		r.settlePlayerCaps(p, normalizedSettings())
	}
	delete(r.clients, client.userId)
	delete(r.players, client.userId)
	close(client.send)
}

func (r *Room) closeAll(messageType string, message string) {
	settings := normalizedSettings()
	for _, p := range r.players {
		r.settlePlayerCaps(p, settings)
	}
	for userId, client := range r.clients {
		client.sendJSON(map[string]any{"type": messageType, "message": message})
		close(client.send)
		_ = client.conn.Close()
		delete(r.clients, userId)
	}
	r.players = make(map[int]*player)
	r.bullets = make(map[string]*bullet)
}

func (r *Room) step(dt float64, now time.Time, settings operation_setting.BattleSetting) {
	if dt <= 0 || dt > 0.2 {
		dt = 1.0 / float64(settings.TickRate)
	}

	for _, p := range r.players {
		r.updatePlayer(p, dt, now, settings)
	}
	r.updateCaps(now, dt, settings)
	r.broadcastSnapshot(now, settings)
}

func (r *Room) updatePlayer(p *player, dt float64, now time.Time, settings operation_setting.BattleSetting) {
	moveX := boolToFloat(p.Input.Right) - boolToFloat(p.Input.Left)
	if moveX < 0 {
		p.Direction = -1
	} else if moveX > 0 {
		p.Direction = 1
	}
	p.VX = moveX * float64(settings.PlayerSpeed)
	p.X += p.VX * dt
	r.resolvePlayerHorizontal(p, settings)

	if isFiniteVector(p.Input.AimX, p.Input.AimY) {
		aimLength := math.Hypot(p.Input.AimX, p.Input.AimY)
		if aimLength > 0.01 {
			p.LastAimX = p.Input.AimX / aimLength
			p.LastAimY = p.Input.AimY / aimLength
		}
	}

	jumpPressed := p.Input.Jump || p.Input.Up
	if jumpPressed && !p.LastJump && p.OnGround {
		p.VY = jumpVelocity
		p.OnGround = false
	}
	p.LastJump = jumpPressed

	if p.Input.Down && p.OnGround {
		p.Y += 2
		p.VY = math.Max(p.VY, float64(settings.PlayerSpeed))
		p.OnGround = false
	}

	p.VY += gravity * dt
	if p.Input.Down {
		p.VY += fastFallAcceleration * dt
	}
	p.VY = math.Min(p.VY, maxFallSpeed)
	oldY := p.Y
	p.Y += p.VY * dt
	r.resolvePlayerVertical(p, oldY, settings)

	if p.Input.Shoot && !p.LastShoot && now.Sub(p.LastShot) >= time.Duration(settings.FireCooldownMs)*time.Millisecond {
		p.LastShot = now
		r.spawnBullet(p, now, settings)
	}
	p.LastShoot = p.Input.Shoot
}

func (r *Room) resolvePlayerHorizontal(p *player, settings operation_setting.BattleSetting) {
	p.X = clampFloat(p.X, playerWidth/2, float64(settings.MapWidth)-playerWidth/2)
	if p.VX == 0 {
		return
	}
	for _, platform := range battlePlatforms(settings) {
		if platform.OneWay || !rectsOverlap(playerLeft(p), playerTop(p), playerWidth, playerHeight, platform.X, platform.Y, platform.W, platform.H) {
			continue
		}
		if p.VX > 0 {
			p.X = platform.X - playerWidth/2
		} else {
			p.X = platform.X + platform.W + playerWidth/2
		}
		p.VX = 0
	}
	p.X = clampFloat(p.X, playerWidth/2, float64(settings.MapWidth)-playerWidth/2)
}

func (r *Room) resolvePlayerVertical(p *player, oldY float64, settings operation_setting.BattleSetting) {
	oldTop := oldY - playerHeight/2
	oldBottom := oldY + playerHeight/2
	newTop := playerTop(p)
	newBottom := playerBottom(p)
	p.OnGround = false

	for _, platform := range battlePlatforms(settings) {
		if playerRight(p) <= platform.X || playerLeft(p) >= platform.X+platform.W {
			continue
		}

		if p.VY >= 0 {
			if p.Input.Down && platform.OneWay {
				continue
			}
			if oldBottom <= platform.Y && newBottom >= platform.Y {
				p.Y = platform.Y - playerHeight/2
				p.VY = 0
				p.OnGround = true
				break
			}
			continue
		}

		if platform.OneWay {
			continue
		}
		if oldTop >= platform.Y+platform.H && newTop <= platform.Y+platform.H {
			p.Y = platform.Y + platform.H + playerHeight/2
			p.VY = 0
			break
		}
	}

	if playerTop(p) < 0 {
		p.Y = playerHeight / 2
		p.VY = 0
	}
	if playerBottom(p) > float64(settings.MapHeight) {
		p.Y = float64(settings.MapHeight) - playerHeight/2
		p.VY = 0
		p.OnGround = true
	}
}

func (r *Room) updateCaps(now time.Time, dt float64, settings operation_setting.BattleSetting) {
	for id, b := range r.bullets {
		previousY := b.Y
		b.VY = math.Min(b.VY+capGravity*dt, maxFallSpeed)
		b.X += b.VX * dt
		b.Y += b.VY * dt
		if now.After(b.ExpiresAt) || b.X < 0 || b.X > float64(settings.MapWidth) || b.Y < 0 || b.Y > float64(settings.MapHeight) {
			delete(r.bullets, id)
			continue
		}
		if capHitPlatform(b, previousY, settings) {
			delete(r.bullets, id)
			continue
		}
		for _, target := range r.players {
			if target.UserId == b.OwnerId || !target.Alive {
				continue
			}
			if !capHitsHead(b, target) {
				continue
			}
			delete(r.bullets, id)
			r.handleHit(b, target)
			break
		}
	}
}

func (r *Room) handleHit(b *bullet, target *player) {
	if target.CapSources == nil {
		target.CapSources = make(map[int]int)
	}
	target.CapStack++
	target.CapSources[b.OwnerId]++
	r.addEvent(eventTypeHit, b.OwnerId, target.UserId, 0)
}

func (r *Room) spawnBullet(p *player, now time.Time, settings operation_setting.BattleSetting) {
	id := newBattleObjectId("bullet")
	direction := p.Direction
	if direction == 0 {
		direction = 1
	}
	vx := float64(direction * settings.BulletSpeed)
	r.bullets[id] = &bullet{
		Id:        id,
		OwnerId:   p.UserId,
		X:         p.X + float64(direction)*(playerWidth/2+capWidth/2),
		Y:         p.Y - playerHeight*0.34,
		VX:        vx,
		VY:        capThrowUpVelocity,
		ExpiresAt: now.Add(2300 * time.Millisecond),
	}
}

func (r *Room) placePlayer(p *player, settings operation_setting.BattleSetting) {
	surfaces := spawnSurfaces(settings)
	surface := surfaces[r.rng.Intn(len(surfaces))]
	p.X = clampFloat(surface.X+playerWidth/2+r.rng.Float64()*math.Max(1, surface.W-playerWidth), playerWidth/2, float64(settings.MapWidth)-playerWidth/2)
	p.Y = surface.Y - playerHeight/2
	p.VX = 0
	p.VY = 0
	p.OnGround = true
	if p.Direction == 0 {
		p.Direction = 1
	}
}

func (r *Room) settlePlayerCaps(target *player, settings operation_setting.BattleSetting) {
	if target == nil || target.CapStack <= 0 || settings.CapQuota <= 0 {
		clearPlayerCaps(target)
		return
	}

	totalCaps := 0
	for userId, count := range target.CapSources {
		if userId > 0 && userId != target.UserId && count > 0 {
			totalCaps += count
		}
	}
	if totalCaps <= 0 {
		clearPlayerCaps(target)
		return
	}

	settleAmount := r.maxCapSettlementAmount(target, totalCaps, settings)
	if settleAmount <= 0 {
		r.addEvent(eventTypeSettlementFailed, 0, target.UserId, 0)
		clearPlayerCaps(target)
		return
	}

	settlements := r.allocateCapSettlements(target, totalCaps, settleAmount, settings)
	settled := 0
	dailyStart := model.BattleDailyUsageStart()
	for _, settlement := range settlements {
		if settlement.Amount <= 0 {
			continue
		}
		_, err := model.TransferBattleQuota(model.BattleQuotaTransferParams{
			RoomId:     r.id,
			EventId:    newBattleObjectId("cap-settle"),
			FromUserId: target.UserId,
			ToUserId:   settlement.UserId,
			Quota:      settlement.Amount,
			Reason:     "cap_settle",
			FromUsageLimit: &model.BattleQuotaLimit{
				Since: dailyStart,
				Max:   settings.MaxDailyLossQuota,
			},
			ToUsageLimit: &model.BattleQuotaLimit{
				Since: dailyStart,
				Max:   settings.MaxDailyGainQuota,
			},
		})
		if err != nil {
			r.addEvent(eventTypeSettlementFailed, settlement.UserId, target.UserId, 0)
			continue
		}
		settled += settlement.Amount
		r.recordSettledTransfer(target.UserId, settlement.UserId, settlement.Amount)
		r.addEvent(eventTypeCapSettlement, settlement.UserId, target.UserId, settlement.Amount)
	}
	if settled <= 0 {
		r.addEvent(eventTypeSettlementFailed, 0, target.UserId, 0)
	}
	clearPlayerCaps(target)
}

func (r *Room) maxCapSettlementAmount(target *player, totalCaps int, settings operation_setting.BattleSetting) int {
	amount := totalCaps * settings.CapQuota
	balance, err := model.GetUserQuota(target.UserId, true)
	if err != nil {
		return 0
	}
	amount = minPositive(amount, balance)
	amount = minPositive(amount, settings.MaxRoundLossQuota-r.roundLosses[target.UserId])

	dailyStart := model.BattleDailyUsageStart()
	usage, err := model.GetBattleQuotaUsageSince(target.UserId, dailyStart)
	if err != nil {
		return 0
	}
	amount = minPositive(amount, settings.MaxDailyLossQuota-usage.Lost)
	return amount
}

func (r *Room) allocateCapSettlements(target *player, totalCaps int, totalAmount int, settings operation_setting.BattleSetting) []capSettlement {
	settlements := make([]capSettlement, 0, len(target.CapSources))
	for userId, count := range target.CapSources {
		if userId <= 0 || userId == target.UserId || count <= 0 {
			continue
		}
		raw := totalAmount * count
		settlements = append(settlements, capSettlement{
			UserId:    userId,
			CapCount:  count,
			Amount:    raw / totalCaps,
			Remainder: raw % totalCaps,
		})
	}
	sort.Slice(settlements, func(i, j int) bool {
		if settlements[i].Remainder == settlements[j].Remainder {
			return settlements[i].UserId < settlements[j].UserId
		}
		return settlements[i].Remainder > settlements[j].Remainder
	})

	assigned := 0
	for index := range settlements {
		assigned += settlements[index].Amount
	}
	for index := 0; assigned < totalAmount && index < len(settlements); index++ {
		settlements[index].Amount++
		assigned++
	}

	sort.Slice(settlements, func(i, j int) bool {
		return settlements[i].UserId < settlements[j].UserId
	})
	for index := range settlements {
		remainingGain := r.remainingCapGain(settlements[index].UserId, settings)
		settlements[index].Amount = minPositive(settlements[index].Amount, remainingGain)
	}
	return settlements
}

func (r *Room) remainingCapGain(userId int, settings operation_setting.BattleSetting) int {
	remaining := settings.MaxRoundGainQuota - r.roundGains[userId]
	dailyStart := model.BattleDailyUsageStart()
	usage, err := model.GetBattleQuotaUsageSince(userId, dailyStart)
	if err != nil {
		return 0
	}
	return minPositive(remaining, settings.MaxDailyGainQuota-usage.Won)
}

func (r *Room) recordSettledTransfer(fromUserId int, toUserId int, amount int) {
	if amount <= 0 {
		return
	}
	r.roundLosses[fromUserId] += amount
	if p := r.players[fromUserId]; p != nil {
		p.RoundLoss = r.roundLosses[fromUserId]
	}
	r.roundGains[toUserId] += amount
	if p := r.players[toUserId]; p != nil {
		p.RoundGain = r.roundGains[toUserId]
	}
}

func clearPlayerCaps(p *player) {
	if p == nil {
		return
	}
	p.CapStack = 0
	p.CapSources = make(map[int]int)
}

func battlePlatforms(settings operation_setting.BattleSetting) []platform {
	scaleX := float64(settings.MapWidth) / 1600
	scaleY := float64(settings.MapHeight) / 900
	floorH := math.Max(26, 40*scaleY)
	thinH := math.Max(18, 30*scaleY)
	wallW := math.Max(28, 34*scaleX)
	wallH := math.Max(80, 140*scaleY)

	platforms := []platform{
		{Id: "floor", X: 0, Y: float64(settings.MapHeight) - floorH, W: float64(settings.MapWidth), H: floorH},
		scaledPlatform("left-low", 80, 735, 360, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("center-low", 540, 695, 330, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("right-low", 1035, 720, 430, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("left-mid", 250, 575, 300, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("center-mid", 735, 545, 310, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("right-mid", 1170, 520, 310, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("left-high", 60, 410, 300, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("center-high", 500, 375, 365, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("right-high", 1015, 345, 345, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("left-top", 220, 255, 340, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("center-top", 760, 235, 300, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("right-top", 1230, 215, 270, thinH, true, scaleX, scaleY, settings),
		scaledPlatform("left-wall", 0, 0, wallW, 900, false, scaleX, scaleY, settings),
		scaledPlatform("right-wall", 1600-wallW/scaleX, 0, wallW, 900, false, scaleX, scaleY, settings),
		scaledPlatform("low-pillar", 665, 735, wallW, wallH, false, scaleX, scaleY, settings),
		scaledPlatform("right-pillar", 1445, 575, wallW, 285, false, scaleX, scaleY, settings),
		scaledPlatform("upper-pillar", 382, 255, wallW, 165, false, scaleX, scaleY, settings),
	}
	return platforms
}

func scaledPlatform(id string, x float64, y float64, w float64, h float64, oneWay bool, scaleX float64, scaleY float64, settings operation_setting.BattleSetting) platform {
	nextX := clampFloat(x*scaleX, 0, float64(settings.MapWidth)-1)
	nextY := clampFloat(y*scaleY, 0, float64(settings.MapHeight)-1)
	nextW := math.Min(w*scaleX, float64(settings.MapWidth)-nextX)
	nextH := math.Min(h*scaleY, float64(settings.MapHeight)-nextY)
	if nextW < 1 {
		nextW = 1
	}
	if nextH < 1 {
		nextH = 1
	}
	return platform{Id: id, X: nextX, Y: nextY, W: nextW, H: nextH, OneWay: oneWay}
}

func spawnSurfaces(settings operation_setting.BattleSetting) []platform {
	platforms := battlePlatforms(settings)
	surfaces := make([]platform, 0, len(platforms))
	for _, platform := range platforms {
		if platform.W >= playerWidth && platform.Y >= playerHeight {
			surfaces = append(surfaces, platform)
		}
	}
	if len(surfaces) == 0 {
		return []platform{{Id: "fallback", X: 0, Y: float64(settings.MapHeight) - 1, W: float64(settings.MapWidth), H: 1}}
	}
	return surfaces
}

func platformSnapshots(settings operation_setting.BattleSetting) []PlatformSnapshot {
	platforms := battlePlatforms(settings)
	snapshots := make([]PlatformSnapshot, 0, len(platforms))
	for _, platform := range platforms {
		snapshots = append(snapshots, PlatformSnapshot{
			Id:     platform.Id,
			X:      platform.X,
			Y:      platform.Y,
			W:      platform.W,
			H:      platform.H,
			OneWay: platform.OneWay,
		})
	}
	return snapshots
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
		Platforms:   platformSnapshots(settings),
		Events:      append(make([]BattleEvent, 0, len(r.events)), r.events...),
	}
	for _, p := range r.players {
		base.Players = append(base.Players, PlayerSnapshot{
			UserId:    p.UserId,
			Username:  p.Username,
			X:         p.X,
			Y:         p.Y,
			VX:        p.VX,
			VY:        p.VY,
			Alive:     p.Alive,
			Direction: p.Direction,
			OnGround:  p.OnGround,
			RoundLoss: r.roundLosses[p.UserId] + p.CapStack*settings.CapQuota,
			RoundGain: r.roundGains[p.UserId] + r.pendingCapGain(p.UserId, settings),
			CapStack:  p.CapStack,
		})
	}
	for _, b := range r.bullets {
		base.Bullets = append(base.Bullets, BulletSnapshot{
			Id:      b.Id,
			OwnerId: b.OwnerId,
			X:       b.X,
			Y:       b.Y,
			VX:      b.VX,
			VY:      b.VY,
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

func (r *Room) pendingCapGain(userId int, settings operation_setting.BattleSetting) int {
	if settings.CapQuota <= 0 {
		return 0
	}
	totalCaps := 0
	for _, p := range r.players {
		if p == nil || p.UserId == userId {
			continue
		}
		totalCaps += p.CapSources[userId]
	}
	return totalCaps * settings.CapQuota
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
	s.CapQuota = clampInt(s.CapQuota, 0, 100000000)
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

func capHitPlatform(b *bullet, previousY float64, settings operation_setting.BattleSetting) bool {
	capLeft := b.X - capWidth/2
	capTop := b.Y - capHeight/2
	for _, platform := range battlePlatforms(settings) {
		if !rectsOverlap(capLeft, capTop, capWidth, capHeight, platform.X, platform.Y, platform.W, platform.H) {
			continue
		}
		if platform.OneWay && previousY+capHeight/2 > platform.Y {
			continue
		}
		return true
	}
	return false
}

func capHitsHead(b *bullet, target *player) bool {
	headTop := playerTop(target) - float64(target.CapStack)*capStackSpacing - capHeight*0.35
	headHeight := playerHeight * 0.34
	return rectsOverlap(
		b.X-capWidth/2,
		b.Y-capHeight/2,
		capWidth,
		capHeight,
		playerLeft(target)+playerWidth*0.08,
		headTop,
		playerWidth*0.84,
		headHeight,
	)
}

func playerLeft(p *player) float64 {
	return p.X - playerWidth/2
}

func playerRight(p *player) float64 {
	return p.X + playerWidth/2
}

func playerTop(p *player) float64 {
	return p.Y - playerHeight/2
}

func playerBottom(p *player) float64 {
	return p.Y + playerHeight/2
}

func rectsOverlap(ax float64, ay float64, aw float64, ah float64, bx float64, by float64, bw float64, bh float64) bool {
	return ax < bx+bw && ax+aw > bx && ay < by+bh && ay+ah > by
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
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
