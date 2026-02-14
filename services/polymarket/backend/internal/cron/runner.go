package cronrunner

import (
	"context"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Runner struct {
	cron    *cron.Cron
	logger  *zap.Logger
	baseCtx context.Context
}

func New(logger *zap.Logger, baseCtx context.Context) *Runner {
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	return &Runner{
		cron:    cron.New(cron.WithSeconds()),
		logger:  logger,
		baseCtx: baseCtx,
	}
}

func (r *Runner) Add(spec string, job func(context.Context)) (cron.EntryID, error) {
	return r.cron.AddFunc(spec, func() {
		if r == nil {
			job(context.Background())
			return
		}
		if r.baseCtx == nil {
			job(context.Background())
			return
		}
		job(r.baseCtx)
	})
}

func (r *Runner) Start() {
	if r.logger != nil {
		r.logger.Info("cron started")
	}
	r.cron.Start()
}

func (r *Runner) Stop() {
	ctx := r.cron.Stop()
	<-ctx.Done()
	if r.logger != nil {
		r.logger.Info("cron stopped")
	}
}
