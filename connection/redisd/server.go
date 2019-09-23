package redisd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"syscall"
	"time"
)

// logLineRgx matches e.g. "1:M 23 Sep 2019 10:02:05.882 * Ready to accept connections".
var logLineRgx = regexp.MustCompile(`\A(\d+):([CM]) (.*?) ([-*#]) (.+)\z`)

// roles contains the Redis processes' roles' descriptions by their indicator characters.
var roles = map[byte]string{
	'C': "RDB writer",
	'M': "master",
}

// logLevels maps the Redis log levels (as in log messages) to logrus log levels.
var logLevels = map[byte]log.Level{
	'-': log.DebugLevel,
	'*': log.InfoLevel,
	'#': log.WarnLevel,
}

// Server represents a managed Redis server.
type Server struct {
	// basedir is a directory containing the Redis server context.
	basedir string
	// cmd represents the main Redis process if any.
	cmd *exec.Cmd
	// stopped is closed as soon as the main Redis process is stopped.
	stopped chan struct{}
}

// redisVersion matches e.g. "redis_version:5.0.5".
var redisVersion = regexp.MustCompile(`(?m)^redis_version:(.*?)\s*$`)

// redisVersionNumber matches e.g. "5".
var redisVersionNumber = regexp.MustCompile(`\d+`)

// Start starts *s and returns a client for connecting to it.
func (s *Server) Start() (*redis.Client, error) {
	{
		var errTD error
		s.basedir, errTD = ioutil.TempDir("", "")

		if errTD != nil {
			return nil, errTD
		}
	}

	log.WithFields(log.Fields{"basedir": s.basedir}).Info("starting Redis server")

	workDir := path.Join(s.basedir, "work-dir")

	if errMkdir := os.Mkdir(workDir, 0700); errMkdir != nil {
		os.RemoveAll(s.basedir)
		s.basedir = ""
		return nil, errMkdir
	}

	config := &bytes.Buffer{}
	socket := path.Join(s.basedir, "socket")
	fmt.Fprintf(config, configTemplate, configLogLevels[log.GetLevel()], workDir, socket)

	cmd := exec.Command("redis-server", "-")
	cmd.Dir = s.basedir
	cmd.Stdin = config

	stdout, errStdout := cmd.StdoutPipe()
	if errStdout != nil {
		os.RemoveAll(s.basedir)
		s.basedir = ""
		return nil, errStdout
	}

	if errStart := cmd.Start(); errStart != nil {
		os.RemoveAll(s.basedir)
		s.basedir = ""
		return nil, errStart
	}

	s.cmd = cmd
	s.stopped = make(chan struct{})

	go s.stdout2log(stdout)

	log.WithFields(log.Fields{"basedir": s.basedir}).Debug("checking the Redis server for actual serving")

	client := redis.NewClient(&redis.Options{
		Network:      "unix",
		Addr:         socket,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
	})

	for {
		info, errInfo := client.Info().Result()
		if errInfo == nil {
			version := redisVersion.FindStringSubmatch(info)
			if version == nil {
				client.Close()
				s.Stop()
				return nil, errors.New("Redis server didn't tell its version")
			}

			major := redisVersionNumber.FindString(version[1])
			if major == "" {
				client.Close()
				s.Stop()
				return nil, errors.New("bad Redis server version: " + version[1])
			}

			majorInt, errMI := strconv.ParseUint(major, 10, 64)
			if errMI != nil {
				majorInt = ^uint64(0)
			}

			if majorInt < 5 {
				client.Close()
				s.Stop()
				return nil, errors.New(fmt.Sprintf("Redis server is too old (%s < 5)", version[1]))
			}

			log.WithFields(log.Fields{
				"basedir": s.basedir,
				"version": version[1],
			}).Debug("Redis server is actually serving now")
			return client, nil
		}

		select {
		case <-s.stopped:
			client.Close()
			return nil, errInfo

		default:
			log.WithFields(log.Fields{
				"basedir": s.basedir,
				"error":   errInfo,
			}).Debug("Redis server isn't actually serving, yet")

			time.Sleep(time.Second)
		}
	}
}

// Stop stops *s.
func (s *Server) Stop() error {
	log.Info("stopping Redis server")

	proc := s.cmd.Process

	if errSignal := proc.Signal(syscall.SIGTERM); errSignal != nil {
		return errSignal
	}

	<-s.stopped
	return nil
}

// stdout2log forwards the Redis server's log from stdout to logrus and manages s.basedir and s.stopped.
func (s *Server) stdout2log(stdout io.Reader) {
	buffer := bufio.NewReader(stdout)

	for {
		line, errRead := buffer.ReadBytes('\n')
		if errRead != nil {
			if errRead != io.EOF || len(line) > 0 {
				log.WithFields(log.Fields{"error": errRead}).Error(
					"got unexpected error while forwarding Redis server logs",
				)
			}

			break
		}

		line = line[:len(line)-1]

		if len(line) > 0 && '0' <= line[0] && line[0] <= '9' {
			if submatch := logLineRgx.FindSubmatch(line); submatch != nil {
				timeStamp, errTime := time.ParseInLocation(
					"2 Jan 2006 15:04:05.999999999", string(submatch[3]), time.Local,
				)
				if errTime != nil {
					timeStamp = time.Now()
				}

				pid, errPid := strconv.ParseUint(string(submatch[1]), 10, 64)
				if errPid != nil {
					pid = 0
				}

				log.WithTime(timeStamp).WithFields(map[string]interface{}{
					"component": "redis-server",
					"pid":       pid,
					"role":      roles[submatch[2][0]],
				}).Log(logLevels[submatch[4][0]], string(submatch[5]))
			}
		}
	}

	if errWait := s.cmd.Wait(); errWait != nil {
		log.WithFields(log.Fields{"error": errWait}).Error("Redis server terminated with an error")
	}

	os.RemoveAll(s.basedir)
	s.basedir = ""

	close(s.stopped)
}
