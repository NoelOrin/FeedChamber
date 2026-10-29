package natsembedded

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestStartEmbeddedServer_ClientCanConnect(t *testing.T) {
	srv, err := Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect embedded nats: %v", err)
	}
	defer nc.Close()
	if !nc.IsConnected() {
		t.Fatal("expected connected")
	}
}

func TestStartEmbeddedServer_JetStreamAvailable(t *testing.T) {
	srv, err := Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL(), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatalf("connect embedded nats: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	if _, err := js.AccountInfo(); err != nil {
		t.Fatalf("account info: %v", err)
	}
}

func TestStartEmbeddedServerOnPort_Fixed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	srv, err := StartOnPort(port)
	if err != nil {
		t.Fatalf("StartOnPort(%d): %v", port, err)
	}
	defer srv.Shutdown()

	want := fmt.Sprintf("nats://127.0.0.1:%d", port)
	if srv.ClientURL() != want {
		t.Fatalf("ClientURL = %q, want %q", srv.ClientURL(), want)
	}
}
