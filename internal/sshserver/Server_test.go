package sshserver_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/containerssh/containerssh/config"
	"github.com/containerssh/containerssh/internal/structutils"
	"github.com/containerssh/containerssh/log"
	"github.com/containerssh/containerssh/service"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"

	"github.com/containerssh/containerssh/internal/sshserver"
)

//region Tests

func TestReadyRejection(t *testing.T) {
	cfg := config.SSHConfig{}
	structutils.Defaults(&cfg)
	if err := cfg.GenerateHostKey(); err != nil {
		assert.Fail(t, "failed to generate host key", err)
		return
	}
	logger := log.NewTestLogger(t)
	handler := &rejectHandler{}

	server, err := sshserver.New(cfg, handler, logger)
	if err != nil {
		assert.Fail(t, "failed to create server", err)
		return
	}
	lifecycle := service.NewLifecycle(server)
	err = lifecycle.Run()
	if err == nil {
		assert.Fail(t, "server.Run() did not result in an error")
	} else {
		assert.Equal(t, "rejected", err.Error())
	}
	lifecycle.Stop(context.Background())
}

func TestAuthFailed(t *testing.T) {
	server := newServerHelper(
		t,
		"127.0.0.1:2222",
		map[string][]byte{
			"foo": []byte("bar"),
		},
		map[string]string{},
	)
	hostKey, err := server.start(t)
	if err != nil {
		assert.Fail(t, "failed to start ssh server", err)
		return
	}
	defer func() {
		server.stop()
		<-server.shutdownChannel
	}()

	sshConfig := &ssh.ClientConfig{
		User: "foo",
		Auth: []ssh.AuthMethod{ssh.Password("invalid")},
	}
	sshConfig.HostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		marshaledKey := key.Marshal()
		if bytes.Equal(marshaledKey, hostKey) {
			return nil
		}
		return fmt.Errorf("invalid host")
	}

	sshConnection, err := ssh.Dial("tcp", "127.0.0.1:2222", sshConfig)
	if err != nil {
		if !strings.Contains(err.Error(), "unable to authenticate") {
			assert.Fail(t, "handshake failed for non-auth reasons", err)
		}
	} else {
		_ = sshConnection.Close()
		assert.Fail(t, "authentication succeeded", err)
	}
}

func TestAuthKeyboardInteractive(t *testing.T) {
	user1 := sshserver.NewTestUser("test")
	user1.AddKeyboardInteractiveChallengeResponse("foo", "bar")

	user2 := sshserver.NewTestUser("test")
	user2.AddKeyboardInteractiveChallengeResponse("foo", "baz")

	logger := log.NewTestLogger(t)
	srv := sshserver.NewTestServer(
		sshserver.NewTestAuthenticationHandler(
			sshserver.NewTestHandler(),
			user2,
		),
		logger,
	)
	srv.Start()

	client1 := sshserver.NewTestClient(srv.GetListen(), srv.GetHostKey(), user1, logger)
	conn, err := client1.Connect()
	if err == nil {
		_ = conn.Close()
		t.Fatal("invalid keyboard-interactive authentication did not result in an error")
	}

	client2 := sshserver.NewTestClient(srv.GetListen(), srv.GetHostKey(), user2, logger)
	conn, err = client2.Connect()
	if err != nil {
		t.Fatalf("valid keyboard-interactive authentication resulted in an error (%v)", err)
	}
	_ = conn.Close()

	defer srv.Stop(10 * time.Second)
}

func TestSessionSuccess(t *testing.T) {
	server := newServerHelper(
		t,
		"127.0.0.1:2222",
		map[string][]byte{
			"foo": []byte("bar"),
		},
		map[string]string{},
	)
	hostKey, err := server.start(t)
	if err != nil {
		assert.Fail(t, "failed to start ssh server", err)
		return
	}
	defer func() {
		server.stop()
		<-server.shutdownChannel
	}()

	reply, exitStatus, err := shellRequestReply(
		"127.0.0.1:2222",
		"foo",
		ssh.Password("bar"),
		hostKey,
		[]byte("Hi"),
		nil,
		nil,
	)
	assert.Equal(t, []byte("Hello world!"), reply)
	assert.Equal(t, 0, exitStatus)
	assert.Equal(t, nil, err)
}

