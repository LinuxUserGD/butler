package integrate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/fatih/color"

	"github.com/itchio/butler/butlerd"
	"github.com/itchio/butler/butlerd/jsonrpc2"
	"github.com/itchio/butler/butlerd/messages"
	"github.com/itchio/headway/state"
	"github.com/itchio/mitch"
	"github.com/pkg/errors"
)

type ButlerConn struct {
	Ctx            context.Context
	Cancel         context.CancelFunc
	RequestContext *butlerd.RequestContext
	Handler        *handler
}

type ButlerInstance struct {
	Ctx      context.Context
	Cancel   context.CancelFunc
	Address  string
	Secret   string
	Consumer *state.Consumer
	Logf     func(format string, args ...interface{})
	Conn     *ButlerConn

	t      *testing.T
	opts   instanceOpts
	Server mitch.Server
}

type instanceOpts struct {
}

type instanceOpt func(o *instanceOpts)

func init() {
	color.NoColor = false
}

func newInstance(t *testing.T, options ...instanceOpt) *ButlerInstance {
	var opts instanceOpts
	for _, o := range options {
		o(&opts)
	}

	ctx, cancel := context.WithCancel(context.Background())

	logf := t.Logf
	if os.Getenv("LOUD_TESTS") == "1" {
		logf = func(msg string, args ...interface{}) {
			fmt.Printf("%s\n", fmt.Sprintf(msg, args...))
		}
	}

	if os.Getenv("QUIET_TESTS") == "1" {
		logf = func(msg string, args ...interface{}) {
			// muffin
		}
	}

	{
		var timeLock sync.Mutex
		lastTime := time.Now().UTC()
		oldlogf := logf
		logf = func(msg string, args ...interface{}) {
			timeLock.Lock()
			diff := time.Since(lastTime)
			lastTime = time.Now().UTC()
			timeLock.Unlock()
			timestampString := fmt.Sprintf("%12s", "")
			if diff >= 2*time.Millisecond {
				msDiff := int64(diff.Seconds() * 1000.0)
				timestampBase := fmt.Sprintf("+%d ms", msDiff)
				timestampString = fmt.Sprintf("%12s", timestampBase)
			}
			oldlogf("%s %s", color.GreenString(timestampString), fmt.Sprintf(msg, args...))
		}
	}

	consumer := &state.Consumer{
		OnMessage: func(lvl string, msg string) {
			var col = color.WhiteString
			switch lvl {
			case "debug":
				col = color.BlueString
			case "info":
				col = color.CyanString
			case "warn":
				col = color.YellowString
			case "error":
				col = color.RedString
			}
			logf("%s", col(msg))
		},
	}

	server, err := mitch.NewServer(ctx, mitch.WithConsumer(consumer))
	must(err)

	args := []string{
		"daemon",
		"--json",
		"--transport", "tcp",
		"--keep-alive",
		"--dbpath", "file::memory:?cache=shared",
		"--destiny-pid", conf.PidString,
		"--destiny-pid", conf.PpidString,
	}
	{
		addressString := fmt.Sprintf("http://%s", server.Address())
		args = append(args, "--address", addressString)
		logf("Using mock server %s", addressString)
	}
	bExec := exec.CommandContext(ctx, conf.ButlerPath, args...)

	stdout, err := bExec.StdoutPipe()
	must(err)

	stderr, err := bExec.StderrPipe()
	must(err)
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			consumer.Infof("[%s] %s", "butler stderr", s.Text())
		}
	}()

	must(bExec.Start())

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- bExec.Wait()
	}()

	s := bufio.NewScanner(stdout)
	addrChan := make(chan string)

	var secret string
	go func() {
		defer cancel()

		for s.Scan() {
			line := s.Text()

			im := make(map[string]interface{})
			err := json.Unmarshal([]byte(line), &im)
			if err != nil {
				consumer.Infof("[%s] %s", "butler stdout", line)
				continue
			}

			typ := im["type"].(string)
			switch typ {
			case "butlerd/listen-notification":
				secret = im["secret"].(string)
				tcpBlock := im["tcp"].(map[string]interface{})
				addrChan <- tcpBlock["address"].(string)
			case "log":
				consumer.Infof("[butler] %s", im["message"].(string))
			default:
				must(errors.Errorf("unknown butlerd request: %s", typ))
			}
		}
	}()

	var address string
	select {
	case address = <-addrChan:
		// cool!
	case err := <-waitErr:
		must(err)
	case <-time.After(2 * time.Second):
		must(errors.Errorf("Timed out waiting for butlerd address"))
	}
	must(err)

	bi := &ButlerInstance{
		t:        t,
		opts:     opts,
		Ctx:      ctx,
		Cancel:   cancel,
		Address:  address,
		Secret:   secret,
		Logf:     logf,
		Consumer: consumer,
		Server:   server,
	}
	bi.Connect()
	bi.SetupTmpInstallLocation()

	return bi
}

func (bi *ButlerInstance) Unwrap() (*butlerd.RequestContext, *handler, context.CancelFunc) {
	return bi.Conn.RequestContext, bi.Conn.Handler, bi.Cancel
}

func (bi *ButlerInstance) Disconnect() {
	bi.Conn.Cancel()
	bi.Conn = nil
}

func (bi *ButlerInstance) Connect() (*butlerd.RequestContext, *handler, context.CancelFunc) {
	ctx, cancel := context.WithCancel(bi.Ctx)

	h := newHandler(bi.Consumer)

	messages.Log.Register(h, func(params butlerd.LogNotification) {
		bi.Consumer.OnMessage(string(params.Level), params.Message)
	})

	tcpConn, err := net.DialTimeout("tcp", bi.Address, 2*time.Second)
	must(err)

	jc := jsonrpc2.NewConn(ctx, jsonrpc2.NewRwcTransport(tcpConn), h)

	rc := &butlerd.RequestContext{
		Conn:     jc,
		Ctx:      ctx,
		Consumer: bi.Consumer,
	}

	_, err = messages.MetaAuthenticate.TestCall(rc, butlerd.MetaAuthenticateParams{
		Secret: bi.Secret,
	})
	must(err)

	bi.Conn = &ButlerConn{
		Ctx:            ctx,
		Cancel:         cancel,
		Handler:        h,
		RequestContext: rc,
	}
	return bi.Unwrap()
}
