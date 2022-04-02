package kitten

import "github.com/ViBiOh/httputils/v4/pkg/model"

func (a App) increaseServed() {
	if model.IsNil(a.servedMetric) {
		return
	}

	a.servedMetric.Inc()
}

func (a App) increaseCached() {
	if model.IsNil(a.cachedMetric) {
		return
	}

	a.cachedMetric.Inc()
}
