package auditlog

import (
	"fmt"
	"sync"

	"github.com/containerssh/containerssh/config"
	"github.com/containerssh/containerssh/internal/auditlog/codec"
	"github.com/containerssh/containerssh/internal/auditlog/codec/asciinema"
	"github.com/containerssh/containerssh/internal/auditlog/codec/binary"
	noneCodec "github.com/containerssh/containerssh/internal/auditlog/codec/none"
	"github.com/containerssh/containerssh/internal/auditlog/storage"
	"github.com/containerssh/containerssh/internal/auditlog/storage/file"
	noneStorage "github.com/containerssh/containerssh/internal/auditlog/storage/none"
	"github.com/containerssh/containerssh/internal/auditlog/storage/s3"

	"github.com/containerssh/containerssh/internal/geoip/geoipprovider"
	"github.com/containerssh/containerssh/log"
)

// New Creates a new audit logging pipeline based on the provided configuration.
func New(config config.AuditLogConfig, geoIPLookupProvider geoipprovider.LookupProvider, logger log.Logger) (Logger, error) {
	if !config.Enable {
		return &empty{}, nil
	}

	encoder, err := NewEncoder(config.Format, logger, geoIPLookupProvider)
	if err != nil {
		return nil, err
	}

	st, err := NewStorage(config, logger)
	if err != nil {
		return nil, err
	}

	return NewLogger(
		config.Intercept,
		encoder,
		st,
		logger,
		geoIPLookupProvider,
	)
}

// NewLogger creates a new audit logging pipeline with the provided elements.
func NewLogger(
	intercept config.AuditLogInterceptConfig,
	encoder codec.Encoder,
	storage storage.WritableStorage,
	logger log.Logger,
	geoIPLookup geoipprovider.LookupProvider,
) (Logger, error) {
	return &loggerImplementation{
		intercept:   intercept,
		encoder:     encoder,
		storage:     storage,
		logger:      logger,
		wg:          &sync.WaitGroup{},
		geoIPLookup: geoIPLookup,
	}, nil
}

// NewEncoder creates a new audit log encoder of the specified format.
func NewEncoder(encoder config.AuditLogFormat, logger log.Logger, geoIPLookupProvider geoipprovider.LookupProvider) (codec.Encoder, error) {
	switch encoder {
	case config.AuditLogFormatNone:
		return noneCodec.NewEncoder(), nil
	case config.AuditLogFormatAsciinema:
		return asciinema.NewEncoder(logger, geoIPLookupProvider), nil
	case config.AuditLogFormatBinary:
		return binary.NewEncoder(geoIPLookupProvider), nil
	default:
		return nil, fmt.Errorf("invalid audit log encoder: %s", encoder)
	}
}

// NewStorage creates a new audit log storage of the specified type and with the specified configuration.
func NewStorage(cfg config.AuditLogConfig, logger log.Logger) (storage.WritableStorage, error) {
	switch cfg.Storage {
	case config.AuditLogStorageNone:
		return noneStorage.NewStorage(), nil
	case config.AuditLogStorageFile:
		return file.NewStorage(cfg.File, logger)
	case config.AuditLogStorageS3:
		return s3.NewStorage(cfg.S3, logger)
	default:
		return nil, fmt.Errorf("invalid audit log storage: %s", cfg.Storage)
	}
}
