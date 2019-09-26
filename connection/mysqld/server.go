package mysqld

import (
	"bufio"
	"database/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

// logLineRgx matches e.g. "2019-09-23 13:39:55 0 [Note] InnoDB: Using Linux native AIO".
var logLineRgx = regexp.MustCompile(`\A(\S+ \S+) (\d+) \[(\w+)] (.+)\z`)

// logLevels maps the MySQLd log levels (as in log messages) to logrus log levels.
var logLevels = map[string]log.Level{
	"Note":    log.InfoLevel,
	"Warning": log.WarnLevel,
	"ERROR":   log.ErrorLevel,
}

// Server represents a managed MySQL server.
type Server struct {
	// basedir is a directory containing the MySQL server context.
	basedir string
	// cmd represents the main MySQLd process if any.
	cmd *exec.Cmd
	// stopped is closed as soon as the main MySQLd process is stopped.
	stopped chan struct{}
	// errorLogEof is closed on the main MySQLd process' error log EOF.
	errorLogEof chan struct{}
	// logPipeCloser is the result of opening the log pipe for writing to ensure our reader's termination.
	logPipeCloser io.Closer
}

// Start starts *s and returns the host to connect to.
func (s *Server) Start() (string, error) {
	me, errUC := user.Current()
	if errUC != nil {
		return "", errUC
	}

	{
		var errTD error
		s.basedir, errTD = ioutil.TempDir("", "")

		if errTD != nil {
			return "", errTD
		}
	}

	log.WithFields(log.Fields{"basedir": s.basedir}).Info("starting MySQL server")

	socket := path.Join(s.basedir, "socket")
	host := fmt.Sprintf("unix(%s)", socket)

	db, errOpen := sql.Open("mysql", fmt.Sprintf("root@%s/", host))
	if errOpen != nil {
		os.RemoveAll(s.basedir)
		s.basedir = ""
		return "", errOpen
	}

	defer db.Close()

	dataDir := path.Join(s.basedir, "data")
	if errMkdir := os.Mkdir(dataDir, 0700); errMkdir != nil {
		os.RemoveAll(s.basedir)
		s.basedir = ""
		return "", errMkdir
	}

	logPipe := path.Join(s.basedir, "log")
	if errMkfifo := syscall.Mkfifo(logPipe, 0700); errMkfifo != nil {
		os.RemoveAll(s.basedir)
		s.basedir = ""
		return "", errMkfifo
	}

	params := []string{
		"--no-defaults",
		"--user=" + me.Username,
		"--pid-file=" + path.Join(s.basedir, "pid"),
		"--socket=" + socket,
		"--datadir=" + dataDir,
		"--tmpdir=/tmp",
		"--skip-networking",
		"--query_cache_size=16M",
		"--expire_logs_days=10",
		"--character-set-server=utf8mb4",
		"--collation-server=utf8mb4_general_ci",
	}

	{
		cmd := exec.Command("mysql_install_db")

		realPath, errRL := os.Readlink(cmd.Path)
		if errRL != nil {
			realPath = cmd.Path
		}

		if !path.IsAbs(realPath) {
			realPath = path.Join(path.Dir(cmd.Path), realPath)
		}

		basedir := path.Dir(path.Dir(realPath))
		params = append(params, "--basedir="+basedir, "--lc-messages-dir="+path.Join(basedir, "share/mysql/english"))

		cmd.Args = append(params, "--log_error="+path.Join(s.basedir, "install"))
		cmd.Dir = s.basedir

		if errRun := cmd.Run(); errRun != nil {
			os.RemoveAll(s.basedir)
			s.basedir = ""
			return "", errRun
		}
	}

	s.errorLogEof = make(chan struct{})
	go s.file2log(logPipe)

	logPipeWriter, errCr := os.Create(logPipe)
	if errCr != nil {
		os.RemoveAll(s.basedir)
		s.basedir = ""
		return "", errCr
	}

	cmd := exec.Command("mysqld", append(params, "--log_error="+logPipe)...)
	cmd.Dir = s.basedir

	stderr, errStderr := cmd.StderrPipe()
	if errStderr != nil {
		logPipeWriter.Close()

		os.RemoveAll(s.basedir)
		s.basedir = ""

		return "", errStderr
	}

	if errStart := cmd.Start(); errStart != nil {
		logPipeWriter.Close()

		os.RemoveAll(s.basedir)
		s.basedir = ""

		return "", errStart
	}

	s.cmd = cmd
	s.stopped = make(chan struct{})
	s.logPipeCloser = logPipeWriter

	go s.stderr2log(stderr)

	log.WithFields(log.Fields{"basedir": s.basedir}).Debug("checking the MySQL server for actual serving")

	for {
		errPing := db.Ping()
		if errPing == nil {
			log.WithFields(log.Fields{"basedir": s.basedir}).Debug("MySQL server is actually serving now")
			return host, nil
		}

		select {
		case <-s.stopped:
			return "", errPing

		default:
			log.WithFields(log.Fields{
				"basedir": s.basedir,
				"error":   errPing,
			}).Debug("MySQL server isn't actually serving, yet")

			time.Sleep(time.Second)
		}
	}
}

// Stop stops *s.
func (s *Server) Stop() error {
	log.Info("stopping MySQL server")

	if errSignal := s.cmd.Process.Signal(syscall.SIGTERM); errSignal != nil {
		return errSignal
	}

	<-s.stopped
	return nil
}

// file2log forwards the MySQL server's log from path to logrus.
func (s *Server) file2log(path string) {
	defer close(s.errorLogEof)

	stream, errOpen := os.Open(path)
	if errOpen != nil {
		log.WithFields(log.Fields{"source": "log file", "error": errOpen}).Error(
			"got unexpected error while forwarding MySQL server logs",
		)
		return
	}

	defer stream.Close()

	stream2log(stream, "log file")
}

// stderr2log forwards the MySQL server's log from stderr to logrus and cleans up *s.
func (s *Server) stderr2log(stderr io.Reader) {
	stream2log(stderr, "stderr")

	if errWait := s.cmd.Wait(); errWait != nil {
		log.WithFields(log.Fields{"error": errWait}).Error("MySQL server terminated with an error")
	}

	s.logPipeCloser.Close()
	<-s.errorLogEof

	os.RemoveAll(s.basedir)
	s.basedir = ""

	close(s.stopped)
}

// stream2log forwards the MySQL server's log from stream to logrus.
func stream2log(stream io.Reader, source string) {
	buffer := bufio.NewReader(stream)

	for {
		line, errRead := buffer.ReadBytes('\n')
		if errRead != nil {
			if errRead != io.EOF || len(line) > 0 {
				log.WithFields(log.Fields{"source": source, "error": errRead}).Error(
					"got unexpected error while forwarding MySQL server logs",
				)
			}

			break
		}

		line = line[:len(line)-1]

		if len(line) > 0 {
			if submatch := logLineRgx.FindSubmatch(line); submatch != nil {
				timeStamp, errTime := time.ParseInLocation("2006-01-02 15:04:05", string(submatch[1]), time.Local)
				if errTime != nil {
					timeStamp = time.Now()
				}

				thread, errPU := strconv.ParseUint(string(submatch[2]), 10, 64)
				if errPU != nil {
					thread = 0
				}

				log.WithTime(timeStamp).WithFields(log.Fields{
					"component": "mysqld",
					"source":    source,
					"thread":    thread,
				}).Log(logLevels[string(submatch[3])], string(submatch[4]))
			}
		}
	}
}
