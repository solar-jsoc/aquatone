package agents

import (
	"fmt"
	"net"
	"strings"

	"sdg-git.solar.local/golang/aquatone/core"
)

type URLTakeoverDetector struct {
	session *core.Session
}

func NewURLTakeoverDetector() *URLTakeoverDetector {
	return &URLTakeoverDetector{}
}

func (td *URLTakeoverDetector) ID() string {
	return "agent:url_takeover_detector"
}

func (td *URLTakeoverDetector) Register(s *core.Session) error {
	err := s.EventBus.SubscribeAsync(core.URLResponsive, td.OnURLResponsive, false)
	if err != nil {
		return err
	}

	td.session = s

	return nil
}

func (td *URLTakeoverDetector) OnURLResponsive(u string) {
	td.session.Out.Debug("[%s] Received new url: %s\n", td.ID(), u)
	page := td.session.GetPage(u)
	if page == nil {
		td.session.Out.Error("Unable to find page for URL: %s\n", u)
		return
	}

	if page.IsIPHost() {
		td.session.Out.Debug("[%s] Skipping takeover detection on IP URL %s\n", td.ID(), u)
		return
	}

	td.session.WaitGroup.Add()
	go func(p *core.Page) {
		defer td.session.WaitGroup.Done()
		td.runDetectorFunctions(p)
	}(page)
}

func (td *URLTakeoverDetector) runDetectorFunctions(page *core.Page) {
	hostname := page.ParsedURL().Hostname()
	addrs, err := net.LookupHost(fmt.Sprintf("%s.", hostname))
	if err != nil {
		td.session.Out.Error("Unable to resolve %s to IP addresses: %s\n", hostname, err)
		return
	}
	cname, err := net.LookupCNAME(fmt.Sprintf("%s.", hostname))
	if err != nil {
		td.session.Out.Error("Unable to resolve %s to CNAME: %s\n", hostname, err)
		return
	}

	td.session.Out.Debug("[%s] IP addresses for %s: %v\n", td.ID(), hostname, addrs)
	td.session.Out.Debug("[%s] CNAME for %s: %s\n", td.ID(), hostname, cname)

	body, err := td.session.ReadFile(fmt.Sprintf("html/%s.html", page.BaseFilename()))
	if err != nil {
		td.session.Out.Debug("[%s] Error reading HTML body file for %s: %s\n", td.ID(), page.URL, err)
		return
	}

	if td.detectGithubPages(page, addrs, string(body)) {
		return
	}

	if td.detectAmazonS3(page, cname, string(body)) {
		return
	}

	if td.detectCampaignMonitor(page, cname, string(body)) {
		return
	}

	if td.detectCargoCollective(page, cname, string(body)) {
		return
	}

	if td.detectFeedPress(page, cname, string(body)) {
		return
	}

	if td.detectGhost(page, cname, string(body)) {
		return
	}

	if td.detectHelpjuice(page, cname, string(body)) {
		return
	}

	if td.detectHelpScout(page, cname, string(body)) {
		return
	}

	if td.detectHeroku(page, cname, string(body)) {
		return
	}

	if td.detectJetBrains(page, cname, string(body)) {
		return
	}

	if td.detectMicrosoftAzure(page, cname, string(body)) {
		return
	}

	if td.detectReadme(page, cname, string(body)) {
		return
	}

	if td.detectSurge(page, addrs, cname, string(body)) {
		return
	}

	if td.detectTumblr(page, addrs, cname, string(body)) {
		return
	}

	if td.detectUserVoice(page, cname, string(body)) {
		return
	}

	if td.detectWordpress(page, cname, string(body)) {
		return
	}

	if td.detectSmugMug(page, cname, string(body)) {
		return
	}

	if td.detectStrikingly(page, addrs, cname, string(body)) {
		return
	}

	if td.detectUptimeRobot(page, cname, string(body)) {
		return
	}

	if td.detectPantheon(page, cname, string(body)) {
		return
	}
}

