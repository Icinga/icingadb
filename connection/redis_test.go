// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package connection

import (
	"github.com/Icinga/icingadb/config/testbackends"
	"github.com/stretchr/testify/assert"
	"testing"
)

func NewTestRDBW(rdb RedisClient) RDBWrapper {
	return RDBWrapper{rdb}
}

func TestNewRDBWrapper(t *testing.T) {
	rdbw := NewRDBWrapper(testbackends.RedisTestAddr, 64)
	assert.True(t, rdbw.CheckConnection(), "Redis should be connected")

	rdbw = NewRDBWrapper("asdasdasdasdasd:5123", 64)
	assert.False(t, rdbw.CheckConnection(), "Redis should not be connected")
	//TODO: Add more tests here
}

func TestRDBWrapper_PipeConfigChunks(t *testing.T) {
	rdbw := NewTestRDBW(testbackends.RedisTestClient)

	if !rdbw.CheckConnection() {
		t.Fatal("This test needs a working Redis connection")
	}

	testbackends.RedisTestClient.Del("icinga:config:testkey")
	testbackends.RedisTestClient.Del("icinga:checksum:testkey")

	testbackends.RedisTestClient.HSet("icinga:config:testkey", "123534534fsdf12sdas12312adg23423f", "this-should-be-the-config")
	testbackends.RedisTestClient.HSet("icinga:checksum:testkey", "123534534fsdf12sdas12312adg23423f", "this-should-be-the-checksum")

	chChunk := rdbw.PipeConfigChunks(make(chan struct{}), []string{"123534534fsdf12sdas12312adg23423f"}, "testkey")
	chunk := <-chChunk
	assert.Equal(t, "this-should-be-the-config", chunk.Configs[0])
	assert.Equal(t, "this-should-be-the-checksum", chunk.Checksums[0])
}

func TestRDBWrapper_PipeChecksumChunks(t *testing.T) {
	rdbw := NewTestRDBW(testbackends.RedisTestClient)

	if !rdbw.CheckConnection() {
		t.Fatal("This test needs a working Redis connection")
	}

	testbackends.RedisTestClient.Del("icinga:checksum:testkey")

	testbackends.RedisTestClient.HSet("icinga:checksum:testkey", "123534534fsdf12sdas12312adg23423f", "this-should-be-the-checksum")

	chChunk := rdbw.PipeChecksumChunks(make(chan struct{}), []string{"123534534fsdf12sdas12312adg23423f"}, "testkey")
	chunk := <-chChunk
	assert.Equal(t, "this-should-be-the-checksum", chunk.Checksums[0])
}
