package icingadb_utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestBenchmark(t *testing.T) {
	benchmarc := NewBenchmark()
	time.Sleep(1 * time.Second)
	benchmarc.Stop()
	dur, _ := time.ParseDuration(benchmarc.String())
	assert.InDelta(t, (1 * time.Second).Seconds(), dur.Seconds(), (10 * time.Millisecond).Seconds())

	benchmarc = NewBenchmark()
	time.Sleep(250 * time.Millisecond)
	benchmarc.Stop()
	dur, _ = time.ParseDuration(benchmarc.String())
	assert.InDelta(t, (250 * time.Millisecond).Seconds(), dur.Seconds(), (10 * time.Millisecond).Seconds())

	benchmarc = NewBenchmark()
	time.Sleep(500 * time.Nanosecond)
	benchmarc.Stop()
	dur, _ = time.ParseDuration(benchmarc.String())
	assert.InDelta(t, (500 * time.Nanosecond).Seconds(), dur.Seconds(), (10 * time.Millisecond).Seconds())

	benchmarc = NewBenchmark()
	benchmarc.diff = time.Second * 10
	assert.Equal(t, float64(10), benchmarc.Seconds())

	by, _ := benchmarc.MarshalText()
	assert.Equal(t, []uint8([]byte{0x31, 0x30, 0x2e, 0x30, 0x30, 0x30, 0x30, 0x30, 0x30, 0x73}), by)
}