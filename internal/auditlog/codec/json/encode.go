package json

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/containerssh/libcontainerssh/auditlog/message"
	"github.com/containerssh/libcontainerssh/internal/auditlog/codec"
	"github.com/containerssh/libcontainerssh/internal/auditlog/storage"
	"github.com/containerssh/libcontainerssh/internal/geoip/geoipprovider"
)

// NewEncoder creates an encoder that encodes messages in JSON format
func NewEncoder(geoIPProvider geoipprovider.LookupProvider) codec.Encoder {
	return &encoder{
		geoIPProvider: geoIPProvider,
	}
}

type encoder struct {
	geoIPProvider geoipprovider.LookupProvider
}

func (e *encoder) GetMimeType() string {
	return "application/json"
}

func (e *encoder) GetFileExtension() string {
	return ".json"
}

func (e *encoder) Encode(messages <-chan message.Message, storage storage.Writer) error {
	startTime := int64(0)
	var ip = ""
	var country = "XX"
	var username *string
	for {
		msg, ok := <-messages
		if !ok {
			break
		}
		if startTime == 0 {
			startTime = msg.Timestamp
		}
		emsg := msg.GetExtendedMessage()
		// Convert nanos to millis as it's the most popular format
		
		emsg.Timestamp /= 1000000
		ip, country, username = e.storeMetadata(msg, storage, startTime, ip, country, username)
		buf, err := json.Marshal(emsg)
		if err != nil {
			return fmt.Errorf("failed to encode audit log message (%w)", err)
		}
		buf = append(buf, '\n')
		// Note: Each write MUST to be a complete json object for database stores to work correctly
		if _, err := storage.Write(buf); err != nil {
			return fmt.Errorf("Failed to write to remote (%w)", err)
		}
		if msg.MessageType == message.TypeDisconnect {
			break
		}
	}
	if err := storage.Close(); err != nil {
		return fmt.Errorf("failed to close audit log stream (%w)", err)
	}
	return nil
}

func (e *encoder) storeMetadata(
	msg message.Message,
	storage storage.Writer,
	startTime int64,
	ip string,
	country string,
	username *string,
) (string, string, *string) {
	switch msg.MessageType {
	case message.TypeConnect:
		remoteAddr := msg.Payload.(message.PayloadConnect).RemoteAddr
		ip = remoteAddr
		country := e.geoIPProvider.Lookup(net.ParseIP(ip))
		storage.SetMetadata(startTime/1000000000, ip, country, username)
	case message.TypeAuthPasswordSuccessful:
		u := msg.Payload.(message.PayloadAuthPassword).Username
		username = &u
		storage.SetMetadata(startTime/1000000000, ip, country, username)
	case message.TypeAuthPubKeySuccessful:
		payload := msg.Payload.(message.PayloadAuthPubKey)
		username = &payload.Username
		storage.SetMetadata(startTime/1000000000, ip, country, username)
	case message.TypeHandshakeSuccessful:
		payload := msg.Payload.(message.PayloadHandshakeSuccessful)
		username = &payload.Username
		storage.SetMetadata(startTime/1000000000, ip, country, username)
	}

	return ip, country, username
}
