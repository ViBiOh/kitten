package kitten

import (
	"context"

	"github.com/ViBiOh/httputils/v4/pkg/model"
)

func (a Service) increaseServed(ctx context.Context) {
	if model.IsNil(a.servedMetric) {
		return
	}

	a.servedMetric.Add(ctx, 1)
}

func (a Service) increaseCached(ctx context.Context) {
	if model.IsNil(a.cachedMetric) {
		return
	}

	a.cachedMetric.Add(ctx, 1)
}
