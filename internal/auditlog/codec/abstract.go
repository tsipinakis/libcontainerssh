package codec

import (
	"io"

	"github.com/containerssh/containerssh/auditlog/message"
	"github.com/containerssh/containerssh/internal/auditlog/storage"
)

// Encoder is a module that is responsible for receiving audit log messages and writing them to a writer.
type Encoder interface {
	// Encode takes messages from a the messages channel, encodes them in the desired format, and writes them to
	//        storage. When the messages channel is closed it will also close the storage writer.
	Encode(messages <-chan message.Message, storage storage.Writer) error
	// GetMimeType returns the MIME type of this format
	GetMimeType() string
	// GetFileExtension returns the desired file extension for this format
	GetFileExtension() string
}

// Decoder is a module that is resonsible for decoding a binary testdata stream into audit log messages.
type Decoder interface {
	Decode(reader io.Reader) (<-chan message.Message, <-chan error)
}
