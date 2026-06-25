// Package metrics — adapter that exposes *Collector as a
// query.MetricsProvider.
//
// We declare the adapter here (not in package query) to avoid an
// import cycle: query -> (uses MetricsProvider interface); metrics ->
// (implements it via this adapter). The adapter is in package metrics
// so it can see the unexported Rate type.
package metrics

import "github.com/quarkloop/quark/server/internal/query"

// AsProvider wraps the Collector to satisfy query.MetricsProvider.
//
// The Collector already has a GetRate(namespace) method, but it
// returns *Rate (metrics-package type). query.MetricsProvider
// expects *query.RateSnapshot. This adapter converts.
func (c *Collector) AsProvider() query.MetricsProvider {
	return &collectorAdapter{c: c}
}

type collectorAdapter struct{ c *Collector }

func (a *collectorAdapter) GetRate(namespace string) *query.RateSnapshot {
	r := a.c.GetRate(namespace)
	if r == nil {
		return nil
	}
	return &query.RateSnapshot{
		MessagesPublished: r.MessagesPublished,
		MessagesReceived:  r.MessagesReceived,
		Errors:            r.Errors,
		CPUTimeNanos:      r.CPUTimeNanos,
		PublishRate:       r.PublishRate,
		ReceiveRate:       r.ReceiveRate,
		ErrorRate:         r.ErrorRate,
		CPUPercent:        r.CPUPercent,
	}
}
