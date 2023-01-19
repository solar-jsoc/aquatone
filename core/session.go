package core

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asaskevich/EventBus"
	"github.com/remeh/sizedwaitgroup"
)

type Stats struct {
	StartedAt            time.Time `json:"startedAt"`
	FinishedAt           time.Time `json:"finishedAt"`
	PortOpen             uint32    `json:"portOpen"`
	PortClosed           uint32    `json:"portClosed"`
	RequestSuccessful    uint32    `json:"requestSuccessful"`
	RequestFailed        uint32    `json:"requestFailed"`
	ResponseCode2xx      uint32    `json:"responseCode2xx"`
	ResponseCode3xx      uint32    `json:"responseCode3xx"`
	ResponseCode4xx      uint32    `json:"responseCode4xx"`
	ResponseCode5xx      uint32    `json:"responseCode5xx"`
	ScreenshotSuccessful uint32    `json:"screenshotSuccessful"`
	ScreenshotFailed     uint32    `json:"screenshotFailed"`
}

func (s *Stats) Duration() time.Duration {
	return s.FinishedAt.Sub(s.StartedAt)
}

func (s *Stats) IncrementPortOpen() {
	atomic.AddUint32(&s.PortOpen, 1)
}

func (s *Stats) IncrementPortClosed() {
	atomic.AddUint32(&s.PortClosed, 1)
}

func (s *Stats) IncrementRequestSuccessful() {
	atomic.AddUint32(&s.RequestSuccessful, 1)
}

func (s *Stats) IncrementRequestFailed() {
	atomic.AddUint32(&s.RequestFailed, 1)
}

func (s *Stats) IncrementResponseCode2xx() {
	atomic.AddUint32(&s.ResponseCode2xx, 1)
}

func (s *Stats) IncrementResponseCode3xx() {
	atomic.AddUint32(&s.ResponseCode3xx, 1)
}

func (s *Stats) IncrementResponseCode4xx() {
	atomic.AddUint32(&s.ResponseCode4xx, 1)
}

func (s *Stats) IncrementResponseCode5xx() {
	atomic.AddUint32(&s.ResponseCode5xx, 1)
}

func (s *Stats) IncrementScreenshotSuccessful() {
	atomic.AddUint32(&s.ScreenshotSuccessful, 1)
}

func (s *Stats) IncrementScreenshotFailed() {
	atomic.AddUint32(&s.ScreenshotFailed, 1)
}

type Session struct {
	sync.Mutex
	Version                string                        `json:"version"`
	Options                Options                       `json:"-"`
	Out                    *Logger                       `json:"-"`
	Stats                  *Stats                        `json:"stats"`
	Pages                  map[string]*Page              `json:"pages"`
	PageSimilarityClusters map[string][]string           `json:"pageSimilarityClusters"`
	Ports                  []int                         `json:"-"`
	EventBus               EventBus.Bus                  `json:"-"`
	WaitGroup              sizedwaitgroup.SizedWaitGroup `json:"-"`
	OutFile                *os.File                      `json:"-"`
}

func (s *Session) Start() {
	s.Pages = make(map[string]*Page)
	s.PageSimilarityClusters = make(map[string][]string)
	s.initStats()
	s.initLogger()
	s.initPorts()
	s.initThreads()
	s.initEventBus()
	s.initWaitGroup()
	s.initDirectories()
}

func (s *Session) End() {
	s.Stats.FinishedAt = time.Now()
}

func (s *Session) Close() {
	_ = s.OutFile.Close()
}

func (s *Session) AddPage(url string) (*Page, error) {
	s.Lock()
	defer s.Unlock()
	if page, ok := s.Pages[url]; ok {
		return page, nil
	}

	page, err := NewPage(url)
	if err != nil {
		return nil, err
	}

	s.Pages[url] = page
	return page, nil
}

func (s *Session) GetPage(url string) *Page {
	if page, ok := s.Pages[url]; ok {
		return page
	}
	return nil
}

func (s *Session) GetPageByUUID(id string) *Page {
	for _, page := range s.Pages {
		if page.UUID == id {
			return page
		}
	}
	return nil
}

func (s *Session) initPaths() error {
	if s.Options.OutDir == nil {
		return fmt.Errorf("output destination must be set")
	}

	fi, err := os.Stat(*s.Options.OutDir)

	if os.IsNotExist(err) {
		return fmt.Errorf("output destination %s does not exist", *s.Options.OutDir)
	}

	if !fi.IsDir() {
		return fmt.Errorf("output destination must be a directory")
	}

	return nil
}

func (s *Session) initStats() {
	if s.Stats != nil {
		return
	}
	s.Stats = &Stats{
		StartedAt: time.Now(),
	}
}

func (s *Session) initPorts() {
	var ports []int
	switch *s.Options.Ports {
	case "small":
		ports = SmallPortList
	case "", "medium", "default":
		ports = MediumPortList
	case "large":
		ports = LargePortList
	case "xlarge", "huge":
		ports = XLargePortList
	default:
		for _, p := range strings.Split(*s.Options.Ports, ",") {
			port, err := strconv.Atoi(strings.TrimSpace(p))
			if err != nil {
				s.Out.Fatal("Invalid port range given\n")
				os.Exit(1)
			}
			if port < 1 || port > 65535 {
				s.Out.Fatal("Invalid port given: %v\n", port)
				os.Exit(1)
			}
			ports = append(ports, port)
		}
	}
	s.Ports = ports
}

