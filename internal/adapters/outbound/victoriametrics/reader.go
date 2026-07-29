package victoriametrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
)

const httpDurationCount = "argus_http_server_request_duration_seconds_count"
const httpDurationBucket = "argus_http_server_request_duration_seconds_bucket"

// Reader issues only fixed, server-generated MetricsQL expressions against
// the internal metrics backend. Project identifiers are numeric and SLO
// configuration is validated before it is persisted, so no user query text is
// accepted at this boundary.
type Reader struct {
	queryURL string
	client   *http.Client
}

func NewReader(baseURL string, timeout time.Duration) (*Reader, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid VictoriaMetrics URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("VictoriaMetrics URL must use HTTP or HTTPS")
	}
	return &Reader{queryURL: parsed.String() + "/api/v1/query", client: &http.Client{Timeout: timeout}}, nil
}

func (r *Reader) AggregateSLO(ctx context.Context, definition models.SLODefinition, now time.Time) (models.SLOMetricAggregate, error) {
	if definition.ProjectID <= 0 || definition.WindowSeconds <= 0 {
		return models.SLOMetricAggregate{}, fmt.Errorf("invalid SLO definition")
	}
	selector := fmt.Sprintf(`%s{argus_project_id=%q}`, httpDurationCount, strconv.FormatInt(definition.ProjectID, 10))
	window := strconv.Itoa(definition.WindowSeconds) + "s"
	total, err := r.scalar(ctx, fmt.Sprintf("sum(increase(%s[%s]))", selector, window), now)
	if err != nil {
		return models.SLOMetricAggregate{}, err
	}
	if total == nil {
		return models.SLOMetricAggregate{Provenance: "victoriametrics/http-server"}, nil
	}
	if *total < 0 || math.IsNaN(*total) || math.IsInf(*total, 0) {
		return models.SLOMetricAggregate{}, fmt.Errorf("invalid total from VictoriaMetrics")
	}
	good := *total
	if definition.SLIKind == string(domain.SLIAvailability) {
		errors, err := r.scalar(ctx, fmt.Sprintf(`sum(increase(%s{argus_project_id=%q,http_status_code=~"5.."}[%s]))`, httpDurationCount, strconv.FormatInt(definition.ProjectID, 10), window), now)
		if err != nil {
			return models.SLOMetricAggregate{}, err
		}
		if errors != nil {
			good -= *errors
		}
	} else if definition.SLIKind == string(domain.SLILatency) {
		threshold := strconv.FormatFloat(float64(definition.LatencyThresholdMS)/1000, 'f', -1, 64)
		share, err := r.scalar(ctx, fmt.Sprintf("histogram_share(%s, sum(increase(%s{argus_project_id=%q}[%s])) by (le))", threshold, httpDurationBucket, strconv.FormatInt(definition.ProjectID, 10), window), now)
		if err != nil {
			return models.SLOMetricAggregate{}, err
		}
		if share == nil {
			return models.SLOMetricAggregate{Provenance: "victoriametrics/http-server"}, nil
		}
		if *share < 0 || *share > 1 || math.IsNaN(*share) || math.IsInf(*share, 0) {
			return models.SLOMetricAggregate{}, fmt.Errorf("invalid histogram share from VictoriaMetrics")
		}
		good = math.Round(*total * *share)
	} else {
		return models.SLOMetricAggregate{}, fmt.Errorf("unsupported SLI kind")
	}
	if good < 0 || good > *total {
		return models.SLOMetricAggregate{}, fmt.Errorf("invalid good-event aggregate")
	}
	freshness, err := r.scalar(ctx, fmt.Sprintf("max(timestamp(%s))", selector), now)
	if err != nil {
		return models.SLOMetricAggregate{}, err
	}
	result := models.SLOMetricAggregate{GoodEvents: int64(math.Round(good)), TotalEvents: int64(math.Round(*total)), Provenance: "victoriametrics/http-server"}
	if freshness != nil && *freshness > 0 {
		observed := time.Unix(int64(*freshness), 0).UTC()
		result.ObservedAt = &observed
	}
	return result, nil
}

type queryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Value []json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func (r *Reader) scalar(ctx context.Context, query string, now time.Time) (*float64, error) {
	form := url.Values{"query": {query}, "time": {strconv.FormatFloat(float64(now.UnixNano())/float64(time.Second), 'f', 3, 64)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.queryURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("VictoriaMetrics query returned %d", response.StatusCode)
	}
	var payload queryResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode VictoriaMetrics query: %w", err)
	}
	if payload.Status != "success" || payload.Data.ResultType != "vector" {
		return nil, fmt.Errorf("unexpected VictoriaMetrics query response")
	}
	if len(payload.Data.Result) == 0 {
		return nil, nil
	}
	if len(payload.Data.Result) != 1 || len(payload.Data.Result[0].Value) != 2 {
		return nil, fmt.Errorf("VictoriaMetrics query returned multiple values")
	}
	var encoded string
	if err = json.Unmarshal(payload.Data.Result[0].Value[1], &encoded); err != nil {
		return nil, err
	}
	value, err := strconv.ParseFloat(encoded, 64)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
