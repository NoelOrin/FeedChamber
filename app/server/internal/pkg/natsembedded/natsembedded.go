package natsembedded

import (
	"fmt"
	"os"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

// Server 是进程内 nats-server 的句柄。
type Server struct {
	ns       *natsserver.Server
	storeDir string
}

// Start 启动一个监听随机端口的进程内 NATS（含 JetStream）。
func Start() (*Server, error) {
	return StartOnPort(0)
}

// StartOnPort 启动进程内 NATS。port <= 0 时使用随机端口，
// port > 0 时固定监听 127.0.0.1:port。
func StartOnPort(port int) (*Server, error) {
	dir, err := os.MkdirTemp("", "orinforum-nats-js-*")
	if err != nil {
		return nil, fmt.Errorf("nats embedded: temp store dir: %w", err)
	}

	listenPort := port
	if listenPort <= 0 {
		listenPort = -1
	}

	opts := &natsserver.Options{
		Host:       "127.0.0.1",
		Port:       listenPort,
		NoLog:      true,
		NoSigs:     true,
		MaxPayload: 1 << 20,
		JetStream:  true,
		StoreDir:   dir,
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("nats embedded: new server: %w", err)
	}
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		ns.Shutdown()
		_ = os.RemoveAll(dir)
		if port > 0 {
			return nil, fmt.Errorf("nats embedded: not ready for connections on 127.0.0.1:%d", port)
		}
		return nil, fmt.Errorf("nats embedded: not ready for connections")
	}
	return &Server{ns: ns, storeDir: dir}, nil
}

// ClientURL 返回内嵌 NATS 的客户端连接地址。
func (s *Server) ClientURL() string {
	if s == nil || s.ns == nil {
		return ""
	}
	return s.ns.ClientURL()
}

// Shutdown 关闭内嵌 NATS 并清理临时 JetStream 数据目录。
func (s *Server) Shutdown() {
	if s == nil {
		return
	}
	if s.ns != nil {
		s.ns.Shutdown()
		s.ns.WaitForShutdown()
		s.ns = nil
	}
	if s.storeDir != "" {
		_ = os.RemoveAll(s.storeDir)
		s.storeDir = ""
	}
}
