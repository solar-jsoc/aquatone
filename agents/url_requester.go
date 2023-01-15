package agents

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/parnurzeal/gorequest"
	"sdg-git.solar.local/golang/aquatone/core"
)

type URLRequester struct {
	session *core.Session
}

func NewURLRequester() *URLRequester {
	return &URLRequester{}
}

func (ur *URLRequester) ID() string {
	return "agent:url_requester"
}

func (ur *URLRequester) Register(s *core.Session) error {
	err := s.EventBus.SubscribeAsync(core.URL, ur.OnURL, false)
	if err != nil {
		return err
	}

	ur.session = s

	return nil
}

func (ur *URLRequester) OnURL(url string) {
	ur.session.Out.Debug("[%s] Received new URL %s\n", ur.ID(), url)

	ur.session.WaitGroup.Add()
	go func(url string) {
		defer ur.session.WaitGroup.Done()

		http := Gorequest(ur.session.Options)
		resp, _, errs := http.Get(url).
			Set("User-Agent", RandomUserAgent()).
			Set("X-Forwarded-For", RandomIPv4Address()).
			Set("Via", fmt.Sprintf("1.1 %s", RandomIPv4Address())).
			Set("Forwarded", fmt.Sprintf("for=%s;proto=http;by=%s", RandomIPv4Address(), RandomIPv4Address())).End()

		if errs != nil {
			ur.session.Stats.IncrementRequestFailed()
			for _, err := range errs {
				ur.session.Out.Debug("[%s] Error: %v\n", ur.ID(), err)
				if os.IsTimeout(err) {
					ur.session.Out.Error("%s: request timeout\n", url)
					return
				}
			}
			ur.session.Out.Debug("%s: failed\n", url)
			return
		}

		ur.session.Stats.IncrementRequestSuccessful()

		var status string
		if resp.StatusCode >= 500 {
			ur.session.Stats.IncrementResponseCode5xx()
			status = ur.session.Out.Red(resp.Status)
		} else if resp.StatusCode >= 400 {
			ur.session.Stats.IncrementResponseCode4xx()
			status = ur.session.Out.Yellow(resp.Status)
		} else if resp.StatusCode >= 300 {
			ur.session.Stats.IncrementResponseCode3xx()
			status = ur.session.Out.Green(resp.Status)
		} else {
			ur.session.Stats.IncrementResponseCode2xx()
			status = ur.session.Out.Green(resp.Status)
		}

		ur.session.Out.Info("%s: %s\n", url, status)

		page, err := ur.createPageFromResponse(url, resp)
		if err != nil {
			ur.session.Out.Debug("[%s] Error: %v\n", ur.ID(), err)
			ur.session.Out.Error("Failed to create page for URL: %s\n", url)
			return
		}

		ur.writeHeaders(page)
		if *ur.session.Options.SaveBody {
			ur.writeBody(page, resp)
		}

		ur.session.EventBus.Publish(core.URLResponsive, url)
	}(url)
}

func (ur *URLRequester) createPageFromResponse(url string, resp gorequest.Response) (*core.Page, error) {
	page, err := ur.session.AddPage(url)
	if err != nil {
		return nil, err
	}

	page.Status = resp.Status
	for name, value := range resp.Header {
		page.AddHeader(name, strings.Join(value, " "))
	}

	return page, nil
}

func (ur *URLRequester) writeHeaders(page *core.Page) {
	filepath := fmt.Sprintf("headers/%s.txt", page.BaseFilename())
	headers := fmt.Sprintf("%s\n", page.Status)

	for _, header := range page.Headers {
		headers += fmt.Sprintf("%v: %v\n", header.Name, header.Value)
	}

	if err := os.WriteFile(ur.session.GetFilePath(filepath), []byte(headers), 0644); err != nil {
		ur.session.Out.Debug("[%s] Error: %v\n", ur.ID(), err)
		ur.session.Out.Error("Failed to write HTTP response headers for %s to %s\n", page.URL, ur.session.GetFilePath(filepath))
	}

	if saved := ur.session.IsFileSaved(ur.session.GetFilePath(filepath), 30*time.Second); !saved {
		ur.session.Out.Error("Failed to write HTTP response headers for %s to %s\n", page.URL, ur.session.GetFilePath(filepath))
	}

	page.HeadersPath = filepath
}

func (ur *URLRequester) writeBody(page *core.Page, resp gorequest.Response) {
	filepath := fmt.Sprintf("html/%s.html", page.BaseFilename())
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ur.session.Out.Debug("[%s] Error: %v\n", ur.ID(), err)
		ur.session.Out.Error("Failed to read response body for %s\n", page.URL)
		return
	}

	if err = os.WriteFile(ur.session.GetFilePath(filepath), body, 0644); err != nil {
		ur.session.Out.Debug("[%s] Error: %v\n", ur.ID(), err)
		ur.session.Out.Error("Failed to write HTTP response body for %s to %s\n", page.URL, ur.session.GetFilePath(filepath))
	}

	if saved := ur.session.IsFileSaved(ur.session.GetFilePath(filepath), 30*time.Second); !saved {
		ur.session.Out.Error("Failed to write HTTP response body for %s to %s\n", page.URL, ur.session.GetFilePath(filepath))
	}

	page.BodyPath = filepath
}
