package agents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"sdg-git.solar.local/golang/aquatone/core"
)

type FingerprintRegexp struct {
	Regexp *regexp.Regexp
}

type Fingerprint struct {
	Name               string            `json:"name"`
	Categories         []string          `json:"categories"`
	Implies            []Fingerprint     `json:"implies"`
	Website            string            `json:"website"`
	Headers            map[string]string `json:"headers"`
	HTML               []string          `json:"html"`
	Script             []string          `json:"script"`
	Meta               map[string]string `json:"meta"`
	HeaderFingerprints map[string]FingerprintRegexp
	HTMLFingerprints   []FingerprintRegexp
	ScriptFingerprints []FingerprintRegexp
	MetaFingerprints   map[string]FingerprintRegexp
}

func (f *Fingerprint) LoadPatterns() {
	f.HeaderFingerprints = make(map[string]FingerprintRegexp)
	f.MetaFingerprints = make(map[string]FingerprintRegexp)
	for header, pattern := range f.Headers {
		fingerprint, err := f.compilePattern(pattern)
		if err != nil {
			continue
		}
		f.HeaderFingerprints[header] = fingerprint
	}

	for _, pattern := range f.HTML {
		fingerprint, err := f.compilePattern(pattern)
		if err != nil {
			continue
		}
		f.HTMLFingerprints = append(f.HTMLFingerprints, fingerprint)
	}

	for _, pattern := range f.Script {
		fingerprint, err := f.compilePattern(pattern)
		if err != nil {
			continue
		}
		f.ScriptFingerprints = append(f.ScriptFingerprints, fingerprint)
	}

	for meta, pattern := range f.Meta {
		fingerprint, err := f.compilePattern(pattern)
		if err != nil {
			continue
		}
		f.MetaFingerprints[meta] = fingerprint
	}
}

func (f *Fingerprint) compilePattern(p string) (FingerprintRegexp, error) {
	var fingerprint FingerprintRegexp
	r, err := regexp.Compile(p)
	if err != nil {
		return fingerprint, err
	}
	fingerprint.Regexp = r

	return fingerprint, nil
}

type URLTechnologyFingerprinter struct {
	session      *core.Session
	fingerprints []Fingerprint
}

func NewURLTechnologyFingerprinter() *URLTechnologyFingerprinter {
	return &URLTechnologyFingerprinter{}
}

func (uf *URLTechnologyFingerprinter) ID() string {
	return "agent:url_technology_fingerprinter"
}

func (uf *URLTechnologyFingerprinter) Register(s *core.Session) error {
	err := s.EventBus.SubscribeAsync(core.URLResponsive, uf.OnURLResponsive, false)
	if err != nil {
		return err
	}

	uf.session = s

	uf.loadFingerprints()

	return nil
}

func (uf *URLTechnologyFingerprinter) loadFingerprints() {
	fingerprints, err := uf.session.Asset("static/wappalyzer_fingerprints.json")
	if err != nil {
		uf.session.Out.Fatal("Can't read technology fingerprints file\n")
		os.Exit(1)
	}

	err = json.Unmarshal(fingerprints, &uf.fingerprints)
	if err != nil {
		uf.session.Out.Fatal("unmarshal fingerprints error: %v", err)
		os.Exit(1)
	}

	for i := range uf.fingerprints {
		uf.fingerprints[i].LoadPatterns()
	}
}

func (uf *URLTechnologyFingerprinter) OnURLResponsive(url string) {
	uf.session.Out.Debug("[%s] Received new responsive URL %s\n", uf.ID(), url)
	page := uf.session.GetPage(url)
	if page == nil {
		uf.session.Out.Error("Unable to find page for URL: %s\n", url)
		return
	}

	uf.session.WaitGroup.Add()
	go func(page *core.Page) {
		defer uf.session.WaitGroup.Done()
		seen := make(map[string]struct{})
		fingerprints := append(uf.fingerprintHeaders(page), uf.fingerprintBody(page)...)
		for _, f := range fingerprints {
			if _, ok := seen[f.Name]; ok {
				continue
			}
			seen[f.Name] = struct{}{}
			page.AddTag(f.Name, "info", f.Website)
			for _, impl := range f.Implies {
				if _, ok := seen[impl.Name]; ok {
					continue
				}
				seen[impl.Name] = struct{}{}
				for _, implf := range uf.fingerprints {
					if impl.Name == implf.Name {
						page.AddTag(implf.Name, "info", implf.Website)
						break
					}
				}
			}
		}
	}(page)
}

func (uf *URLTechnologyFingerprinter) fingerprintHeaders(page *core.Page) []Fingerprint {
	var technologies []Fingerprint

	for _, header := range page.Headers {
		for _, fingerprint := range uf.fingerprints {
			for name, pattern := range fingerprint.HeaderFingerprints {
				if name != header.Name {
					continue
				}

				if pattern.Regexp.MatchString(header.Value) {
					uf.session.Out.Debug("[%s] Identified technology %s on %s from %s response header\n", uf.ID(), fingerprint.Name, page.URL, header.Name)
					technologies = append(technologies, fingerprint)
				}
			}
		}
	}

	return technologies
}

func (uf *URLTechnologyFingerprinter) fingerprintBody(page *core.Page) []Fingerprint {
	var technologies []Fingerprint
	body, err := uf.session.ReadFile(fmt.Sprintf("html/%s.html", page.BaseFilename()))
	if err != nil {
		uf.session.Out.Debug("[%s] Error reading HTML body file for %s: %s\n", uf.ID(), page.URL, err)
		return technologies
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		uf.session.Out.Debug("[%s] Error when parsing HTML body file for %s: %s\n", uf.ID(), page.URL, err)
		return technologies
	}

	strBody := string(body)
	scripts := doc.Find("script")
	meta := doc.Find("meta")

	for _, fingerprint := range uf.fingerprints {
		for _, pattern := range fingerprint.HTMLFingerprints {
			if pattern.Regexp.MatchString(strBody) {
				uf.session.Out.Debug("[%s] Identified technology %s on %s from HTML\n", uf.ID(), fingerprint.Name, page.URL)
				technologies = append(technologies, fingerprint)
			}
		}

		for _, pattern := range fingerprint.ScriptFingerprints {
			scripts.Each(func(i int, s *goquery.Selection) {
				if script, exists := s.Attr("src"); exists {
					if pattern.Regexp.MatchString(script) {
						uf.session.Out.Debug("[%s] Identified technology %s on %s from script tag\n", uf.ID(), fingerprint.Name, page.URL)
						technologies = append(technologies, fingerprint)
					}
				}
			})
		}

		for name, pattern := range fingerprint.MetaFingerprints {
			meta.Each(func(i int, s *goquery.Selection) {
				if n, _ := s.Attr("name"); n == name {
					content, _ := s.Attr("content")
					if pattern.Regexp.MatchString(content) {
						uf.session.Out.Debug("[%s] Identified technology %s on %s from meta tag\n", uf.ID(), fingerprint.Name, page.URL)
						technologies = append(technologies, fingerprint)
					}
				}
			})
		}
	}

	return technologies
}
