package icingadb_ha_lib

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestHA_setResponsibility(t *testing.T) {
	responsibilities := [6]uint32{ resp_ReadyForTakeover, resp_TakeoverNoSync, resp_TakeoverSync, resp_Stop, resp_NotReadyForTakeover }
	h := new(HA)

	previous := uint32(0)
	for _,r := range responsibilities {
		assert.Equal(t, previous, h.setResponsibility(r), "Should be equal")
		previous = r
	}
}

func TestHA_IsResponsible(t *testing.T) {
	h := new(HA)
	h.setResponsibility(resp_TakeoverSync)
	assert.True(t, h.IsResponsible(), "Should be responsible")
	h.setResponsibility(resp_TakeoverNoSync)
	assert.False(t, h.IsResponsible(), "Should not be responsible")
}

func TestHA_icinga2IsAlive(t *testing.T) {
	h := new(HA)
	h.icinga2MTime = time.Now().Unix() - 5
	assert.True(t, h.icinga2IsAlive(), "Should be alive")
	h.icinga2MTime = h.icinga2MTime - 15
	assert.False(t, h.icinga2IsAlive(), "Should be dead")
}