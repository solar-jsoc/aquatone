package agents

import (
	"fmt"
	"net"

	"sdg-git.solar.local/golang/aquatone/core"
)

type URLHostnameResolver struct {
	session *core.Session
}

func NewURLHostnameResolver() *URLHostnameResolver {
	return &URLHostnameResolver{}
}

func (hr *URLHostnameResolver) ID() string {
	return "agent:url_hostname_resolver"
}

func (hr *URLHostnameResolver) Register(s *core.Session) error {
	err := s.EventBus.SubscribeAsync(core.URLResponsive, hr.OnURLResponsive, false)
	if err != nil {
		return err
	}

	hr.session = s

	return nil
}

func (hr *URLHostnameResolver) OnURLResponsive(url string) {
	hr.session.Out.Debug("[%s] Received new responsive URL %s\n", hr.ID(), url)
	page := hr.session.GetPage(url)
	if page == nil {
		hr.session.Out.Error("Unable to find page for URL: %s\n", url)
		return
	}

	if page.IsIPHost() {
		hr.session.Out.Debug("[%s] Skipping hostname resolving on IP host: %s\n", hr.ID(), url)
		page.Addrs = []string{page.ParsedURL().Hostname()}
		return
	}

	hr.session.WaitGroup.Add()
	go func(page *core.Page) {
		defer hr.session.WaitGroup.Done()
		addrs, err := net.LookupHost(fmt.Sprintf("%s.", page.ParsedURL().Hostname()))
		if err != nil {
			hr.session.Out.Debug("[%s] Error: %v\n", hr.ID(), err)
			hr.session.Out.Error("Failed to resolve hostname for %s\n", page.URL)
			return
		}

		page.Addrs = addrs
	}(page)
}
