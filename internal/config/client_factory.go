package config

import (
	"github.com/containerssh/containerssh/config"
	http2 "github.com/containerssh/containerssh/http"
	"github.com/containerssh/containerssh/internal/metrics"
	"github.com/containerssh/containerssh/log"
)

// MetricNameConfigBackendRequests is the number of requests to the config server
const MetricNameConfigBackendRequests = "containerssh_config_server_requests"

// MetricNameConfigBackendFailure is the number of request failures to the configuration backend.
const MetricNameConfigBackendFailure = "containerssh_config_server_failures"

// NewClient creates a new configuration client that can be used to fetch a user-specific configuration.
func NewClient(
	config config.ClientConfig,
	logger log.Logger,
	metricsCollector metrics.Collector,
) (Client, error) {
	var httpClient http2.Client
	var err error
	if config.HTTPClientConfiguration.URL != "" {
		httpClient, err = http2.NewClient(config.HTTPClientConfiguration, logger)
		if err != nil {
			return nil, err
		}
	}
	backendRequestsMetric := metricsCollector.MustCreateCounter(
		MetricNameConfigBackendRequests,
		"requests",
		"The number of requests sent to the configuration server.",
	)
	backendFailureMetric := metricsCollector.MustCreateCounter(
		MetricNameConfigBackendFailure,
		"requests",
		"The number of request failures to the configuration server.",
	)
	return &client{
		httpClient:            httpClient,
		logger:                logger,
		backendRequestsMetric: backendRequestsMetric,
		backendFailureMetric:  backendFailureMetric,
	}, nil
}