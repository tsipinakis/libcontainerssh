package auditlog

import (
	"context"
	"io"
	"net"

	"github.com/containerssh/containerssh/auditlog/message"
)

// Logger is a top level audit logger.
type Logger interface {
	// OnConnect creates an audit log message for a new connection and simultaneously returns a connection object for
	//           connection-specific messages
	OnConnect(connectionID message.ConnectionID, ip net.TCPAddr) (Connection, error)
	// Shutdown triggers all failing uploads to cancel, waits for all currently running uploads to finish, then returns.
	// When the shutdownContext expires it will do its best to immediately upload any running background processes.
	Shutdown(shutdownContext context.Context)
}

// Connection is an audit logger for a specific connection
type Connection interface {
	// OnDisconnect creates an audit log message for a disconnect event.
	OnDisconnect()

	// OnAuthPassword creates an audit log message for an authentication attempt.
	OnAuthPassword(username string, password []byte)
	// OnAuthPasswordSuccess creates an audit log message for a successful authentication.
	OnAuthPasswordSuccess(username string, password []byte)
	// OnAuthPasswordFailed creates an audit log message for a failed authentication.
	OnAuthPasswordFailed(username string, password []byte)
	// OnAuthPasswordBackendError creates an audit log message for an auth server (backend) error during password
	//                            verification.
	OnAuthPasswordBackendError(username string, password []byte, reason string)

	// OnAuthPubKey creates an audit log message for an authentication attempt with public key.
	OnAuthPubKey(username string, pubKey string)
	// OnAuthPubKeySuccess creates an audit log message for a successful public key authentication.
	OnAuthPubKeySuccess(username string, pubKey string)
	// OnAuthPubKeyFailed creates an audit log message for a failed public key authentication.
	OnAuthPubKeyFailed(username string, pubKey string)
	// OnAuthPubKeyBackendError creates an audit log message for a failure while talking to the auth server (backend)
	//                          during public key authentication.
	OnAuthPubKeyBackendError(username string, pubKey string, reason string)

	// OnAuthKeyboardInteractiveChallenge is sent when a keyboard-interactive challenge is issued to the user.
	// Multiple of these challenges can be issued to a user, even after a previous password or pubkey authentication.
	OnAuthKeyboardInteractiveChallenge(
		username string,
		instruction string,
		questions []message.KeyboardInteractiveQuestion,
	)
	// OnAuthKeyboardInteractiveAnswer is recording an answer from a user to a keyboard-interactive authentication.
	OnAuthKeyboardInteractiveAnswer(
		username string,
		answers []message.KeyboardInteractiveAnswer,
	)
	// OnAuthKeyboardInteractiveFailed records a failure during the keyboard-interactive authentication.
	OnAuthKeyboardInteractiveFailed(username string)
	// OnAuthKeyboardInteractiveBackendError records a backend failure during the keyboard-interactive authentication.
	OnAuthKeyboardInteractiveBackendError(username string, reason string)

	// OnHandshakeFailed creates an entry that indicates a handshake failure.
	OnHandshakeFailed(reason string)
	// OnHandshakeSuccessful creates an entry that indicates a successful SSH handshake.
	OnHandshakeSuccessful(username string)

	// OnGlobalRequestUnknown creates an audit log message for a global request that is not supported.
	OnGlobalRequestUnknown(requestType string)

	// OnNewChannel creates an audit log message for a new channel request.
	OnNewChannel(channelID message.ChannelID, channelType string)
	// OnNewChannelFailed creates an audit log message for a failure in requesting a new channel.
	OnNewChannelFailed(channelID message.ChannelID, channelType string, reason string)
	// OnNewChannelSuccess creates an audit log message for successfully requesting a new channel and returns a
	//                     channel-specific audit logger.
	OnNewChannelSuccess(channelID message.ChannelID, channelType string) Channel
}

// Channel is an audit logger for one specific hannel
type Channel interface {
	// OnRequestUnknown creates an audit log message for a channel request that is not supported.
	OnRequestUnknown(requestID uint64, requestType string, payload []byte)
	// OnRequestDecodeFailed creates an audit log message for a channel request that is supported but could not be
	//                       decoded.
	OnRequestDecodeFailed(requestID uint64, requestType string, payload []byte, reason string)
	// OnRequestFailed is called when a backend failed to respond to a request.
	OnRequestFailed(requestID uint64, reason error)

	// OnRequestSetEnv creates an audit log message for a channel request to set an environment variable.
	OnRequestSetEnv(requestID uint64, name string, value string)
	// OnRequestExec creates an audit log message for a channel request to execute a program.
	OnRequestExec(requestID uint64, program string)
	// OnRequestPty creates an audit log message for a channel request to create an interactive terminal.
	OnRequestPty(requestID uint64, term string, columns uint32, rows uint32, width uint32, height uint32, modeList []byte)
	// OnRequestExec creates an audit log message for a channel request to execute a shell.
	OnRequestShell(requestID uint64)
	// OnRequestExec creates an audit log message for a channel request to send a signal to the currently running
	//               program.
	OnRequestSignal(requestID uint64, signal string)
	// OnRequestExec creates an audit log message for a channel request to execute a well-known subsystem (e.g. SFTP)
	OnRequestSubsystem(requestID uint64, subsystem string)
	// OnRequestWindow creates an audit log message for a channel request to resize the current window.
	OnRequestWindow(requestID uint64, columns uint32, rows uint32, width uint32, height uint32)

	// GetStdinProxy creates an intercepting audit log reader proxy for the standard input.
	GetStdinProxy(stdin io.Reader) io.Reader
	// GetStdinProxy creates an intercepting audit log writer proxy for the standard output.
	GetStdoutProxy(stdout io.Writer) io.Writer
	// GetStdinProxy creates an intercepting audit log writer proxy for the standard error.
	GetStderrProxy(stderr io.Writer) io.Writer

	// OnExit is called when the executed program quits. The exitStatus parameter contains the exit code of the
	// application.
	OnExit(exitStatus uint32)

	// OnExitSignal is called when the executed program quits due to a signal.
	OnExitSignal(signal string, coreDumped bool, errorMessage string, languageTag string)

	// OnWriteClose is called when the channel is closed for writing.
	OnWriteClose()

	// OnClose is called when the channel is closed.
	OnClose()
}