func TestSessionError(t *testing.T) {
	server := newServerHelper(
		t,
		"127.0.0.1:2222",
		map[string][]byte{
			"foo": []byte("bar"),
		},
		map[string]string{},
	)
	hostKey, err := server.start(t)
	if err != nil {
		assert.Fail(t, "failed to start ssh server", err)
		return
	}
	defer func() {
		server.stop()
		<-server.shutdownChannel
	}()

	reply, exitStatus, err := shellRequestReply(
		"127.0.0.1:2222",
		"foo",
		ssh.Password("bar"),
		hostKey,
		[]byte("Ho"),
		nil,
		nil,
	)
	assert.Equal(t, 1, exitStatus)
	assert.Equal(t, []byte{}, reply)
	assert.Equal(t, nil, err)
}

func TestPubKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(
		rand.Reader,
		4096,
	)
	assert.Nil(t, err, "failed to generate RSA key (%v)", err)
	signer, err := ssh.NewSignerFromKey(rsaKey)
	assert.Nil(t, err, "failed to create signer (%v)", err)
	publicKey := signer.PublicKey()
	authorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
	server := newServerHelper(
		t,
		"127.0.0.1:2222",
		map[string][]byte{},
		map[string]string{
			"foo": authorizedKey,
		},
	)
	hostKey, err := server.start(t)
	if err != nil {
		assert.Fail(t, "failed to start ssh server", err)
		return
	}
	defer func() {
		server.stop()
		<-server.shutdownChannel
	}()

	reply, exitStatus, err := shellRequestReply(
		"127.0.0.1:2222",
		"foo",
		ssh.PublicKeys(signer),
		hostKey,
		[]byte("Hi"),
		nil,
		nil,
	)
	assert.Nil(t, err, "failed to send shell request (%v)", err)
	assert.Equal(t, 0, exitStatus)
	assert.Equal(t, []byte("Hello world!"), reply)
}

//endregion

//region Helper

func shellRequestReply(
	host string,
	user string,
	authMethod ssh.AuthMethod,
	hostKey []byte,
	request []byte,
	onShell chan struct{},
	canSendResponse chan struct{},
) (reply []byte, exitStatus int, err error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{authMethod},
	}
	sshConfig.HostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if bytes.Equal(key.Marshal(), hostKey) {
			return nil
		}
		return fmt.Errorf("invalid host")
	}
	sshConnection, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, -1, fmt.Errorf("handshake failed (%w)", err)
	}
	defer func() {
		if sshConnection != nil {
			_ = sshConnection.Close()
		}
	}()

	session, err := sshConnection.NewSession()
	if err != nil {
		return nil, -1, fmt.Errorf("new session failed (%w)", err)
	}

	stdin, stdout, err := createPipe(session)
	if err != nil {
		return nil, -1, err
	}

	if err := session.Setenv("TERM", "xterm"); err != nil {
		return nil, -1, err
	}

	if err := session.Shell(); err != nil {
		return nil, -1, fmt.Errorf("failed to request shell (%w)", err)
	}
	if onShell != nil {
		onShell <- struct{}{}
	}
	if canSendResponse != nil {
		<-canSendResponse
	}
	if _, err := stdin.Write(request); err != nil {
		return nil, -1, fmt.Errorf("failed to write to shell (%w)", err)
	}
	return read(stdout, stdin, session)
}

func read(stdout io.Reader, stdin io.WriteCloser, session *ssh.Session) (
	[]byte,
	int,
	error,
) {
	var exitStatus int
	data := make([]byte, 4096)
	n, err := stdout.Read(data)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, -1, fmt.Errorf("failed to read from stdout (%w)", err)
	}
	if err := stdin.Close(); err != nil && !errors.Is(err, io.EOF) {
		return data[:n], -1, fmt.Errorf("failed to close stdin (%w)", err)
	}
	if err := session.Wait(); err != nil {
		exitError := &ssh.ExitError{}
		if errors.As(err, &exitError) {
			exitStatus = exitError.ExitStatus()
		} else {
			return data[:n], -1, fmt.Errorf("failed to wait for exit (%w)", err)
		}
	}
	if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
		return data[:n], -1, fmt.Errorf("failed to close session (%w)", err)
	}
	return data[:n], exitStatus, nil
}

