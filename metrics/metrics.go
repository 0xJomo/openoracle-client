package metrics

import (
	"github.com/Layr-Labs/eigensdk-go/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics interface {
	metrics.Metrics
	IncNumTasksReceived()
	IncNumTasksAcceptedByAggregator()
	// This metric would either need to be tracked by the aggregator itself,
	// or we would need to write a collector that queries onchain for this info
	// AddPercentageStakeSigned(percentage float64)
	IncNumRequestToNasdaq()
	IncNumErrorFromNasdaq()
	IncNumRequestToCnbc()
	IncNumErrorFromCnbc()
	IncNumRequestToTradingview()
	IncNumErrorFromTradingview()
}

// AvsMetrics contains instrumented metrics that should be incremented by the avs node using the methods below
type AvsAndEigenMetrics struct {
	metrics.Metrics
	numTasksReceived prometheus.Counter
	// if numSignedTaskResponsesAcceptedByAggregator != numTasksReceived, then there is a bug
	numSignedTaskResponsesAcceptedByAggregator prometheus.Counter
	numRequestToNasdaq                         prometheus.Counter
	numErrorFromNasdaq                         prometheus.Counter
	numRequestToCnbc                           prometheus.Counter
	numErrorFromCnbc                           prometheus.Counter
	numRequestToTradingview                    prometheus.Counter
	numErrorFromTradingview                    prometheus.Counter
}

const openOracleNamespace = "openoracle"

func NewAvsAndEigenMetrics(avsName string, eigenMetrics *metrics.EigenMetrics, reg prometheus.Registerer) *AvsAndEigenMetrics {
	return &AvsAndEigenMetrics{
		Metrics: eigenMetrics,
		numTasksReceived: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: openOracleNamespace,
				Name:      "num_tasks_received",
				Help:      "The number of tasks received by reading from the avs service manager contract",
			}),
		numSignedTaskResponsesAcceptedByAggregator: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: openOracleNamespace,
				Name:      "num_signed_task_responses_accepted_by_aggregator",
				Help:      "The number of signed task responses accepted by the aggregator",
			}),
		numRequestToNasdaq: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: openOracleNamespace,
				Name:      "num_request_sent_to_nasdaq",
				Help:      "The number of requests sent to nasdaq to get task data",
			}),
		numErrorFromNasdaq: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: openOracleNamespace,
				Name:      "num_error_received_from_nasdaq",
				Help:      "The number of errors received from nasdaq when requesting task data",
			}),
		numRequestToCnbc: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: openOracleNamespace,
				Name:      "num_request_sent_to_cnbc",
				Help:      "The number of requests sent to cnbc to get task data",
			}),
		numErrorFromCnbc: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: openOracleNamespace,
				Name:      "num_error_received_from_cnbc",
				Help:      "The number of errors received from cnbc when requesting task data",
			}),
		numRequestToTradingview: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: openOracleNamespace,
				Name:      "num_request_sent_to_tradingview",
				Help:      "The number of requests sent to tradingview to get task data",
			}),
		numErrorFromTradingview: promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: openOracleNamespace,
				Name:      "num_error_received_from_tradingview",
				Help:      "The number of errors received from tradingview when requesting task data",
			}),
	}
}

func (m *AvsAndEigenMetrics) IncNumTasksReceived() {
	m.numTasksReceived.Inc()
}

func (m *AvsAndEigenMetrics) IncNumTasksAcceptedByAggregator() {
	m.numSignedTaskResponsesAcceptedByAggregator.Inc()
}

func (m *AvsAndEigenMetrics) IncNumRequestToNasdaq() {
	m.numRequestToNasdaq.Inc()
}

func (m *AvsAndEigenMetrics) IncNumErrorFromNasdaq() {
	m.numErrorFromNasdaq.Inc()
}

func (m *AvsAndEigenMetrics) IncNumRequestToCnbc() {
	m.numRequestToCnbc.Inc()
}

func (m *AvsAndEigenMetrics) IncNumErrorFromCnbc() {
	m.numErrorFromCnbc.Inc()
}

func (m *AvsAndEigenMetrics) IncNumRequestToTradingview() {
	m.numRequestToTradingview.Inc()
}

func (m *AvsAndEigenMetrics) IncNumErrorFromTradingview() {
	m.numErrorFromTradingview.Inc()
}
