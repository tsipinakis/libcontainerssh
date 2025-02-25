package auth

import (
	"encoding/base64"
	"fmt"
	goHttp "net/http"
	"strings"

	"github.com/containerssh/containerssh/auth"
	"github.com/containerssh/containerssh/http"
	"github.com/containerssh/containerssh/log"
	"github.com/containerssh/containerssh/message"
)

type handler struct {
	passwordHandler goHttp.Handler
	pubkeyHandler   goHttp.Handler
}

func (h handler) ServeHTTP(writer goHttp.ResponseWriter, request *goHttp.Request) {
	parts := strings.Split(request.URL.Path, "/")
	switch parts[len(parts)-1] {
	case "password":
		h.passwordHandler.ServeHTTP(writer, request)
	case "pubkey":
		h.pubkeyHandler.ServeHTTP(writer, request)
	default:
		writer.WriteHeader(404)
	}
}

type passwordHandler struct {
	backend Handler
	logger  log.Logger
}

func (p *passwordHandler) OnRequest(request http.ServerRequest, response http.ServerResponse) error {
	requestObject := auth.PasswordAuthRequest{}
	if err := request.Decode(&requestObject); err != nil {
		return err
	}
	password, err := base64.StdEncoding.DecodeString(requestObject.Password)
	if err != nil {
		return fmt.Errorf("failed to decode password (%w)", err)
	}
	success, metadata, err := p.backend.OnPassword(
		requestObject.Username,
		password,
		requestObject.RemoteAddress,
		requestObject.ConnectionID,
	)
	if err != nil {
		p.logger.Debug(message.Wrap(err, message.EAuthRequestDecodeFailed, "failed to execute password request"))
		response.SetStatus(500)
		response.SetBody(
			auth.ResponseBody{
				Success:  false,
				Metadata: metadata,
			})
		return nil
	} else {
		response.SetBody(
			auth.ResponseBody{
				Success:  success,
				Metadata: metadata,
			})
	}
	return nil
}

type pubKeyHandler struct {
	backend Handler
	logger  log.Logger
}

func (p *pubKeyHandler) OnRequest(request http.ServerRequest, response http.ServerResponse) error {
	requestObject := auth.PublicKeyAuthRequest{}
	if err := request.Decode(&requestObject); err != nil {
		return err
	}
	success, metadata, err := p.backend.OnPubKey(
		requestObject.Username,
		requestObject.PublicKey,
		requestObject.RemoteAddress,
		requestObject.ConnectionID,
	)
	if err != nil {
		p.logger.Debug(message.Wrap(err, message.EAuthRequestDecodeFailed, "failed to execute public key request"))
		response.SetStatus(500)
		response.SetBody(
			auth.ResponseBody{
				Success:  false,
				Metadata: metadata,
			})
		return nil
	} else {
		response.SetBody(
			auth.ResponseBody{
				Success:  success,
				Metadata: metadata,
			})
	}
	return nil
}