func createPipe(session *ssh.Session) (io.WriteCloser, io.Reader, error) {
	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to request stdin (%w)", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to request stdout (%w)", err)
	}
	return stdin, stdout, nil
}

func newServerHelper(
	t *testing.T,
	listen string,
	passwords map[string][]byte,
	pubKeys map[string]string,
) *serverHelper {
	return &serverHelper{
		t:               t,
		listen:          listen,
		passwords:       passwords,
		pubKeys:         pubKeys,
		receivedChannel: make(chan struct{}, 1),
	}
}

type serverHelper struct {
	t               *testing.T
	server          sshserver.Server
	lifecycle       service.Lifecycle
	passwords       map[string][]byte
	pubKeys         map[string]string
	listen          string
	shutdownChannel chan struct{}
	receivedChannel chan struct{}
}

func (h *serverHelper) start(t *testing.T) (hostKey []byte, err error) {
	if h.server != nil {
		return nil, fmt.Errorf("server already running")
	}
	cfg := config.SSHConfig{}
	structutils.Defaults(&cfg)
	cfg.Listen = h.listen
	if err := cfg.GenerateHostKey(); err != nil {
		return nil, err
	}
	private, err := ssh.ParsePrivateKey([]byte(cfg.HostKeys[0]))
	if err != nil {
		return nil, err
	}
	hostKey = private.PublicKey().Marshal()
	logger := log.NewTestLogger(t)
	readyChannel := make(chan struct{}, 1)
	h.shutdownChannel = make(chan struct{}, 1)
	errChannel := make(chan error, 1)
	handler := newFullHandler(
		readyChannel,
		h.shutdownChannel,
		h.passwords,
		h.pubKeys,
	)
	server, err := sshserver.New(cfg, handler, logger)
	if err != nil {
		return hostKey, err
	}
	lifecycle := service.NewLifecycle(server)
	h.lifecycle = lifecycle
	go func() {
		err = lifecycle.Run()
		if err != nil {
			errChannel <- err
		}
	}()
	//Wait for the server to be ready
	select {
	case err := <-errChannel:
		return hostKey, err
	case <-readyChannel:
	}
	h.server = server
	return hostKey, nil
}

func (h *serverHelper) stop() {
	if h.lifecycle != nil {
		shutdownContext, cancelFunc := context.WithTimeout(context.Background(), 60*time.Second)
		h.lifecycle.Stop(shutdownContext)
		cancelFunc()
	}
}

//endregion

//region Handlers

//region Rejection

type rejectHandler struct {
}

func (r *rejectHandler) OnReady() error {
	return fmt.Errorf("rejected")
}

func (r *rejectHandler) OnShutdown(_ context.Context) {
}

func (r *rejectHandler) OnNetworkConnection(_ net.TCPAddr, _ string) (sshserver.NetworkConnectionHandler, error) {
	return nil, fmt.Errorf("not implemented")
}

//endregion

//region Full

func newFullHandler(
	readyChannel chan struct{},
	shutdownChannel chan struct{},
	passwords map[string][]byte,
	pubKeys map[string]string,
) sshserver.Handler {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &fullHandler{
		ctx:          ctx,
		cancelFunc:   cancelFunc,
		ready:        readyChannel,
		shutdownDone: shutdownChannel,
		passwords:    passwords,
		pubKeys:      pubKeys,
	}
}

//region Handler
type fullHandler struct {
	sshserver.AbstractHandler

	ctx             context.Context
	shutdownContext context.Context
	cancelFunc      context.CancelFunc
	passwords       map[string][]byte
	pubKeys         map[string]string
	ready           chan struct{}
	shutdownDone    chan struct{}
}

