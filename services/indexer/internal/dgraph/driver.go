package dgraph

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dgraph-io/dgo/v250"
	"github.com/dgraph-io/dgo/v250/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	Address string
	Logger  *slog.Logger
}

// Driver stores graph and vector data in Dgraph using its native vector and
// graph primitives.
type Driver struct {
	client *dgo.Dgraph
	conn   *grpc.ClientConn
	logger *slog.Logger

	metaMu        sync.Mutex
	metaPredicate map[string]string
}

func New(ctx context.Context, cfg Config) (*Driver, error) {
	if cfg.Address == "" {
		cfg.Address = "127.0.0.1:9080"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	conn, err := grpc.DialContext(ctx, cfg.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial dgraph %s: %w", cfg.Address, err)
	}
	driver := &Driver{
		client:        dgo.NewDgraphClient(api.NewDgraphClient(conn)),
		conn:          conn,
		logger:        cfg.Logger,
		metaPredicate: make(map[string]string),
	}
	if err := driver.ensureBaseSchemaWithRetry(ctx, 30*time.Second); err != nil {
		conn.Close()
		return nil, err
	}
	return driver, nil
}

func (d *Driver) Ping(ctx context.Context) error {
	_, err := d.client.NewReadOnlyTxn().Query(ctx, `{ q(func: has(dgraph.type), first: 1) { uid } }`)
	if err != nil {
		return fmt.Errorf("dgraph ping: %w", err)
	}
	return nil
}

func (d *Driver) Close() error {
	if d.conn == nil {
		return nil
	}
	return d.conn.Close()
}
