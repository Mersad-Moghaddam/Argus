package victoriametrics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"argus/internal/observability"
)

// Writer imports already-sanitized samples using VictoriaMetrics' documented
// JSON-line import endpoint. It is intentionally not a general purpose remote
// write client: the OTLP boundary owns allowlisting and cardinality controls.
type Writer struct {
	importURL string
	client    *http.Client
}

func NewWriter(baseURL string, timeout time.Duration) (*Writer, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid VictoriaMetrics URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("VictoriaMetrics URL must use HTTP or HTTPS")
	}
	return &Writer{importURL: parsed.String() + "/api/v1/import", client: &http.Client{Timeout: timeout}}, nil
}

type importLine struct {
	Metric     map[string]string `json:"metric"`
	Values     []float64         `json:"values"`
	Timestamps []int64           `json:"timestamps"`
}

func (w *Writer) Write(ctx context.Context, samples []observability.MetricSample) error {
	if len(samples) == 0 {
		return nil
	}
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	for _, sample := range samples {
		if sample.Name == "" || sample.Timestamp.IsZero() {
			return fmt.Errorf("invalid metric sample")
		}
		metric := make(map[string]string, len(sample.Labels)+1)
		metric["__name__"] = sample.Name
		for key, value := range sample.Labels {
			metric[key] = value
		}
		if err := encoder.Encode(importLine{Metric: metric, Values: []float64{sample.Value}, Timestamps: []int64{sample.Timestamp.UnixMilli()}}); err != nil {
			return fmt.Errorf("encode VictoriaMetrics import: %w", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.importURL, bytes.NewReader(body.Bytes()))
	if err != nil {
		return fmt.Errorf("create VictoriaMetrics import: %w", err)
	}
	req.Header.Set("Content-Type", "application/stream+json")
	response, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send VictoriaMetrics import: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := bufio.NewReader(io.LimitReader(response.Body, 1024)).ReadString('\n')
		return fmt.Errorf("VictoriaMetrics import returned %d: %s", response.StatusCode, strings.TrimSpace(message))
	}
	return nil
}
