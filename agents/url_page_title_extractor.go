package agents

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"sdg-git.solar.local/golang/aquatone/core"
)

type URLPageTitleExtractor struct {
	session *core.Session
}

func NewURLPageTitleExtractor() *URLPageTitleExtractor {
	return &URLPageTitleExtractor{}
}

func (pe *URLPageTitleExtractor) ID() string {
	return "agent:url_page_title_extractor"
}

func (pe *URLPageTitleExtractor) Register(s *core.Session) error {
	err := s.EventBus.SubscribeAsync(core.URLResponsive, pe.OnURLResponsive, false)
	if err != nil {
		return err
	}

	pe.session = s

	return nil
}

func (pe *URLPageTitleExtractor) OnURLResponsive(url string) {
	pe.session.Out.Debug("[%s] Received new responsive URL %s\n", pe.ID(), url)
	page := pe.session.GetPage(url)
	if page == nil {
		pe.session.Out.Error("Unable to find page for URL: %s\n", url)
		return
	}

	pe.session.WaitGroup.Add()
	go func(page *core.Page) {
		defer pe.session.WaitGroup.Done()
		body, err := pe.session.ReadFile(fmt.Sprintf("html/%s.html", page.BaseFilename()))
		if err != nil {
			pe.session.Out.Debug("[%s] Error reading HTML body file for %s: %s\n", pe.ID(), page.URL, err)
			return
		}

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
		if err != nil {
			pe.session.Out.Debug("[%s] Error when parsing HTML body file for %s: %s\n", pe.ID(), page.URL, err)
			return
		}

		page.PageTitle = strings.TrimSpace(doc.Find("Title").Text())
	}(page)
}
