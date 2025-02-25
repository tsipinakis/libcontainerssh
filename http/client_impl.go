package http

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/containerssh/containerssh/config"
	"github.com/containerssh/containerssh/log"
	"github.com/containerssh/containerssh/message"
	"github.com/gorilla/schema"
)

type client struct {
	config config.HTTPClientConfiguration
	logger log.Logger
	tlsConfig        *tls.Config
	extraHeaders     map[string][]string
	allowLaxDecoding bool
}

func (c *client) Put(
	path string,
	requestBody interface{},
	responseBody interface{},
) (statusCode int, err error) {
	return c.request(
		http.MethodPut,
		path,
		requestBody,
		responseBody,
	)
}

func (c *client) Patch(
	path string,
	requestBody interface{},
	responseBody interface{},
) (statusCode int, err error) {
	return c.request(
		http.MethodPatch,
		path,
		requestBody,
		responseBody,
	)
}

func (c *client) Delete(
	path string,
	requestBody interface{},
	responseBody interface{},
) (statusCode int, err error) {
	return c.request(
		http.MethodDelete,
		path,
		requestBody,
		responseBody,
	)
}

func (c *client) Request(Method string, path string, requestBody interface{}, responseBody interface{}) (statusCode int, err error) {
	return c.request(
		Method,
		path,
		requestBody,
		responseBody,
	)
}

func (c *client) Get(path string, responseBody interface{}) (statusCode int, err error) {
	return c.request(
		http.MethodGet,
		path,
		nil,
		responseBody,
	)
}

func (c *client) Post(
	path string,
	requestBody interface{},
	responseBody interface{},
) (
	int,
	error,
) {
	return c.request(
		http.MethodPost,
		path,
		requestBody,
		responseBody,
	)
}

func (c *client) request(
	method string,
	path string,
	requestBody interface{},
	responseBody interface{},
) (int, error) {
	logger := c.logger.WithLabel("method", method).WithLabel("path", path)

	httpClient := c.createHTTPClient(logger)

	req, err := c.createRequest(method, path, requestBody, logger)
	if err != nil {
		return 0, err
	}

	logger.Debug(message.NewMessage(message.MHTTPClientRequest, "HTTP %s request to %s%s", method, c.config.URL, path))

	resp, err := httpClient.Do(req)
	if err != nil {
		var typedError message.Message
		if errors.As(err, &typedError) {
			return 0, err
		}
		err = message.Wrap(err,
			message.EHTTPFailureConnectionFailed, "HTTP %s request to %s%s failed", method, c.config.URL, path)
		logger.Debug(err)
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	logger.Debug(
		message.NewMessage(
			message.MHTTPClientResponse,
		"HTTP response with status %d",
		resp.StatusCode,
	).Label("statusCode", resp.StatusCode))

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = message.Wrap(err,
			message.EHTTPFailureConnectionFailed, "HTTP %s request to %s%s failed", method, c.config.URL, path)
		logger.Debug(err)
		return 0, err
	}

	if responseBody == nil {
		return resp.StatusCode, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if !c.allowLaxDecoding {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(responseBody); err != nil {
		err = message.Wrap(err, message.EHTTPFailureDecodeFailed, "Failed to decode HTTP response")
		logger.Debug(err)
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

func (c *client) createRequest(method string, path string, requestBody interface{}, logger log.Logger) (
	*http.Request,
	error,
) {
	buffer := &bytes.Buffer{}
	switch c.config.RequestEncoding {
	case config.RequestEncodingDefault:
		fallthrough
	case config.RequestEncodingJSON:
		err := json.NewEncoder(buffer).Encode(requestBody)
		if err != nil {
			//This is a bug
			err := message.Wrap(err, message.EHTTPFailureEncodeFailed, "BUG: HTTP request encoding failed")
			logger.Critical(err)
			return nil, err
		}
	case config.RequestEncodingWWWURLEncoded:
		encoder := schema.NewEncoder()
		form := url.Values{}
		if err := encoder.Encode(requestBody, form); err != nil {
			err := message.Wrap(err, message.EHTTPFailureEncodeFailed, "BUG: HTTP request encoding failed")
			logger.Critical(err)
			return nil, err
		}
		buffer.WriteString(form.Encode())
	default:
		panic(fmt.Errorf("invalid request encoding: %s", c.config.RequestEncoding))
	}
	req, err := http.NewRequest(
		method,
		fmt.Sprintf("%s%s", c.config.URL, path),
		buffer,
	)
	if err != nil {
		err := message.Wrap(err, message.EHTTPFailureEncodeFailed, "BUG: HTTP request encoding failed")
		logger.Critical(err)
		return nil, err
	}
	for header, values := range c.extraHeaders {
		for i, value := range values {
			if i == 0 {
				req.Header.Set(header, value)
			} else {
				req.Header.Add(header, value)
			}
		}
	}
	switch c.config.RequestEncoding {
	case config.RequestEncodingDefault:
		fallthrough
	case config.RequestEncodingJSON:
		req.Header.Set("Content-Type", "application/json")
	case config.RequestEncodingWWWURLEncoded:
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	default:
		panic(fmt.Errorf("invalid request encoding: %s", c.config.RequestEncoding))
	}
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (c *client) createHTTPClient(logger log.Logger) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: c.tlsConfig,
	}

	httpClient := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !c.config.AllowRedirects {
				return message.NewMessage(
					message.EHTTPClientRedirectsDisabled,
					"Redirects disabled, server tried to redirect to %s", req.URL,
				).Label("redirect", req.URL)
			}
			logger.Debug(
				message.NewMessage(
					message.MHTTPClientRedirect, "HTTP redirect to %s", req.URL,
				).Label("redirect", req.URL),
			)
			return nil
		},
		Timeout: c.config.Timeout,
	}
	return httpClient
}
