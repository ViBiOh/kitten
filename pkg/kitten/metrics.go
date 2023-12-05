package kitten

import (
	"context"

	"github.com/ViBiOh/httputils/v4/pkg/model"
)

func (s Service) increaseServed(ctx context.Context) {
	if model.IsNil(s.servedMetric) {
		return
	}

	s.servedMetric.Add(ctx, 1)
}

func (s Service) increaseCached(ctx context.Context) {
	if model.IsNil(s.cachedMetric) {
		return
	}

	s.cachedMetric.Add(ctx, 1)
}
