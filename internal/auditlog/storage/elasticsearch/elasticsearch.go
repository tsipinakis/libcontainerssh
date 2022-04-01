package elasticsearch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/containerssh/libcontainerssh/config"
	"github.com/containerssh/libcontainerssh/internal/auditlog/storage"

	"github.com/containerssh/libcontainerssh/log"
	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchutil"
)

// NewStorage Create a file storage that stores testdata in a local directory. The file storage cannot store metadata.
func NewStorage(cfg config.AuditLogElasticSearchConfig, log log.Logger) (storage.ReadWriteStorage, error) {
	return &elasticSearchStorage{
		cfg: cfg,
		log: log,
	}, nil
}

type elasticSearchStorage struct {
	cfg config.AuditLogElasticSearchConfig
	log log.Logger
}

func (s *elasticSearchStorage) newClient() (*opensearch.Client, error) {
	es, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{""},
		Username:  "",
		Password:  "",

		RetryOnStatus: []int{502, 503, 504, 429},
		RetryBackoff: func(i int) time.Duration {
			return time.Duration(1) * time.Second
		},
		MaxRetries: 5,
	})
	if err != nil {
		s.log.Debug(err)
		return nil, err
	}
	return es, nil
}

func (s *elasticSearchStorage) Shutdown(_ context.Context) {
}

// OpenReader opens a reader for a specific audit log
func (s *elasticSearchStorage) OpenReader(name string) (io.ReadCloser, error) {
	return nil, nil
}

// List lists the available audit logs
func (s *elasticSearchStorage) List() (<-chan storage.Entry, <-chan error) {
	es, err := s.newClient()
	if err != nil {
		s.log.Error(err)
		return nil, nil
	}
	
	return nil, nil
}

// OpenWriter opens a writer to store an audit log
func (s *elasticSearchStorage) OpenWriter(name string) (storage.Writer, error) {
	es, err := s.newClient()
	if err != nil {
		return nil, err
	}

	res, err := es.Indices.Create(
		"plus_containerssh_test",
		es.Indices.Create.WithBody(strings.NewReader(`{
	  "mappings": {
	    "properties": {
	      "timestamp": {
	        "type": "date",
			"format": "epoch_millis"
	      },
		  "connectionId": {
			  "type": "keyword"
		  }
	    }
	  }
	}`)),
	)
	fmt.Println(res, err)

	bi, err := opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		Index:         "plus_containerssh_test",
		Client:        es,
		FlushInterval: 30 * time.Second,
	})
	if err != nil {
		s.log.Debug(err)
		return nil, err
	}

	//res, err := es.Indices.Create("plus_containerssh_test")
	//if err != nil {
	//	s.log.Debug(err)
	//	return nil, err
	//}
	//if res.IsError() {
	//	s.log.Debug(err)
	//	return nil, fmt.Errorf("Result returned error %s", res)
	//}
	//res.Body.Close()

	return &writer{
		bulkIndexer: bi,
	}, nil
}

type reader struct {
	log log.Logger
}

func (r *reader) Read(p []byte) (n int, err error) {
	return 0, nil
}

func (r *reader) Close() error {
	return nil
}

type writer struct {
	bulkIndexer opensearchutil.BulkIndexer
	log         log.Logger
}

func (w *writer) Write(p []byte) (n int, err error) {
	err = w.bulkIndexer.Add(
		context.Background(),
		opensearchutil.BulkIndexerItem{
			Action: "index",
			Body:   bytes.NewReader(p),
			OnFailure: func(ctx context.Context, bii opensearchutil.BulkIndexerItem, res opensearchutil.BulkIndexerResponseItem, err error) {
				if err != nil {
					// TODO: FIXME
					fmt.Printf("ERROR: %s", err)
				} else {
					fmt.Printf("ERROR: %s: %s", res.Error.Type, res.Error.Reason)
				}
			},
		},
	)
	if err != nil {
		return 0, err
	}
	fmt.Println(string(p))
	return len(p), nil
}

func (w *writer) Close() error {
	return w.bulkIndexer.Close(context.Background())
}

func (w *writer) SetMetadata(startTime int64, sourceIP string, country string, username *string) {
}