func (td *URLTakeoverDetector) detectGithubPages(p *core.Page, addrs []string, body string) bool {
	githubAddrs := [...]string{"185.199.108.153", "185.199.109.153", "185.199.110.153", "185.199.111.153"}
	fingerprints := [...]string{"There isn't a GitHub Pages site here.", "For root URLs (like http://example.com/) you must provide an index.html file"}
	for _, githubAddr := range githubAddrs {
		for _, addr := range addrs {
			if addr == githubAddr {
				for _, fingerprint := range fingerprints {
					if strings.Contains(body, fingerprint) {
						p.AddTag("Domain Takeover", "danger", "https://help.github.com/articles/using-a-custom-domain-with-github-pages/")
						td.session.Out.Warn("%s: vulnerable to takeover on Github Pages\n", p.URL)
						return true
					}
				}
				return true
			}
		}
	}
	return false
}

func (td *URLTakeoverDetector) detectAmazonS3(p *core.Page, cname string, body string) bool {
	fingerprints := [...]string{"NoSuchBucket", "The specified bucket does not exist"}
	if !strings.HasSuffix(cname, ".amazonaws.com.") {
		return false
	}
	for _, fingerprint := range fingerprints {
		if strings.Contains(body, fingerprint) {
			p.AddTag("Domain Takeover", "danger", "https://docs.aws.amazon.com/AmazonS3/latest/dev/website-hosting-custom-domain-walkthrough.html")
			td.session.Out.Warn("%s: vulnerable to takeover on Amazon S3\n", p.URL)
			return true
		}
	}
	return true
}

func (td *URLTakeoverDetector) detectCampaignMonitor(p *core.Page, cname string, body string) bool {
	if cname != "cname.createsend.com." {
		return false
	}
	p.AddTag("Campaign Monitor", "info", "https://www.campaignmonitor.com/")
	if strings.Contains(body, "Double check the URL or ") {
		p.AddTag("Domain Takeover", "danger", "https://help.campaignmonitor.com/custom-domain-names")
		td.session.Out.Warn("%s: vulnerable to takeover on Campaign Monitor\n", p.URL)
		return true
	}
	return true
}

func (td *URLTakeoverDetector) detectCargoCollective(p *core.Page, cname string, body string) bool {
	if cname != "subdomain.cargocollective.com." {
		return false
	}
	p.AddTag("Cargo Collective", "info", "https://cargocollective.com/")
	if strings.Contains(body, "404 Not Found") {
		p.AddTag("Domain Takeover", "danger", "https://support.2.cargocollective.com/Using-a-Third-Party-Domain")
		td.session.Out.Warn("%s: vulnerable to takeover on Cargo Collective\n", p.URL)
		return true
	}
	return true
}

func (td *URLTakeoverDetector) detectFeedPress(p *core.Page, cname string, body string) bool {
	if cname != "redirect.feedpress.me." {
		return false
	}
	p.AddTag("FeedPress", "info", "https://feed.press/")
	if strings.Contains(body, "The feed has not been found.") {
		p.AddTag("Domain Takeover", "danger", "https://support.feed.press/article/61-how-to-create-a-custom-hostname")
		td.session.Out.Warn("%s: vulnerable to takeover on FeedPress\n", p.URL)
		return true
	}
	return true
}

func (td *URLTakeoverDetector) detectGhost(p *core.Page, cname string, body string) bool {
	if !strings.HasSuffix(cname, ".ghost.io.") {
		return false
	}
	if strings.Contains(body, "The thing you were looking for is no longer here, or never was") {
		p.AddTag("Domain Takeover", "danger", "https://docs.ghost.org/faq/using-custom-domains/")
		td.session.Out.Warn("%s: vulnerable to takeover on Ghost\n", p.URL)
		return true
	}
	return true
}

func (td *URLTakeoverDetector) detectHelpjuice(p *core.Page, cname string, body string) bool {
	if !strings.HasSuffix(cname, ".helpjuice.com.") {
		return false
	}
	p.AddTag("Helpjuice", "info", "https://helpjuice.com/")
	if strings.Contains(body, "We could not find what you're looking for.") {
		p.AddTag("Domain Takeover", "danger", "https://help.helpjuice.com/34339-getting-started/custom-domain")
		td.session.Out.Warn("%s: vulnerable to takeover on Helpjuice\n", p.URL)
		return true
	}
	return false
}

