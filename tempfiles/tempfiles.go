package tempfiles

import (
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/lesovsky/noisia"
	"time"
)

const (
	queryCreateTable = `CREATE TABLE IF NOT EXISTS _noisia_tempfiles_workload (a text, b text, c text, d text, e text, f text, g text, h text, i text, j text, k text, l text, m text, n text, o text, p text, q text, r text, s text, t text, u text, v text, w text, x text, y text, z text)`
	queryLoadData    = `INSERT INTO _noisia_tempfiles_workload (a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,v,w,x,y,z) SELECT random()::text,random()::text,
random()::text,random()::text,random()::text,random()::text,random()::text,random()::text,random()::text,random()::text,random()::text,
random()::text,random()::text,random()::text,random()::text,random()::text,random()::text,random()::text,random()::text,random()::text,
random()::text,random()::text,random()::text,random()::text,random()::text,random()::text from generate_series(1,$1)`
	querySelectData = `SELECT a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,v,w,x,y,z FROM _noisia_tempfiles_workload GROUP BY z,y,x,w,v,u,t,s,r,q,p,o,n,m,l,k,j,i,h,g,f,e,d,c,b,a ORDER BY a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,v,w,x,y,z DESC`
)

// Config defines configuration settings for temp files workload
type Config struct {
	// PostgresConninfo defines connections string used for connecting to Postgres.
	PostgresConninfo string
	// Jobs defines how many workers should be created for producing temp files.
	Jobs uint16
	// TempFilesRate defines rate interval for queries executing.
	TempFilesRate int
	// TempFilesScaleFactor defines multiplier for amount of fixtures in temporary table.
	TempFilesScaleFactor int
}

type workload struct {
	config *Config
	pool   *pgxpool.Pool
}

func NewWorkload(config *Config) noisia.Workload {
	return &workload{config, &pgxpool.Pool{}}
}

func (w *workload) Run(ctx context.Context) error {
	pool, err := pgxpool.Connect(ctx, w.config.PostgresConninfo)
	if err != nil {
		return err
	}
	w.pool = pool
	defer w.pool.Close()

	// Prepare temp tables and fixtures for workload.
	if err := w.prepare(ctx); err != nil {
		return err
	}

	// Cleanup in the end.
	defer func() { _ = w.cleanup(ctx) }()

	// calculate inter-query interval for rate throttling
	interval := 1000000000 / int64(w.config.TempFilesRate)

	// keep specified number of workers using channel - run new workers until there is any free slot
	guard := make(chan struct{}, w.config.Jobs)
	for {
		select {
		// run workers only when it's possible to write into channel (channel is limited by number of jobs)
		case guard <- struct{}{}:
			go func() {
				// Don't care about errors.
				_, _ = pool.Exec(ctx, querySelectData)
				time.Sleep(time.Duration(interval) * time.Nanosecond)

				<-guard
			}()
		case <-ctx.Done():
			//log.Info("exit signaled, stop temp files workload")
			return nil
		}
	}
}

func (w *workload) prepare(ctx context.Context) error {
	_, err := w.pool.Exec(ctx, queryCreateTable)
	if err != nil {
		return err
	}
	_, err = w.pool.Exec(ctx, queryLoadData, 1000*w.config.TempFilesScaleFactor)
	if err != nil {
		return err
	}
	return nil
}

func (w *workload) cleanup(ctx context.Context) error {
	_, err := w.pool.Exec(ctx, "DROP TABLE IF EXISTS _noisia_tempfiles_workload")
	if err != nil {
		return err
	}
	return nil
}
