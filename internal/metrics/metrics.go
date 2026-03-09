package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/abatalev/covaggregator/internal/coverage"
)

var (
	CoverageInstruction = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jacoco_coverage_instruction_percentage",
			Help: "JaCoCo instruction coverage percentage (0-100)",
		},
		[]string{"service", "version", "type"},
	)

	CoverageLine = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jacoco_coverage_line_percentage",
			Help: "JaCoCo line coverage percentage (0-100)",
		},
		[]string{"service", "version", "type"},
	)

	CoverageBranch = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jacoco_coverage_branch_percentage",
			Help: "JaCoCo branch coverage percentage (0-100)",
		},
		[]string{"service", "version", "type"},
	)

	InstructionsCovered = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jacoco_instructions_covered",
			Help: "Number of covered instructions",
		},
		[]string{"service", "version", "type"},
	)

	InstructionsMissed = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jacoco_instructions_missed",
			Help: "Number of missed instructions",
		},
		[]string{"service", "version", "type"},
	)

	LinesCovered = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jacoco_lines_covered",
			Help: "Number of covered lines",
		},
		[]string{"service", "version", "type"},
	)

	LinesMissed = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jacoco_lines_missed",
			Help: "Number of missed lines",
		},
		[]string{"service", "version", "type"},
	)

	InstancesTotal = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jacoco_instances_total",
			Help: "Total number of JVM instances",
		},
		[]string{"service"},
	)

	CollectionSuccess = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jacoco_collection_success_total",
			Help: "Total number of successful collections",
		},
		[]string{"service", "version"},
	)

	CollectionFail = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jacoco_collection_fail_total",
			Help: "Total number of failed collections",
		},
		[]string{"service", "version"},
	)
)

func UpdateCoverageMetrics(service, version, reportType string, cov *coverage.Coverage) {
	if cov == nil {
		return
	}

	labels := prometheus.Labels{
		"service": service,
		"version": version,
		"type":    reportType,
	}

	if cov.Instruction != nil {
		CoverageInstruction.With(labels).Set(cov.Instruction.Percent)
		InstructionsCovered.With(labels).Set(float64(cov.Instruction.Covered))
		InstructionsMissed.With(labels).Set(float64(cov.Instruction.Missed))
	}

	if cov.Line != nil {
		CoverageLine.With(labels).Set(cov.Line.Percent)
		LinesCovered.With(labels).Set(float64(cov.Line.Covered))
		LinesMissed.With(labels).Set(float64(cov.Line.Missed))
	}

	if cov.Branch != nil {
		CoverageBranch.With(labels).Set(cov.Branch.Percent)
	}
}