func (td *URLTakeoverDetector) detectHelpScout(p *core.Page, cname string, body string) bool {
	if !strings.HasSuffix(cname, ".helpscoutdocs.com.") {
		return false
	}
	p.AddTag("HelpScout", "info", "https://www.helpscout.net/")
	if strings.Contains(body, "No settings were found for this company:") {
		p.AddTag("Domain Takeover", "danger", "https://docs.helpscout.net/article/42-setup-custom-domain")
		td.session.Out.Warn("%s: vulnerable to takeover on HelpScout\n", p.URL)
		return true
	}
	return true
}

func (td *URLTakeoverDetector) detectHeroku(p *core.Page, cname string, body string) bool {
	herokuCnames := [...]string{".herokudns.com.", ".herokuapp.com.", ".herokussl.com."}
	for _, herokuCname := range herokuCnames {
		if strings.HasSuffix(cname, herokuCname) {
			p.AddTag("Heroku", "info", "https://www.heroku.com/")
			if strings.Contains(body, "No such app") {
				p.AddTag("Domain Takeover", "danger", "https://devcenter.heroku.com/articles/custom-domains")
				td.session.Out.Warn("%s: vulnerable to takeover on Heroku\n", p.URL)
				return true
			}
			return true
		}
	}
	return false
}

func (td *URLTakeoverDetector) detectJetBrains(p *core.Page, cname string, body string) bool {
	if !strings.HasSuffix(cname, ".myjetbrains.com.") {
		return false
	}
	p.AddTag("JetBrains", "info", "https://www.jetbrains.com/")
	if strings.Contains(body, "is not a registered InCloud YouTrack") {
		p.AddTag("Domain Takeover", "danger", "https://www.jetbrains.com/help/youtrack/incloud/Domain-Settings.html#use-custom-domain-name")
		td.session.Out.Warn("%s: vulnerable to takeover on JetBrains\n", p.URL)
		return true
	}
	return true
}

func (td *URLTakeoverDetector) detectMicrosoftAzure(p *core.Page, cname string, body string) bool {
	if !strings.HasSuffix(cname, ".azurewebsites.net.") {
		return false
	}
	p.AddTag("Microsoft Azure", "info", "https://azure.microsoft.com/")
	if strings.Contains(body, "404 Web Site not found") {
		p.AddTag("Domain Takeover", "danger", "https://docs.microsoft.com/en-us/azure/app-service/app-service-web-tutorial-custom-domain")
		td.session.Out.Warn("%s: vulnerable to takeover on Microsoft Azure\n", p.URL)
		return true
	}
	return true
}

func (td *URLTakeoverDetector) detectReadme(p *core.Page, cname string, body string) bool {
	readmeCnames := [...]string{".readme.io.", ".readmessl.com."}
	for _, readmeCname := range readmeCnames {
		if strings.HasSuffix(cname, readmeCname) {
			p.AddTag("Readme", "info", "https://readme.io/")
			if strings.Contains(body, "Project doesnt exist... yet!") {
				p.AddTag("Domain Takeover", "danger", "https://readme.readme.io/docs/setting-up-custom-domain")
				td.session.Out.Warn("%s: vulnerable to takeover on Readme\n", p.URL)
				return true
			}
			return true
		}
	}
	return false
}

func (td *URLTakeoverDetector) detectSurge(p *core.Page, addrs []string, cname string, body string) bool {
	detected := false
	for _, addr := range addrs {
		if addr == "45.55.110.124" {
			detected = true
			break
		}
	}
	if cname == "na-west1.surge.sh." {
		detected = true
	}
	if detected {
		p.AddTag("Surge", "info", "https://surge.sh/")
		if strings.Contains(body, "project not found") {
			p.AddTag("Domain Takeover", "danger", "https://surge.sh/help/adding-a-custom-domain")
			td.session.Out.Warn("%s: vulnerable to takeover on Surge\n", p.URL)
		}
		return true
	}
	return false
}

