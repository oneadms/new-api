package battle

import (
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
		DropPickupRadius:  38,
		DropExpireSeconds: 18,
	}
}

func stubBattleQuota(t *testing.T, quotas map[int]int) {
	t.Helper()
	originalQuota := getBattleUserQuota
	originalUsage := getBattleQuotaUsageSince
	getBattleUserQuota = func(userId int, _ bool) (int, error) {
		return quotas[userId], nil
	}
	getBattleQuotaUsageSince = func(int, int64) (model.BattleQuotaUsage, error) {
		return model.BattleQuotaUsage{}, nil
	}
	t.Cleanup(func() {
		getBattleUserQuota = originalQuota
		getBattleQuotaUsageSince = originalUsage
	})
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

func TestUpdatePlayerDownDoesNotDropThroughSolidFloor(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	floor := battleTestPlatform(t, settings, "floor")
	player := &player{
		UserId:   1,
		X:        floor.X + floor.W/2,
		Y:        floor.Y - playerHeight/2,
		Alive:    true,
		OnGround: true,
		Input: PlayerInput{
			Down: true,
		},
	}

	room.updatePlayer(player, 0.016, now, settings)

	assert.InDelta(t, floor.Y-playerHeight/2, player.Y, 0.001)
	assert.True(t, player.OnGround)
	assert.Zero(t, player.VY)
}

func TestUpdatePlayerDownDropsThroughOneWayPlatform(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	platform := battleTestPlatform(t, settings, "center-low")
	player := &player{
		UserId:   1,
		X:        platform.X + platform.W/2,
		Y:        platform.Y - playerHeight/2,
		Alive:    true,
		OnGround: true,
		Input: PlayerInput{
			Down: true,
		},
	}

	room.updatePlayer(player, 0.016, now, settings)

	assert.Greater(t, player.Y, platform.Y-playerHeight/2)
	assert.False(t, player.OnGround)
}

func TestPlayerCanMoveAwayFromRightCorner(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	floor := battleTestPlatform(t, settings, "floor")
	player := &player{
		UserId:    1,
		X:         float64(settings.MapWidth) - playerWidth/2,
		Y:         floor.Y - playerHeight/2,
		Alive:     true,
		OnGround:  true,
		Direction: -1,
		Input: PlayerInput{
			Left: true,
		},
	}

	startX := player.X
	room.updatePlayer(player, 0.1, now, settings)

	assert.Less(t, player.X, startX)
}

func TestClearPendingRewardsForLeavingPlayer(t *testing.T) {
	room := newRoom("test", NewManager())
	target := &player{
		UserId:     2,
		CapStack:   4,
		CapSources: map[int]int{1: 3, 3: 1},
	}
	room.players[target.UserId] = target

	room.clearPendingRewardsForUser(1)

	assert.Equal(t, 1, target.CapStack)
	assert.NotContains(t, target.CapSources, 1)
	assert.Equal(t, 1, target.CapSources[3])
}

func TestMatchModeWaitsForScheduledStartAndPlayers(t *testing.T) {
	room := newRoom("test", NewManager())
	now := time.Now()
	settings := battleSimulationSettings()
	settings.MatchModeEnabled = true
	settings.MatchMinPlayers = 2
	settings.MatchDurationSecs = 90
	settings.MatchStartAt = now.Add(time.Minute).Unix()
	room.players[1] = &player{UserId: 1, Alive: true}
	room.players[2] = &player{UserId: 2, Alive: true}

	paused := room.updateMatchState(now, settings)

	assert.True(t, paused)
	assert.Equal(t, matchPhaseWaiting, room.matchPhase)

	paused = room.updateMatchState(now.Add(time.Minute), settings)

	assert.False(t, paused)
	assert.Equal(t, matchPhaseRunning, room.matchPhase)
	assert.Equal(t, now.Add(time.Minute+90*time.Second), room.matchEndsAt)
	require.Len(t, room.events, 1)
	assert.Equal(t, eventTypeMatchStarted, room.events[0].Type)
}

func TestBroadcastSnapshotIncludesPlayerAckSeq(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	player := &player{
		UserId:        1,
		Username:      "player",
		X:             50,
		Y:             50,
		Alive:         true,
		InputSeq:      42,
		CapStack:      7,
		CapStormUntil: now.Add(time.Second),
	}
	room.players[player.UserId] = player
	room.bullets["storm-cap"] = &bullet{
		Id:      "storm-cap",
		Kind:    bulletKindCapStorm,
		OwnerId: player.UserId,
		X:       120,
		Y:       80,
	}
	room.powerups["storm"] = &powerup{
		Id:        "storm",
		Type:      powerupTypeCapStorm,
		X:         240,
		Y:         300,
		ExpiresAt: now.Add(time.Second),
	}
	room.clients[player.UserId] = &Client{
		userId: player.UserId,
		send:   make(chan []byte, 1),
		room:   room,
	}

	room.broadcastSnapshot(now, settings)

	data := <-room.clients[player.UserId].send
	var snapshot Snapshot
	require.NoError(t, common.Unmarshal(data, &snapshot))
	assert.Equal(t, int64(42), snapshot.AckSeq)
	require.NotEmpty(t, snapshot.Platforms)
	require.Len(t, snapshot.Players, 1)
	assert.Equal(t, 7, snapshot.Players[0].CapStack)
	assert.Equal(t, player.CapStormUntil.UnixMilli(), snapshot.Players[0].CapStormUntil)
	require.Len(t, snapshot.Bullets, 1)
	assert.Equal(t, bulletKindCapStorm, snapshot.Bullets[0].Kind)
	require.Len(t, snapshot.Powerups, 1)
	assert.Equal(t, powerupTypeCapStorm, snapshot.Powerups[0].Type)
}

func TestUpdateCapsStacksCapsOnTargetHead(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	stubBattleQuota(t, map[int]int{1: 1000, 2: 1000})
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

func TestUpdateCapsMarksInvalidWhenTargetCannotCoverCap(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	stubBattleQuota(t, map[int]int{1: 1000, 2: 50})
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

	assert.Zero(t, target.CapStack)
	assert.Empty(t, target.CapSources)
	assert.Empty(t, room.bullets)
	require.Len(t, room.events, 1)
	assert.Equal(t, eventTypeCapInvalid, room.events[0].Type)
	assert.Equal(t, attacker.UserId, room.events[0].UserId)
	assert.Equal(t, target.UserId, room.events[0].TargetUserId)
	assert.Equal(t, 1, room.events[0].CapCount)
	assert.Equal(t, capInvalidReasonTarget, room.events[0].Reason)
}

func TestUpdateCapsMarksInvalidWhenThrowerCannotCoverPendingRewards(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	stubBattleQuota(t, map[int]int{1: 100, 2: 1000, 3: 1000})
	now := time.Now()
	attacker := &player{UserId: 1, X: 500, Y: 500, Alive: true}
	existingTarget := &player{
		UserId:     3,
		CapStack:   1,
		CapSources: map[int]int{1: 1},
		Alive:      true,
	}
	target := &player{UserId: 2, X: 560, Y: 500, Alive: true}
	room.players[attacker.UserId] = attacker
	room.players[target.UserId] = target
	room.players[existingTarget.UserId] = existingTarget

	room.bullets["cap"] = &bullet{
		Id:        "cap",
		OwnerId:   attacker.UserId,
		X:         target.X,
		Y:         target.Y - playerHeight/2 + 8,
		ExpiresAt: now.Add(time.Second),
	}
	room.updateCaps(now, 0.01, settings)

	assert.Zero(t, target.CapStack)
	require.Len(t, room.events, 1)
	assert.Equal(t, eventTypeCapInvalid, room.events[0].Type)
	assert.Equal(t, capInvalidReasonThrower, room.events[0].Reason)
}

func TestUpdatePowerupPickupsActivatesCapStorm(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	player := &player{
		UserId: 1,
		X:      200,
		Y:      300,
		Alive:  true,
	}
	room.players[player.UserId] = player
	room.powerups["storm"] = &powerup{
		Id:        "storm",
		Type:      powerupTypeCapStorm,
		X:         player.X,
		Y:         player.Y - playerHeight*0.18,
		ExpiresAt: now.Add(time.Second),
	}

	room.updatePowerupPickups(now, settings)

	assert.Empty(t, room.powerups)
	assert.True(t, player.capStormActive(now))
	assert.Equal(t, now.Add(capStormDuration), player.CapStormUntil)
	require.Len(t, room.events, 1)
	assert.Equal(t, eventTypePowerupPickup, room.events[0].Type)
	assert.Equal(t, player.UserId, room.events[0].UserId)
}

func TestTrySpawnCapStormLaunchesOneProjectilePerHeadCap(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	now := time.Now()
	player := &player{
		UserId:        1,
		X:             400,
		Y:             500,
		Alive:         true,
		Direction:     1,
		CapStack:      4,
		CapStormUntil: now.Add(capStormDuration),
	}

	spawned := room.trySpawnCapStorm(player, now, settings)

	require.True(t, spawned)
	require.Len(t, room.bullets, player.CapStack)
	assert.Equal(t, 4, player.CapStack)
	assert.Equal(t, now, player.LastStormThrow)
	var attemptId string
	for _, bullet := range room.bullets {
		assert.Equal(t, bulletKindCapStorm, bullet.Kind)
		assert.Equal(t, player.UserId, bullet.OwnerId)
		assert.NotEmpty(t, bullet.AttemptId)
		if attemptId == "" {
			attemptId = bullet.AttemptId
		}
		assert.Equal(t, attemptId, bullet.AttemptId)
	}

	assert.False(t, room.trySpawnCapStorm(player, now.Add(100*time.Millisecond), settings))
	assert.Len(t, room.bullets, player.CapStack)
}

func TestCapStormHitTransfersWholeStackToTarget(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	stubBattleQuota(t, map[int]int{1: 1000, 3: 1000})
	now := time.Now()
	owner := &player{
		UserId:        1,
		X:             400,
		Y:             500,
		Alive:         true,
		CapStack:      5,
		CapSources:    map[int]int{2: 5},
		CapStormUntil: now.Add(capStormDuration),
	}
	target := &player{
		UserId:     3,
		X:          560,
		Y:          500,
		Alive:      true,
		CapStack:   1,
		CapSources: map[int]int{4: 1},
	}
	room.players[owner.UserId] = owner
	room.players[target.UserId] = target
	room.bullets["storm-1"] = &bullet{
		Id:        "storm-1",
		Kind:      bulletKindCapStorm,
		OwnerId:   owner.UserId,
		AttemptId: "attempt",
		X:         target.X,
		Y:         target.Y - playerHeight/2 + 8,
		ExpiresAt: now.Add(time.Second),
	}
	room.bullets["storm-2"] = &bullet{
		Id:        "storm-2",
		Kind:      bulletKindCapStorm,
		OwnerId:   owner.UserId,
		AttemptId: "attempt",
		X:         target.X,
		Y:         target.Y - playerHeight/2 + 8,
		ExpiresAt: now.Add(time.Second),
	}

	room.handleCapStormHit(room.bullets["storm-1"], target, settings)

	assert.Zero(t, owner.CapStack)
	assert.Empty(t, owner.CapSources)
	assert.True(t, owner.CapStormUntil.IsZero())
	assert.Equal(t, 6, target.CapStack)
	assert.Equal(t, 5, target.CapSources[owner.UserId])
	assert.Equal(t, 1, target.CapSources[4])
	assert.Empty(t, room.bullets)
	require.Len(t, room.events, 1)
	assert.Equal(t, eventTypeCapStormHit, room.events[0].Type)
	assert.Equal(t, owner.UserId, room.events[0].UserId)
	assert.Equal(t, target.UserId, room.events[0].TargetUserId)
	assert.Equal(t, 5, room.events[0].CapCount)
}

func TestCapStormHitLeavesUnaffordableCapsOnOwner(t *testing.T) {
	room := newRoom("test", NewManager())
	settings := battleSimulationSettings()
	stubBattleQuota(t, map[int]int{1: 1000, 3: 250})
	now := time.Now()
	owner := &player{
		UserId:        1,
		X:             400,
		Y:             500,
		Alive:         true,
		CapStack:      5,
		CapSources:    map[int]int{2: 5},
		CapStormUntil: now.Add(capStormDuration),
	}
	target := &player{UserId: 3, X: 560, Y: 500, Alive: true}
	room.players[owner.UserId] = owner
	room.players[target.UserId] = target
	room.bullets["storm-1"] = &bullet{
		Id:        "storm-1",
		Kind:      bulletKindCapStorm,
		OwnerId:   owner.UserId,
		AttemptId: "attempt",
		X:         target.X,
		Y:         target.Y - playerHeight/2 + 8,
		ExpiresAt: now.Add(time.Second),
	}

	room.handleCapStormHit(room.bullets["storm-1"], target, settings)

	assert.Equal(t, 3, owner.CapStack)
	assert.Equal(t, 3, owner.CapSources[2])
	assert.Equal(t, 2, target.CapStack)
	assert.Equal(t, 2, target.CapSources[owner.UserId])
	require.Len(t, room.events, 2)
	assert.Equal(t, eventTypeCapStormHit, room.events[0].Type)
	assert.Equal(t, 2, room.events[0].CapCount)
	assert.Equal(t, eventTypeCapInvalid, room.events[1].Type)
	assert.Equal(t, 3, room.events[1].CapCount)
	assert.Equal(t, capInvalidReasonTarget, room.events[1].Reason)
}

func battleTestPlatform(t *testing.T, settings operation_setting.BattleSetting, id string) platform {
	t.Helper()
	for _, item := range battlePlatforms(settings) {
		if item.Id == id {
			return item
		}
	}
	t.Fatalf("platform %q not found", id)
	return platform{}
}
