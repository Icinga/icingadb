package notifications

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/redis"
	"github.com/icinga/icingadb/internal/config"
	"github.com/icinga/icingadb/pkg/common"
	"github.com/icinga/icingadb/pkg/icingadb"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNeedsSubmission(t *testing.T) {
	const objectID = "c0ffee"

	incident := func(severity event.Severity) map[string]incidentResp {
		return map[string]incidentResp{objectID: {Incident: source.Incident{Severity: severity}}}
	}

	tests := []struct {
		name      string
		incidents map[string]incidentResp
		hardState uint8
		want      bool
	}{
		{"problem without incident", map[string]incidentResp{}, 1, true},
		{"OK without incident", map[string]incidentResp{}, 0, false},
		{"problem with same incident", incident(event.SeverityCritical), 1, false},
		{"problem with different incident", incident(event.SeverityWarning), 1, true},
		{"OK with incident", incident(event.SeverityCritical), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := needsSubmission(tt.incidents, objectID, &v1.State{HardState: tt.hardState}, false)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestClient_ApplyDelta_Incidents makes sure that Client.ApplyDelta goes on to submit the entities of the delta
// if Icinga Notifications has no incidents at all, e.g. when it was freshly installed, but not if the incidents
// could not be fetched.
//
// There is no real Redis server, only a listener which counts the connections made to it, as that tells whether
// ApplyDelta tried to fetch the entities from Redis, which is what it does right before submitting them.
func TestClient_ApplyDelta_Incidents(t *testing.T) {
	var incidentRequests atomic.Int32
	var incidentsFailing atomic.Bool

	api := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/incidents", r.URL.Path)
		incidentRequests.Add(1)

		if incidentsFailing.Load() {
			http.Error(rw, "boom", http.StatusInternalServerError)
		} else {
			rw.WriteHeader(http.StatusAccepted) // No incidents.
		}
	}))
	defer api.Close()

	var redisConns atomic.Int32
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			redisConns.Add(1)
			_ = conn.Close()
		}
	}()

	redisAddr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)

	logger := logging.NewLogger(zap.NewNop().Sugar(), time.Second)
	rc, err := redis.NewClientFromConfig(&redis.Config{
		Host:    "127.0.0.1",
		Port:    redisAddr.Port,
		Options: redis.Options{HMGetCount: 1, MaxHMGetConnections: 1, Timeout: time.Second},
	}, logger)
	require.NoError(t, err)

	client := &Client{redisClient: rc, logger: logger, ha: new(icingadb.HA)}
	client.currentAPIClient.Store(&apiClientSession{})
	_, err = client.exchangeAPIClientSession(config.NotificationsConfig{Config: source.Config{Url: api.URL}}, true)
	require.NoError(t, err)

	env := &v1.Environment{}
	ctx := env.NewContext(context.Background())
	delta := &icingadb.Delta{
		Subject:       common.NewSyncSubject(v1.NewHostState),
		RedisSnapshot: map[string]struct{}{"c0ffee": {}},
	}

	// The incidents are unavailable, so we can't tell what has to be submitted.
	incidentsFailing.Store(true)
	assert.NoError(t, client.ApplyDelta(ctx, delta))
	assert.Equal(t, int32(1), incidentRequests.Load())
	assert.Zero(t, redisConns.Load(), "entities must not be submitted without knowing the incidents")

	// The failure above wasn't cached, so the incidents are fetched again by the next call. Not having any incidents
	// is no reason to stop, ApplyDelta has to carry on.
	incidentsFailing.Store(false)
	_ = client.ApplyDelta(ctx, delta) // Fails as there is no Redis, but only after it has been asked for the entities.
	assert.Equal(t, int32(2), incidentRequests.Load())
	assert.NotZero(t, redisConns.Load(), "entities must be submitted even though there are no incidents")

	// The (empty) incidents are fetched only once.
	redisConns.Store(0)
	_ = client.ApplyDelta(ctx, delta)
	assert.Equal(t, int32(2), incidentRequests.Load())
	assert.NotZero(t, redisConns.Load())
}
