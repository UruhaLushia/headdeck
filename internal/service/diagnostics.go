package service

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	hc "github.com/UruhaLushia/headdeck/internal/headscale/config"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

var metricLine = regexp.MustCompile(`^([a-zA-Z_:][\w:]*)(?:\{.*\})?\s+(\S+)(?:\s+\S+)?$`)

func parseMetricValues(content string) map[string]float64 {
	values := make(map[string]float64)
	for line := range strings.SplitSeq(content, "\n") {
		match := metricLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		switch match[1] {
		case "go_goroutines", "process_resident_memory_bytes", "go_memstats_heap_alloc_bytes",
			"process_cpu_seconds_total", "process_open_fds", "http_requests_inflight",
			"headscale_nodestore_nodes", "headscale_nodestore_queue_depth", "process_start_time_seconds":
			value, err := strconv.ParseFloat(match[2], 64)
			if err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) {
				values[match[1]] += value
			}
		}
	}
	return values
}

func (s *Service) Diagnostic(ctx context.Context, section string) (*protocol.Diagnostic, error) {
	var path string
	switch section {
	case "metrics":
		path = "/metrics"
	case "overview", "batcher", "nodestore", "routes", "derp", "policy-manager":
		path = "/debug/" + section
	case "goroutine":
		path = "/debug/pprof/goroutine?debug=1"
	default:
		return nil, protocol.NewError(protocol.CodeBadRequest, "unsupported diagnostic section")
	}
	s.diagnosticMu.Lock()
	defer s.diagnosticMu.Unlock()
	if cached := s.diagnostics[section]; cached != nil && time.Since(cached.CollectedAt) < 10*time.Second {
		return cached, nil
	}
	base, err := hc.DiagnosticsUpstream(s.headscaleConfigPath)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeClientConfig, "Headscale diagnostics: %v", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	data, err := s.hs.ReadDiagnostic(ctx, base, path)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeHeadscaleUnavailable, "%v", err)
	}
	format := "text"
	if json.Valid(data) {
		format = "json"
	}
	out := &protocol.Diagnostic{Section: section, CollectedAt: time.Now().UTC(), Format: format}
	if section == "metrics" {
		values := parseMetricValues(string(data))
		start, hasStart := values["process_start_time_seconds"]
		if n := len(s.metricSamples); n > 0 && hasStart {
			if previous, ok := s.metricSamples[n-1].Values["process_start_time_seconds"]; ok && previous != start {
				s.metricSamples = nil
			}
		}
		samples := make([]protocol.MetricSample, 0, 120)
		for _, sample := range s.metricSamples {
			if out.CollectedAt.Sub(sample.Time) <= time.Hour {
				samples = append(samples, sample)
			}
		}
		samples = append(samples, protocol.MetricSample{Time: out.CollectedAt, Values: values})
		if len(samples) > 120 {
			samples = samples[len(samples)-120:]
		}
		s.metricSamples = samples
		out.Samples = samples
	} else {
		out.Content = string(data)
	}
	if s.diagnostics == nil {
		s.diagnostics = make(map[string]*protocol.Diagnostic)
	}
	s.diagnostics[section] = out
	return out, nil
}
