package mailer

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

// ── 测试用假 SMTP 服务器 ────────────────────────────────────────────────────
//
// 不引第三方 SMTP mock：net/smtp 客户端依赖的正是「EHLO 能力通告 / STARTTLS
// 之后才允许 AUTH / DATA 以单独一行的点结束」这些协议细节，自己实现一遍才看得见
// 客户端到底发了什么。

type capturedMail struct {
	mailFrom string
	rcpts    []string
	body     string
	creds    string // AUTH PLAIN 解码结果，形如 "\0user\0pass"
}

type fakeSMTP struct {
	ln     net.Listener
	ca     *fakeCA
	tlsCfg *tls.Config

	implicit     bool // true = 465 式隐式 TLS：accept 后直接套 TLS
	advertiseTLS bool // false = 不通告 STARTTLS/AUTH（模拟不支持 TLS 的中继）

	mail capturedMail
}

// startFakeSMTP 起一个假服务器。advertiseTLS=false 时它只说明文能力，
// 用来钉住「发送方在没有 TLS 的中继上不会走到 AUTH」。
func startFakeSMTP(t *testing.T, implicit, advertiseTLS bool) *fakeSMTP {
	t.Helper()
	ca := newFakeCA(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeSMTP{
		ln:           ln,
		ca:           ca,
		tlsCfg:       &tls.Config{Certificates: []tls.Certificate{ca.leaf}},
		implicit:     implicit,
		advertiseTLS: advertiseTLS,
	}
	go s.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *fakeSMTP) port() int {
	_, p, err := net.SplitHostPort(s.ln.Addr().String())
	if err != nil {
		panic(err)
	}
	var n int
	if _, err := fmt.Sscanf(p, "%d", &n); err != nil {
		panic(err)
	}
	return n
}

func (s *fakeSMTP) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}

func (s *fakeSMTP) handle(c net.Conn) {
	defer c.Close()

	secure := s.implicit
	if secure {
		c = tls.Server(c, s.tlsCfg)
	}
	br := bufio.NewReader(c)

	_ = writeSMTPLine(c, "220 fake ESMTP")
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		up := strings.ToUpper(strings.TrimRight(line, "\r\n"))

		switch {
		case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
			_ = writeSMTPLine(c, s.ehloBody(secure))
		case up == "STARTTLS":
			_ = writeSMTPLine(c, "220 2.0.0 Ready to start TLS")
			tc := tls.Server(c, s.tlsCfg)
			if err := tc.Handshake(); err != nil {
				return
			}
			c, br = tc, bufio.NewReader(tc)
			secure = true
		case strings.HasPrefix(up, "AUTH PLAIN "):
			s.mail.creds = decodePlainAuth(line[len("AUTH PLAIN "):])
			_ = writeSMTPLine(c, "235 2.7.0 Authentication successful")
		case up == "AUTH PLAIN":
			_ = writeSMTPLine(c, "334 "+base64.StdEncoding.EncodeToString([]byte("=")))
			resp, err := br.ReadString('\n')
			if err != nil {
				return
			}
			s.mail.creds = decodePlainAuth(strings.TrimRight(resp, "\r\n"))
			_ = writeSMTPLine(c, "235 2.7.0 Authentication successful")
		case strings.HasPrefix(up, "MAIL FROM:"):
			s.mail.mailFrom = stripAngle(commandArg(line))
			_ = writeSMTPLine(c, "250 2.1.0 OK")
		case strings.HasPrefix(up, "RCPT TO:"):
			s.mail.rcpts = append(s.mail.rcpts, stripAngle(commandArg(line)))
			_ = writeSMTPLine(c, "250 2.1.5 OK")
		case up == "DATA":
			_ = writeSMTPLine(c, "354 Start mail input")
			body, err := readData(br)
			if err != nil {
				return
			}
			s.mail.body = body
			_ = writeSMTPLine(c, "250 2.0.0 OK: queued")
		case up == "QUIT":
			_ = writeSMTPLine(c, "221 bye")
			return
		default:
			_ = writeSMTPLine(c, "250 OK")
		}
	}
}

// ehloBody 通告能力。AUTH 只在连接已加密时给出——真实服务器也这样，
// 顺带保证客户端不会在明文上送凭据。
func (s *fakeSMTP) ehloBody(secure bool) string {
	lines := []string{"250-fake"}
	if s.advertiseTLS && !secure {
		lines = append(lines, "250-STARTTLS")
	}
	if secure {
		lines = append(lines, "250-AUTH PLAIN")
	}
	lines = append(lines, "250 OK")
	return strings.Join(lines, "\r\n")
}

func writeSMTPLine(c net.Conn, line string) error {
	if !strings.HasSuffix(line, "\r\n") {
		line += "\r\n"
	}
	_, err := c.Write([]byte(line))
	return err
}

// readData 读到「单独一行的点」为止，返回去掉 dot-stuffing 后的正文。
func readData(br *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return "", err
		}
		if strings.TrimRight(line, "\r\n") == "." {
			return sb.String(), nil
		}
		sb.WriteString(strings.TrimPrefix(line, "."))
	}
}

func decodePlainAuth(b64 string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimRight(b64, "\r\n"))
	if err != nil {
		return "!malformed:" + b64
	}
	return string(raw)
}

func stripAngle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	return strings.TrimSuffix(s, ">")
}

// commandArg 取「MAIL FROM:<a@b>」里冒号之后的部分，保留原大小写
// （能力通告判断用大写副本，收件人地址不能大写）。
func commandArg(line string) string {
	trimmed := strings.TrimRight(line, "\r\n")
	if i := strings.Index(trimmed, ":"); i >= 0 {
		return strings.TrimSpace(trimmed[i+1:])
	}
	return trimmed
}

// ── 测试 CA ────────────────────────────────────────────────────────────────

type fakeCA struct {
	leaf tls.Certificate
	pool *x509.CertPool
}

// newFakeCA 现签一张 CA + 一张 127.0.0.1 的叶子证书。
// pool 只给测试用：客户端默认走系统信任库，自签证书必须显式注入才会被接受。
func newFakeCA(t *testing.T) *fakeCA {
	t.Helper()

	notBefore, notAfter := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "fake-smtp-ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}

	srvKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("srv key: %v", err)
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("srv cert: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	return &fakeCA{
		leaf: tls.Certificate{
			Certificate: [][]byte{srvDER, caDER},
			PrivateKey:  srvKey,
		},
		pool: pool,
	}
}