func (s *Session) initLogger() {
	var writer io.Writer
	var noColor bool

	if s.Options.OutFile != nil {
		outFilePath := filepath.Join(*s.Options.OutDir, *s.Options.OutFile)
		file, err := os.OpenFile(outFilePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			writer = file
			noColor = true
		}
	}

	s.Out = NewLogger(writer, *s.Options.Debug, *s.Options.Silent, noColor)
}

func (s *Session) initThreads() {
	if *s.Options.Threads == 0 {
		numCPUs := runtime.NumCPU()
		s.Options.Threads = &numCPUs
	}
}

func (s *Session) initEventBus() {
	s.EventBus = EventBus.New()
}

func (s *Session) initWaitGroup() {
	s.WaitGroup = sizedwaitgroup.New(*s.Options.Threads)
}

func (s *Session) initDirectories() {
	for _, d := range []string{"headers", "html", "screenshots"} {
		d = s.GetFilePath(d)
		if _, err := os.Stat(d); os.IsNotExist(err) {
			err = os.MkdirAll(d, 0755)
			if err != nil {
				s.Out.Fatal("Failed to create required directory %s\n", d)
				os.Exit(1)
			}
		}
	}
}

func (s *Session) BaseFilenameFromURL(str string) string {
	u, err := url.Parse(str)
	if err != nil {
		return ""
	}

	h := sha1.New()
	io.WriteString(h, u.Path)
	io.WriteString(h, u.Fragment)

	pathHash := fmt.Sprintf("%x", h.Sum(nil))[0:16]
	host := strings.Replace(u.Host, ":", "__", 1)
	filename := fmt.Sprintf("%s__%s__%s", u.Scheme, strings.Replace(host, ".", "_", -1), pathHash)

	return strings.ToLower(filename)
}

func (s *Session) GetFilePath(p string) string {
	return path.Join(*s.Options.OutDir, p)
}

func (s *Session) ReadFile(p string) ([]byte, error) {
	content, err := os.ReadFile(s.GetFilePath(p))
	if err != nil {
		return content, err
	}
	return content, nil
}

func (s *Session) ToJSON() string {
	sessionJSON, _ := json.Marshal(s)
	return string(sessionJSON)
}

func (s *Session) SaveToFile(filename string) error {
	filePath := s.GetFilePath(filename)

	err := os.WriteFile(filePath, []byte(s.ToJSON()), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (s *Session) Asset(name string) ([]byte, error) {
	return Asset(name)
}

func NewSession() (*Session, error) {
	var err error
	var session Session

	session.Version = Version

	if session.Options, err = ParseOptions(); err != nil {
		return nil, err
	}

	if err = session.initPaths(); err != nil {
		return nil, err
	}

	if *session.Options.ChromePath != "" {
		if _, err := os.Stat(*session.Options.ChromePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("Chrome path %s does not exist", *session.Options.ChromePath)
		}
	}

	if *session.Options.SessionPath != "" {
		if _, err := os.Stat(*session.Options.SessionPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("Session path %s does not exist", *session.Options.SessionPath)
		}
	}

	if *session.Options.TemplatePath != "" {
		if _, err := os.Stat(*session.Options.TemplatePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("Template path %s does not exist", *session.Options.TemplatePath)
		}
	}

	envOutPath := os.Getenv("AQUATONE_OUT_PATH")
	if *session.Options.OutDir == "." && envOutPath != "" {
		session.Options.OutDir = &envOutPath
	}

	outdir := filepath.Clean(*session.Options.OutDir)
	session.Options.OutDir = &outdir

	session.Version = Version
	session.Start()

	return &session, nil
}

func (s *Session) IsFileSaved(file string, timeout time.Duration) bool {
	timeLimit := time.NewTimer(timeout)
	_, err := os.Open(file)
	for err != nil && os.IsNotExist(err) {
		select {
		case <-timeLimit.C:
			return false
		default:
			time.Sleep(100 * time.Millisecond)
			_, err = os.Open(file)
		}
	}
	return err == nil
}

func (s *Session) Tar() error {
	tarName := "report.tar.gz"
	tarPath := filepath.Join(os.TempDir(), tarName)

	err := tarIt(*s.Options.OutDir, tarPath)
	if err != nil {
		return err
	}

	dst, err := os.Create(s.GetFilePath(tarName))
	if err != nil {
		return err
	}

	src, err := os.Open(tarPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	return os.Remove(tarPath)
}

func tarIt(source, target string) error {
	tarFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	gz := gzip.NewWriter(tarFile)
	defer gz.Close()

	gz.Name = filepath.Base(target)

	tarball := tar.NewWriter(gz)
	defer tarball.Close()

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if path == source {
				return nil
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			header.Name = strings.TrimPrefix(path, source)

			if err = tarball.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarball, file)
			return err
		})
}
