package agents

import (
	"fmt"
	"net"
	"time"

	"sdg-git.solar.local/golang/aquatone/core"
)

type TCPPortScanner struct {
	session *core.Session
}

func NewTCPPortScanner() *TCPPortScanner {
	return &TCPPortScanner{}
}

func (ps *TCPPortScanner) ID() string {
	return "agent:tcp_port_scanner"
}

func (ps *TCPPortScanner) Register(s *core.Session) error {
	err := s.EventBus.SubscribeAsync(core.Host, ps.OnHost, false)
	if err != nil {
		return err
	}

	ps.session = s

	return nil
}

func (ps *TCPPortScanner) OnHost(host string) {
	ps.session.Out.Debug("[%s] Received new host: %s\n", ps.ID(), host)
	for _, port := range ps.session.Ports {
		ps.session.WaitGroup.Add()
		go func(port int, host string) {
			defer ps.session.WaitGroup.Done()
			if ps.scanPort(port, host) {
				ps.session.Stats.IncrementPortOpen()
				ps.session.Out.Info(
					"%s: port %s %s\n",
					host,
					ps.session.Out.Green(fmt.Sprintf("%d", port)),
					ps.session.Out.Green("open"),
				)
				ps.session.EventBus.Publish(core.TCPPort, port, host)
			} else {
				ps.session.Stats.IncrementPortClosed()
				ps.session.Out.Debug("[%s] Port %d is closed on %s\n", ps.ID(), port, host)
			}
		}(port, host)
	}
}

func (ps *TCPPortScanner) scanPort(port int, host string) bool {
	conn, _ := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Duration(*ps.session.Options.ScanTimeout)*time.Millisecond)
	if conn != nil {
		_ = conn.Close()
		return true
	}
	return false
}
