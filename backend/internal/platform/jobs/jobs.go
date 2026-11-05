package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/provider"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type DispatchNotificationArgs struct {
	NotificationID string `json:"notificationId"`
}

func (DispatchNotificationArgs) Kind() string { return "dispatch_notification" }

type DispatchNotificationWorker struct {
	river.WorkerDefaults[DispatchNotificationArgs]
	pool         *pgxpool.Pool
	sender       provider.EmailSender
	webURL       string
	providerName string
}

func (w *DispatchNotificationWorker) Work(ctx context.Context, job *river.Job[DispatchNotificationArgs]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if job.Args.NotificationID == "" {
		return errors.New("jobs: notification id is required")
	}
	return w.dispatch(ctx, job.Args.NotificationID)
}

func NewClient(pool *pgxpool.Pool, logger *slog.Logger, sender provider.EmailSender, webURL, providerName string) (*river.Client[pgx.Tx], error) {
	if pool == nil || logger == nil || sender == nil || webURL == "" || providerName == "" {
		return nil, errors.New("jobs: pool, logger, email sender, web URL and provider name are required")
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &DispatchNotificationWorker{pool: pool, sender: sender, webURL: webURL, providerName: providerName})
	sweeper := &NotificationOutboxWorker{pool: pool}
	river.AddWorker(workers, sweeper)
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger: logger,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 20},
		},
		Workers: workers,
		PeriodicJobs: []*river.PeriodicJob{river.NewPeriodicJob(river.PeriodicInterval(30*time.Second), func() (river.JobArgs, *river.InsertOpts) {
			return NotificationOutboxArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "notification-outbox", RunOnStart: true})},
	})
	if err != nil {
		return nil, fmt.Errorf("jobs: creating river client: %w", err)
	}
	sweeper.client = client
	return client, nil
}

// JobUnitOfWork makes domain mutations and River insertion one PostgreSQL
// transaction. A caller must never publish or invoke a provider inside work.
type JobUnitOfWork struct {
	pool   *pgxpool.Pool
	client *river.Client[pgx.Tx]
}

func NewJobUnitOfWork(pool *pgxpool.Pool, client *river.Client[pgx.Tx]) *JobUnitOfWork {
	return &JobUnitOfWork{pool: pool, client: client}
}

type TxJobs interface {
	Insert(context.Context, river.JobArgs, *river.InsertOpts) error
}

func (u *JobUnitOfWork) WithinTx(
	ctx context.Context,
	options pgx.TxOptions,
	work func(context.Context, pgx.Tx, TxJobs) error,
) (err error) {
	tx, err := u.pool.BeginTx(ctx, options)
	if err != nil {
		return fmt.Errorf("jobs: beginning transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if err = work(ctx, tx, riverTxJobs{client: u.client, tx: tx}); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("jobs: committing transaction: %w", err)
	}
	return nil
}

type riverTxJobs struct {
	client *river.Client[pgx.Tx]
	tx     pgx.Tx
}

func (j riverTxJobs) Insert(ctx context.Context, args river.JobArgs, options *river.InsertOpts) error {
	if _, err := j.client.InsertTx(ctx, j.tx, args, options); err != nil {
		return fmt.Errorf("jobs: inserting transactional job: %w", err)
	}
	return nil
}
