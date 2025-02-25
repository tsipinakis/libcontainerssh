package none

import (
	"github.com/containerssh/containerssh/auditlog/message"
	"github.com/containerssh/containerssh/internal/auditlog/codec"
	"github.com/containerssh/containerssh/internal/auditlog/storage"
)

// NewEncoder creates an encoder that swallows everything. This can be used as a dummy encoder to not consume CPU.
func NewEncoder() codec.Encoder {
	return &encoder{}
}

type encoder struct {
}

func (e *encoder) GetMimeType() string {
	return "application/octet-stream"
}

func (e *encoder) GetFileExtension() string {
	return ""
}

func (e *encoder) Encode(messages <-chan message.Message, _ storage.Writer) error {
	for {
		msg, ok := <-messages
		if !ok {
			break
		}
		if msg.MessageType == message.TypeDisconnect {
			break
		}
	}
	return nil
}
