package auth

import (
	"fmt"

	"github.com/containerssh/containerssh/config"
	"github.com/containerssh/containerssh/http"
	"github.com/containerssh/containerssh/internal/metrics"
	"github.com/containerssh/containerssh/log"
	"github.com/containerssh/containerssh/message"
)

// NewHttpAuthClient creates a new HTTP authentication client
//goland:noinspection GoUnusedExportedFunction
func NewHttpAuthClient(
	cfg config.AuthConfig,
	logger log.Logger,
	metrics metrics.Collector,
) (Client, error) {
	if cfg.Method != config.AuthMethodWebhook {
		return nil, fmt.Errorf("authentication is not set to webhook")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if cfg.URL != "" {
		logger.Warning(
			message.NewMessage(
				message.EAuthDeprecated,
				"The auth.url setting is deprecated, please switch to using auth.webhook.url. See https://containerssh.io/deprecations/authurl for details.",
			))
		//goland:noinspection GoDeprecation
		cfg.Webhook.HTTPClientConfiguration = cfg.HTTPClientConfiguration
		//goland:noinspection GoDeprecation
		cfg.Webhook.Password = cfg.Password
		//goland:noinspection GoDeprecation
		cfg.Webhook.PubKey = cfg.PubKey
		//goland:noinspection GoDeprecation
		cfg.HTTPClientConfiguration = config.HTTPClientConfiguration{}
	}

	realClient, err := http.NewClient(
		cfg.Webhook.HTTPClientConfiguration,
		logger,
	)
	if err != nil {
		return nil, err
	}

	backendRequestsMetric, backendFailureMetric, authSuccessMetric, authFailureMetric := createMetrics(metrics)
	return &httpAuthClient{
		enablePassword:        cfg.Webhook.Password,
		enablePubKey:          cfg.Webhook.PubKey,
		timeout:               cfg.AuthTimeout,
		httpClient:            realClient,
		logger:                logger,
		metrics:               metrics,
		backendRequestsMetric: backendRequestsMetric,
		backendFailureMetric:  backendFailureMetric,
		authSuccessMetric:     authSuccessMetric,
		authFailureMetric:     authFailureMetric,
	}, nil
}

func createMetrics(metrics metrics.Collector) (
	metrics.Counter,
	metrics.Counter,
	metrics.GeoCounter,
	metrics.GeoCounter,
) {
	backendRequestsMetric := metrics.MustCreateCounter(
		MetricNameAuthBackendRequests,
		"requests",
		"The number of requests sent to the configuration server.",
	)
	backendFailureMetric := metrics.MustCreateCounter(
		MetricNameAuthBackendFailure,
		"requests",
		"The number of request failures to the configuration server.",
	)
	authSuccessMetric := metrics.MustCreateCounterGeo(
		MetricNameAuthSuccess,
		"requests",
		"The number of successful authentications.",
	)
	authFailureMetric := metrics.MustCreateCounterGeo(
		MetricNameAuthFailure,
		"requests",
		"The number of failed authentications.",
	)
	return backendRequestsMetric, backendFailureMetric, authSuccessMetric, authFailureMetric
}
