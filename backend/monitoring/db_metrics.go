package monitoring

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// DBPoolMetrics exports database/sql pool state. Prometheus alerts can detect
// pool saturation from open/max and wait_count/wait_duration trends.
type DBPoolMetrics struct {
	open, inUse, idle, maxOpen, waitCount, waitDuration, idleClosed, lifetimeClosed prometheus.Gauge
	db                                                                              *sql.DB
}

func NewDBPoolMetrics(db *gorm.DB) (*DBPoolMetrics, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	return &DBPoolMetrics{
		db:             sqlDB,
		open:           prometheus.NewGauge(prometheus.GaugeOpts{Name: "db_pool_open_connections", Help: "Open database connections."}),
		inUse:          prometheus.NewGauge(prometheus.GaugeOpts{Name: "db_pool_in_use_connections", Help: "Database connections currently in use."}),
		idle:           prometheus.NewGauge(prometheus.GaugeOpts{Name: "db_pool_idle_connections", Help: "Idle database connections."}),
		maxOpen:        prometheus.NewGauge(prometheus.GaugeOpts{Name: "db_pool_max_open_connections", Help: "Configured database maximum open connections."}),
		waitCount:      prometheus.NewGauge(prometheus.GaugeOpts{Name: "db_pool_wait_count", Help: "Cumulative waits for a database connection."}),
		waitDuration:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "db_pool_wait_duration_seconds", Help: "Cumulative time spent waiting for a database connection."}),
		idleClosed:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "db_pool_idle_closed_total", Help: "Connections closed due to idle limits."}),
		lifetimeClosed: prometheus.NewGauge(prometheus.GaugeOpts{Name: "db_pool_lifetime_closed_total", Help: "Connections closed due to lifetime limits."}),
	}, nil
}

func (m *DBPoolMetrics) Start(ctx context.Context) {
	prometheus.MustRegister(m.open, m.inUse, m.idle, m.maxOpen, m.waitCount, m.waitDuration, m.idleClosed, m.lifetimeClosed)
	m.collect()
	ticker := time.NewTicker(15 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.collect()
			}
		}
	}()
}

func (m *DBPoolMetrics) collect() {
	stats := m.db.Stats()
	m.open.Set(float64(stats.OpenConnections))
	m.inUse.Set(float64(stats.InUse))
	m.idle.Set(float64(stats.Idle))
	m.maxOpen.Set(float64(stats.MaxOpenConnections))
	m.waitCount.Set(float64(stats.WaitCount))
	m.waitDuration.Set(stats.WaitDuration.Seconds())
	m.idleClosed.Set(float64(stats.MaxIdleClosed + stats.MaxIdleTimeClosed))
	m.lifetimeClosed.Set(float64(stats.MaxLifetimeClosed))
}
