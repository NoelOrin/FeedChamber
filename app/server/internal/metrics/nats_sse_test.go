package metrics

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
)

// gatherByName returns the gathered families keyed by name.
func gatherByName(t *testing.T) map[string]*dto.MetricFamily {
	t.Helper()
	mfs, err := Registry.Gather()
	if err != nil {
		t.Fatalf("Registry.Gather() error = %v", err)
	}
	out := make(map[string]*dto.MetricFamily, len(mfs))
	for _, mf := range mfs {
		out[mf.GetName()] = mf
	}
	return out
}

// gaugeValue returns the gauge value of the sample carrying label=want.
func gaugeValue(t *testing.T, mf *dto.MetricFamily, label, want string) (float64, bool) {
	t.Helper()
	for _, m := range mf.GetMetric() {
		if hasLabelValue(m, label, want) {
			return m.GetGauge().GetValue(), true
		}
	}
	return 0, false
}

// counterValue returns the counter value of the sample carrying label=want.
func counterValue(t *testing.T, mf *dto.MetricFamily, label, want string) (float64, bool) {
	t.Helper()
	for _, m := range mf.GetMetric() {
		if hasLabelValue(m, label, want) {
			return m.GetCounter().GetValue(), true
		}
	}
	return 0, false
}

// TestNATSClientsSeededConnected pins the startup state: both clients are
// seeded at 1 because a successful nats.Connect is what created them. A zero
// here would make a healthy instance page on the disconnect alert.
func TestNATSClientsSeededConnected(t *testing.T) {
	byName := gatherByName(t)
	mf := byName["orinforum_nats_connected"]
	if mf == nil {
		t.Fatal("orinforum_nats_connected missing before first observation")
	}

	for _, client := range []string{NATSClientKV, NATSClientEventBus} {
		v, ok := gaugeValue(t, mf, "client", client)
		if !ok {
			t.Errorf("client=%s series missing", client)
			continue
		}
		if v != 1 {
			t.Errorf("client=%s connected = %v, want 1 after a successful connect", client, v)
		}
	}
}

// TestNATSDisconnectAndReconnectTransitions pins the state machine, and pins
// that a disconnect is counted separately from the gauge: the gauge alone
// cannot tell "flapping every minute" from "one outage an hour ago".
func TestNATSDisconnectAndReconnectTransitions(t *testing.T) {
	const client = "transition_probe"

	SeedNATSClient(client)
	byName := gatherByName(t)
	if v, _ := gaugeValue(t, byName["orinforum_nats_connected"], "client", client); v != 1 {
		t.Fatalf("seeded connected = %v, want 1", v)
	}

	ObserveNATSDisconnected(client)
	byName = gatherByName(t)
	if v, _ := gaugeValue(t, byName["orinforum_nats_connected"], "client", client); v != 0 {
		t.Errorf("connected after disconnect = %v, want 0", v)
	}
	if v, _ := counterValue(t, byName["orinforum_nats_disconnects_total"], "client", client); v != 1 {
		t.Errorf("disconnects_total = %v, want 1", v)
	}

	ObserveNATSReconnected(client)
	byName = gatherByName(t)
	if v, _ := gaugeValue(t, byName["orinforum_nats_connected"], "client", client); v != 1 {
		t.Errorf("connected after reconnect = %v, want 1", v)
	}
	if v, _ := counterValue(t, byName["orinforum_nats_reconnects_total"], "client", client); v != 1 {
		t.Errorf("reconnects_total = %v, want 1", v)
	}
	// The disconnect counter must not move on a successful reconnect, or the
	// "flapping" alert (disconnects >> reconnects) would never fire.
	if v, _ := counterValue(t, byName["orinforum_nats_disconnects_total"], "client", client); v != 1 {
		t.Errorf("disconnects_total after reconnect = %v, want still 1", v)
	}
}

// TestSSEChannelsSeeded pins that both channels exist for both families before
// any stream opens, so the dashboard is not blank on a quiet instance.
func TestSSEChannelsSeeded(t *testing.T) {
	byName := gatherByName(t)

	for _, family := range []string{"orinforum_sse_connections", "orinforum_sse_write_failures_total"} {
		mf := byName[family]
		if mf == nil {
			t.Errorf("%s missing before first stream", family)
			continue
		}
		for _, channel := range []string{SSEChannelNotification, SSEChannelRealtime} {
			found := false
			for _, m := range mf.GetMetric() {
				if hasLabelValue(m, "channel", channel) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: channel=%s series missing", family, channel)
			}
		}
	}
}

// TestSSEWriteFailureIsCounted pins that the failure counter increments without
// touching the connection gauge: the gauge is decremented by the handler's
// defer, and folding the two together would hide a delivery failure inside a
// normal disconnect.
func TestSSEWriteFailureIsCounted(t *testing.T) {
	byName := gatherByName(t)
	before, _ := counterValue(t, byName["orinforum_sse_write_failures_total"], "channel", SSEChannelRealtime)
	connBefore, _ := gaugeValue(t, byName["orinforum_sse_connections"], "channel", SSEChannelRealtime)

	ObserveSSEWriteFailure(SSEChannelRealtime)

	byName = gatherByName(t)
	after, _ := counterValue(t, byName["orinforum_sse_write_failures_total"], "channel", SSEChannelRealtime)
	if after != before+1 {
		t.Errorf("write_failures_total = %v, want %v", after, before+1)
	}
	connAfter, _ := gaugeValue(t, byName["orinforum_sse_connections"], "channel", SSEChannelRealtime)
	if connAfter != connBefore {
		t.Errorf("write failure changed the connection gauge: %v -> %v", connBefore, connAfter)
	}
}
