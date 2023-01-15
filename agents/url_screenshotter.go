package agents

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sdg-git.solar.local/golang/aquatone/core"
)

type URLScreenshotter struct {
	session         *core.Session
	chromePath      string
	tempUserDirPath string
}

func NewURLScreenshotter() *URLScreenshotter {
	return &URLScreenshotter{}
}

func (us *URLScreenshotter) ID() string {
	return "agent:url_screenshotter"
}

func (us *URLScreenshotter) Register(s *core.Session) error {
	err := s.EventBus.SubscribeAsync(core.URLResponsive, us.OnURLResponsive, false)
	if err != nil {
		return err
	}

	err = s.EventBus.SubscribeAsync(core.SessionEnd, us.OnSessionEnd, false)
	if err != nil {
		return err
	}

	us.session = s

	us.createTempUserDir()
	us.locateChrome()

	return nil
}

func (us *URLScreenshotter) OnURLResponsive(url string) {
	us.session.Out.Debug("[%s] Received new responsive URL %s\n", us.ID(), url)
	page := us.session.GetPage(url)
	if page == nil {
		us.session.Out.Error("Unable to find page for URL: %s\n", url)
		return
	}

	us.session.WaitGroup.Add()
	go func(page *core.Page) {
		defer us.session.WaitGroup.Done()
		us.screenshotPage(page)
	}(page)
}

func (us *URLScreenshotter) OnSessionEnd() {
	us.session.Out.Debug("[%s] Received SessionEnd event\n", us.ID())
	_ = os.RemoveAll(us.tempUserDirPath)
	us.session.Out.Debug("[%s] Deleted temporary user directory at: %s\n", us.ID(), us.tempUserDirPath)
}

func (us *URLScreenshotter) createTempUserDir() {
	dir, err := os.MkdirTemp("", "aquatone-chrome")
	if err != nil {
		us.session.Out.Fatal("Unable to create temporary user directory for Chrome/Chromium browser\n")
		os.Exit(1)
	}

	us.session.Out.Debug("[%s] Created temporary user directory at: %s\n", us.ID(), dir)
	us.tempUserDirPath = dir
}

func (us *URLScreenshotter) locateChrome() {
	if *us.session.Options.ChromePath != "" {
		us.chromePath = *us.session.Options.ChromePath
		return
	}

	paths := []string{
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-beta",
		"/usr/bin/google-chrome-unstable",
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"C:/Program Files (x86)/Google/Chrome/Application/chrome.exe",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		us.chromePath = path
	}

	if us.chromePath == "" {
		us.session.Out.Fatal("Unable to locate a valid installation of Chrome. Install Google Chrome or try specifying a valid location with the -chrome-path option.\n")
		os.Exit(1)
	}

	if strings.Contains(strings.ToLower(us.chromePath), "chrome") {
		us.session.Out.Warn("Using unreliable Google Chrome for screenshots. Install Chromium for better results.\n\n")
	} else {
		out, err := exec.Command(us.chromePath, "--version").Output()
		if err != nil {
			us.session.Out.Warn("An error occurred while trying to determine version of Chromium.\n\n")
			return
		}
		version := string(out)
		re := regexp.MustCompile(`(\d+)\.`)
		match := re.FindStringSubmatch(version)
		if len(match) <= 0 {
			us.session.Out.Warn("Unable to determine version of Chromium. Screenshotting might be unreliable.\n\n")
			return
		}
		majorVersion, _ := strconv.Atoi(match[1])
		if majorVersion < 72 {
			us.session.Out.Warn("An older version of Chromium is installed. Screenshotting of HTTPS URLs might be unreliable.\n\n")
		}
	}

	us.session.Out.Debug("[%s] Located Chrome/Chromium binary at %s\n", us.ID(), us.chromePath)
}

func (us *URLScreenshotter) screenshotPage(page *core.Page) {
	filePath := fmt.Sprintf("screenshots/%s.png", page.BaseFilename())
	var chromeArguments = []string{
		"--headless", "--disable-gpu", "--hide-scrollbars", "--mute-audio", "--disable-notifications",
		"--no-first-run", "--disable-crash-reporter", "--ignore-certificate-errors", "--incognito",
		"--disable-infobars", "--disable-sync", "--no-default-browser-check",
		"--user-data-dir=" + us.tempUserDirPath,
		"--user-agent=" + RandomUserAgent(),
		"--window-size=" + *us.session.Options.Resolution,
		"--screenshot=" + us.session.GetFilePath(filePath),
	}

	if os.Geteuid() == 0 {
		chromeArguments = append(chromeArguments, "--no-sandbox")
	}

	if *us.session.Options.Proxy != "" {
		chromeArguments = append(chromeArguments, "--proxy-server="+*us.session.Options.Proxy)
	}

	chromeArguments = append(chromeArguments, page.URL)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*us.session.Options.ScreenshotTimeout)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, us.chromePath, chromeArguments...)
	defer us.killChromeProcessIfRunning(cmd)

	if err := cmd.Start(); err != nil {
		us.session.Out.Debug("[%s] Error: %v\n", us.ID(), err)
		us.session.Stats.IncrementScreenshotFailed()
		us.session.Out.Error("%s: screenshot failed: %s\n", page.URL, err)
		return
	}

	if err := cmd.Wait(); err != nil {
		us.session.Stats.IncrementScreenshotFailed()
		us.session.Out.Debug("[%s] Error: %v\n", us.ID(), err)
		if ctx.Err() == context.DeadlineExceeded {
			us.session.Out.Error("%s: screenshot timed out\n", page.URL)
		} else {
			us.session.Out.Error("%s: screenshot failed: %s\n", page.URL, err)
		}
		return
	}

	us.session.Stats.IncrementScreenshotSuccessful()
	us.session.Out.Info("%s: %s\n", page.URL, us.session.Out.Green("screenshot successful"))
	page.ScreenshotPath = filePath
	page.HasScreenshot = true

	saved := us.session.IsFileSaved(us.session.GetFilePath(filePath), time.Duration(*us.session.Options.ScreenshotTimeout)*time.Millisecond)
	if !saved {
		us.session.Out.Error("Error: file %q not saved\n", filePath)
	}

	return
}

func (us *URLScreenshotter) killChromeProcessIfRunning(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	_ = cmd.Process.Release()
	_ = cmd.Process.Kill()
}
