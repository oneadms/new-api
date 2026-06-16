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
		MapWidth:          1600,
		MapHeight:         900,
		PlayerSpeed:       260,
		BulletSpeed:       780,
		FireCooldownMs:    220,
		CapQuota:          100,
		MaxRoundLossQuota: 5000,
		MaxRoundGainQuota: 5000,
		MaxDailyLossQuota: 20000,
		MaxDailyGainQuota: 20000,
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
	for _, prefix := range []string{"bullet", "cap-settle", "room"} {
		for range 20 {
			id := newBattleObjectId(prefix)
			assert.LessOrEqual(t, len(id), 80)
			assert.NotContains(t, seen, id)
			seen[id] = struct{}{}
		}
	}
}

func TestUpdatePlayerUsesPlatformMovementJumpAndCapThrow(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	floorY := float64(settings.MapHeight) - 40
	player := &player{
		UserId:    1,
		X:         400,
		Y:         floorY - playerHeight/2,
		Alive:     true,
		Direction: -1,
		OnGround:  true,
		LastAimX:  1,
		Input: PlayerInput{
			Right: true,
			Shoot: true,
			Jump:  true,
		},
	}

	room.updatePlayer(player, 0.016, now, settings)

	assert.Greater(t, player.X, float64(400))
	assert.Less(t, player.Y, floorY-playerHeight/2)
	assert.Equal(t, 1, player.Direction)
	assert.False(t, player.OnGround)
	assert.Less(t, player.VY, float64(0))
	require.Len(t, room.bullets, 1)
	for _, bullet := range room.bullets {
		assert.Equal(t, player.UserId, bullet.OwnerId)
		assert.Equal(t, float64(settings.BulletSpeed), bullet.VX)
		assert.Equal(t, capThrowUpVelocity, bullet.VY)
	}

	room.updatePlayer(player, 0.01, now.Add(100*time.Millisecond), settings)
	assert.Len(t, room.bullets, 1)
}

func TestBroadcastSnapshotIncludesPlayerAckSeq(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	player := &player{
		UserId:   1,
		Username: "player",
		X:        50,
		Y:        50,
		Alive:    true,
		InputSeq: 42,
		CapStack: 7,
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
	require.NotEmpty(t, snapshot.Platforms)
	require.Len(t, snapshot.Players, 1)
	assert.Equal(t, 7, snapshot.Players[0].CapStack)
}

func TestUpdateCapsStacksCapsOnTargetHead(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	attacker := &player{UserId: 1, X: 500, Y: 500, Alive: true}
	target := &player{UserId: 2, X: 560, Y: 500, Alive: true}
	room.players[attacker.UserId] = attacker
	room.players[target.UserId] = target

	room.bullets["cap"] = &bullet{
		Id:        "cap",
		OwnerId:   attacker.UserId,
		X:         target.X,
		Y:         target.Y - playerHeight/2 + 8,
		ExpiresAt: now.Add(time.Second),
	}
	room.updateCaps(now, 0.01, settings)

	assert.Equal(t, 1, target.CapStack)
	assert.Equal(t, 1, target.CapSources[attacker.UserId])
	assert.True(t, target.Alive)
	assert.Empty(t, room.bullets)
	require.Len(t, room.events, 1)
	assert.Equal(t, eventTypeHit, room.events[0].Type)
}
