package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSubscriptionDisabledGroupsByJSONString(t *testing.T) {
	original := SubscriptionDisabledGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateSubscriptionDisabledGroupsByJSONString(original))
	})

	require.NoError(t, UpdateSubscriptionDisabledGroupsByJSONString(`[" vip ","auto","vip",""]`))
	assert.Equal(t, `["auto","vip"]`, SubscriptionDisabledGroups2JSONString())
	assert.True(t, IsSubscriptionDisabledGroup("vip"))
	assert.False(t, IsSubscriptionDisabledGroup("default"))

	require.Error(t, UpdateSubscriptionDisabledGroupsByJSONString(`{"vip":true}`))
	assert.Equal(t, `["auto","vip"]`, SubscriptionDisabledGroups2JSONString())
}
