package battle

import (
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func battleSimulationSettings() operation_setting.BattleSetting {
	return operation_setting.BattleSetting{
		MapWidth:          200,
		MapHeight:         120,
		PlayerSpeed:       100,
		BulletSpeed:       500,
		BulletDamage:      60,
		FireCooldownMs:    200,
		RespawnSeconds:    3,
		MaxRoundLossQuota: 0,
	}
}

func TestNormalizeRoomId(t *testing.T) {
	assert.Equal(t, "lobby", normalizeRoomId(""))
	assert.Equal(t, "room-42_test", normalizeRoomId(" room-42_test! "))
	assert.Equal(t, "lobby", normalizeRoomId("中文房间"))
	assert.Len(t, normalizeRoomId("abcdefghijklmnopqrstuvwxyz0123456789-extra"), 40)
}

func TestSanitizeInput(t *testing.T) {
	invalid := sanitizeInput(PlayerInput{AimX: math.NaN(), AimY: math.Inf(1)})
	assert.Equal(t, float64(1), invalid.AimX)
	assert.Zero(t, invalid.AimY)

	clamped := sanitizeInput(PlayerInput{AimX: 3, AimY: -4})
	assert.Equal(t, float64(1), clamped.AimX)
	assert.Equal(t, float64(-1), clamped.AimY)
}

func TestBattleObjectIdsFitRecordColumn(t *testing.T) {
	seen := make(map[string]struct{})
	for _, prefix := range []string{"drop", "pickup", "bullet"} {
		for range 20 {
			id := newBattleObjectId(prefix)
			assert.LessOrEqual(t, len(id), 80)
			assert.NotContains(t, seen, id)
			seen[id] = struct{}{}
		}
	}
}

func TestUpdatePlayerUsesServerMovementAndBulletRules(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	player := &player{
		UserId:   1,
		X:        175,
		Y:        60,
		HP:       100,
		Alive:    true,
		LastAimX: 1,
		Input: PlayerInput{
			Right: true,
			Shoot: true,
			AimX:  4,
		},
	}

	room.updatePlayer(player, 1, now, settings)

	assert.Equal(t, float64(settings.MapWidth-playerRadius), player.X)
	assert.Equal(t, float64(60), player.Y)
	assert.Equal(t, float64(1), player.LastAimX)
	assert.Zero(t, player.LastAimY)
	require.Len(t, room.bullets, 1)
	for _, bullet := range room.bullets {
		assert.Equal(t, player.UserId, bullet.OwnerId)
		assert.Equal(t, float64(settings.BulletSpeed), bullet.VX)
		assert.Zero(t, bullet.VY)
	}

	room.updatePlayer(player, 0.01, now.Add(100*time.Millisecond), settings)
	assert.Len(t, room.bullets, 1)
	room.updatePlayer(player, 0.01, now.Add(250*time.Millisecond), settings)
	assert.Len(t, room.bullets, 2)
}

func TestBroadcastSnapshotIncludesPlayerAckSeq(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	player := &player{
		UserId:   1,
		Username: "player",
		X:        50,
		Y:        50,
		HP:       100,
		Alive:    true,
		InputSeq: 42,
	}
	room.players[player.UserId] = player
	room.clients[player.UserId] = &Client{
		userId: player.UserId,
		send:   make(chan []byte, 1),
		room:   room,
	}

	room.broadcastSnapshot(time.Now(), settings)

	data := <-room.clients[player.UserId].send
	var snapshot Snapshot
	require.NoError(t, common.Unmarshal(data, &snapshot))
	assert.Equal(t, int64(42), snapshot.AckSeq)
}

func TestUpdateBulletsAppliesServerHitAndKnockout(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	attacker := &player{UserId: 1, X: 30, Y: 60, HP: 100, Alive: true}
	target := &player{UserId: 2, X: 80, Y: 60, HP: 100, Alive: true}
	room.players[attacker.UserId] = attacker
	room.players[target.UserId] = target

	room.bullets["first"] = &bullet{
		Id:        "first",
		OwnerId:   attacker.UserId,
		X:         target.X,
		Y:         target.Y,
		Damage:    settings.BulletDamage,
		ExpiresAt: now.Add(time.Second),
	}
	room.updateBullets(now, 0.01, settings)
	assert.Equal(t, 40, target.HP)
	assert.True(t, target.Alive)
	assert.Empty(t, room.bullets)

	room.bullets["second"] = &bullet{
		Id:        "second",
		OwnerId:   attacker.UserId,
		X:         target.X,
		Y:         target.Y,
		Damage:    settings.BulletDamage,
		ExpiresAt: now.Add(time.Second),
	}
	room.updateBullets(now, 0.01, settings)

	assert.Zero(t, target.HP)
	assert.False(t, target.Alive)
	assert.Equal(t, 1, target.Deaths)
	assert.Equal(t, 1, attacker.Score)
	assert.Equal(t, now.Add(3*time.Second), target.RespawnAt)
	require.Len(t, room.events, 3)
	assert.Equal(t, eventTypeHit, room.events[0].Type)
	assert.Equal(t, eventTypeHit, room.events[1].Type)
	assert.Equal(t, eventTypeKnockout, room.events[2].Type)
}