func (td *URLTakeoverDetector) detectTumblr(p *core.Page, addrs []string, cname string, body string) bool {
	detected := false
	for _, addr := range addrs {
		if addr == "66.6.44.4" {
			detected = true
			break
		}
	}
	if cname == "domains.tumblr.com." {
		detected = true
	}
	if detected {
		if strings.Contains(body, "Whatever you were looking for doesn't currently exist at this address") {
			p.AddTag("Domain Takeover", "danger", "https://tumblr.zendesk.com/hc/en-us/articles/231256548-Custom-domains")
			td.session.Out.Warn("%s: vulnerable to takeover on Tumblr\n", p.URL)
		}
		return true
	}
	return false
}

func (td *URLTakeoverDetector) detectUserVoice(p *core.Page, cname string, body string) bool {
	if !strings.HasSuffix(cname, ".uservoice.com.") {
		return false
	}
	p.AddTag("UserVoice", "info", "https://www.uservoice.com/")
	if strings.Contains(body, "This UserVoice subdomain is currently available!") {
		p.AddTag("Domain Takeover", "danger", "https://developer.uservoice.com/docs/site/domain-aliasing/")
		td.session.Out.Warn("%s: vulnerable to takeover on UserVoice\n", p.URL)
	}
	return true
}

func (td *URLTakeoverDetector) detectWordpress(p *core.Page, cname string, body string) bool {
	if !strings.HasSuffix(cname, ".wordpress.com.") {
		return false
	}
	if strings.Contains(body, "Do you want to register") {
		p.AddTag("Domain Takeover", "danger", "https://en.support.wordpress.com/domains/map-subdomain/")
		td.session.Out.Warn("%s: vulnerable to takeover on Wordpress\n", p.URL)
	}
	return true
}

func (td *URLTakeoverDetector) detectSmugMug(p *core.Page, cname string, body string) bool {
	if cname != "domains.smugmug.com." {
		return false
	}
	p.AddTag("SmugMug", "info", "https://www.smugmug.com/")
	if body == "" {
		p.AddTag("Domain Takeover", "danger", "https://help.smugmug.com/use-a-custom-domain-BymMexwJVHG")
		td.session.Out.Warn("%s: vulnerable to takeover on SmugMug\n", p.URL)
	}
	return true
}

func (td *URLTakeoverDetector) detectStrikingly(p *core.Page, addrs []string, cname string, body string) bool {
	detected := false
	for _, addr := range addrs {
		if addr == "54.183.102.22" {
			detected = true
			break
		}
	}
	if strings.HasSuffix(cname, ".s.strikinglydns.com.") {
		detected = true
	}
	if detected {
		p.AddTag("Strikingly", "info", "https://www.strikingly.com/")
		if strings.Contains(body, "But if you're looking to build your own website,") {
			p.AddTag("Domain Takeover", "danger", "https://support.strikingly.com/hc/en-us/articles/215046947-Connect-Custom-Domain")
			td.session.Out.Warn("%s: vulnerable to takeover on Strikingly\n", p.URL)
		}
		return true
	}
	return false
}

func (td *URLTakeoverDetector) detectUptimeRobot(p *core.Page, cname string, body string) bool {
	if cname != "stats.uptimerobot.com." {
		return false
	}
	p.AddTag("UptimeRobot", "info", "https://uptimerobot.com/")
	if strings.Contains(body, "This public status page <b>does not seem to exist</b>.") {
		p.AddTag("Domain Takeover", "danger", "https://blog.uptimerobot.com/introducing-public-status-pages-yay/")
		td.session.Out.Warn("%s: vulnerable to takeover on UptimeRobot\n", p.URL)
	}
	return true
}

func (td *URLTakeoverDetector) detectPantheon(p *core.Page, cname string, body string) bool {
	if !strings.HasSuffix(cname, ".pantheonsite.io.") {
		return false
	}
	p.AddTag("Pantheon", "info", "https://pantheon.io/")
	if strings.Contains(body, "The gods are wise") {
		p.AddTag("Domain Takeover", "danger", "https://pantheon.io/docs/domains/")
		td.session.Out.Warn("%s: vulnerable to takeover on Pantheon\n", p.URL)
	}
	return true
}
