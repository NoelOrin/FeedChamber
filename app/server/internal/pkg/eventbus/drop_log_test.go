package eventbus

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A full dispatch buffer must produce an observable log line, not just a
// metric increment: without it, a burst that drops events is invisible in the
// logs and only shows up if someone happens to be watching the dashboard.
func TestEnqueue_DropIsLogged(t *testing.T) {
	hook := test.NewGlobal()
	defer logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))

	eb := &EventBus{
		ch:   make(chan Event, 1),
		done: make(chan struct{}),
	}
	// Fill the buffer so the next enqueue hits the default (drop) branch.
	eb.ch <- Event{Type: "first"}

	eb.enqueue(Event{Type: EventContentReplied})

	entries := hook.AllEntries()
	require.NotEmpty(t, entries, "a dropped event must emit a log line")
	last := entries[len(entries)-1]
	assert.Equal(t, logrus.WarnLevel, last.Level)
	assert.Equal(t, EventContentReplied, last.Data["event"])
	assert.Contains(t, last.Message, "drop")
}
