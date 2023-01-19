package agents

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"sdg-git.solar.local/golang/aquatone/core"
)

type URLPublisher struct {
	session *core.Session
}

func NewURLPublisher() *URLPublisher {
	return &URLPublisher{}
}

func (up *URLPublisher) ID() string {
	return "agent:url_publisher"
}

func (up *URLPublisher) Register(s *core.Session) error {
	err := s.EventBus.SubscribeAsync(core.TCPPort, up.OnTCPPort, false)
	if err != nil {
		return err
	}

	up.session = s

	return nil
}

func (up *URLPublisher) OnTCPPort(port int, host string) {
	up.session.Out.Debug("[%s] Received new open port on %s: %d\n", up.ID(), host, port)
	var url string
	if up.isTLS(port, host) {
		url = HostAndPortToURL(host, port, "https")
	} else {
		url = HostAndPortToURL(host, port, "http")
	}
	up.session.EventBus.Publish(core.URL, url)
}

func (up *URLPublisher) isTLS(port int, host string) bool {
	if port == 80 {
		return false
	}

	if port == 443 {
		return true
	}

	dialer := &net.Dialer{Timeout: time.Duration(*up.session.Options.HTTPTimeout) * time.Millisecond}
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", host, port), conf)
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}
