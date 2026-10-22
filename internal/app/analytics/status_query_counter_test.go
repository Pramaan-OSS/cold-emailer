package analytics

import (
	"context"
	"github.com/jackc/pgx/v5"
	"sync/atomic"
)

type statusQueryCounter struct{ queries atomic.Int64 }

func (c *statusQueryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.queries.Add(1)
	return ctx
}

func (*statusQueryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