func (f *fullHandler) OnReady() error {
	f.ready <- struct{}{}
	return nil
}

func (f *fullHandler) OnShutdown(shutdownContext context.Context) {
	f.shutdownContext = shutdownContext
	<-f.shutdownContext.Done()
	close(f.shutdownDone)
}

func (f *fullHandler) OnNetworkConnection(_ net.TCPAddr, _ string) (sshserver.NetworkConnectionHandler, error) {
	return &fullNetworkConnectionHandler{
		handler: f,
	}, nil
}

//endregion

//region Network connection conformanceTestHandler

type fullNetworkConnectionHandler struct {
	sshserver.AbstractNetworkConnectionHandler

	handler *fullHandler
}

func (f *fullNetworkConnectionHandler) OnAuthPassword(
	username string,
	password []byte,
	_ string,
) (response sshserver.AuthResponse, metadata map[string]string, reason error) {
	if storedPassword, ok := f.handler.passwords[username]; ok && bytes.Equal(storedPassword, password) {
		return sshserver.AuthResponseSuccess, nil, nil
	}
	return sshserver.AuthResponseFailure, nil, fmt.Errorf("authentication failed")
}

func (f *fullNetworkConnectionHandler) OnAuthPubKey(username string, pubKey string, _ string,) (response sshserver.AuthResponse, metadata map[string]string, reason error) {
	if storedPubKey, ok := f.handler.pubKeys[username]; ok && storedPubKey == pubKey {
		return sshserver.AuthResponseSuccess, nil, nil
	}
	return sshserver.AuthResponseFailure, nil, fmt.Errorf("authentication failed")
}

func (f *fullNetworkConnectionHandler) OnHandshakeSuccess(_ string, _ string, _ map[string]string) (connection sshserver.SSHConnectionHandler, failureReason error) {
	return &fullSSHConnectionHandler{
		handler: f.handler,
	}, nil
}

//endregion

//region SSH connection conformanceTestHandler

type fullSSHConnectionHandler struct {
	sshserver.AbstractSSHConnectionHandler

	handler *fullHandler
}

func (f *fullSSHConnectionHandler) OnSessionChannel(
	_ uint64,
	_ []byte,
	session sshserver.SessionChannel,
) (channel sshserver.SessionChannelHandler, failureReason sshserver.ChannelRejection) {
	return &fullSessionChannelHandler{
		handler: f.handler,
		env:     map[string]string{},
		session: session,
	}, nil
}

//endregion

//region Session channel conformanceTestHandler

type fullSessionChannelHandler struct {
	sshserver.AbstractSessionChannelHandler

	handler *fullHandler
	env     map[string]string
	session sshserver.SessionChannel
}

func (f *fullSessionChannelHandler) OnEnvRequest(_ uint64, name string, value string) error {
	f.env[name] = value
	return nil
}

func (f *fullSessionChannelHandler) OnShell(
	_ uint64,
) error {
	stdin := f.session.Stdin()
	stdout := f.session.Stdout()
	go func() {
		data := make([]byte, 4096)
		n, err := stdin.Read(data)
		if err != nil {
			f.session.ExitStatus(1)
			_ = f.session.Close()
			return
		}
		if string(data[:n]) != "Hi" {
			f.session.ExitStatus(1)
			_ = f.session.Close()
			return
		}
		if _, err := stdout.Write([]byte("Hello world!")); err != nil {
			f.session.ExitStatus(1)
			_ = f.session.Close()

		}
		f.session.ExitStatus(0)
		_ = f.session.Close()
	}()
	return nil
}

func (f *fullSessionChannelHandler) OnSignal(_ uint64, _ string) error {
	return nil
}

func (f *fullSessionChannelHandler) OnWindow(_ uint64, _ uint32, _ uint32, _ uint32, _ uint32) error {
	return nil
}

func (f *fullSessionChannelHandler) OnShutdown(_ context.Context) {
	_ = f.session.Close()
}

//endregion

//endregion

//endregion
