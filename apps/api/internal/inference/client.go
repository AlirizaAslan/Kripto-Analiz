package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"pulsealpha/api/internal/domain"
)

type Client struct {
	endpoint string
	model    string
	http     *http.Client
}

type Request struct {
	Symbol   string             `json:"symbol"`
	Market   domain.Market      `json:"market"`
	Features map[string]float64 `json:"features"`
}

type Response struct {
	UpProbability      float64                     `json:"upProbability"`
	DownProbability    float64                     `json:"downProbability"`
	NeutralProbability float64                     `json:"neutralProbability"`
	PredictedDirection string                      `json:"predictedDirection"`
	ConfidenceScore    float64                     `json:"confidenceScore"`
	RiskLabel          domain.RiskLabel            `json:"riskLabel"`
	SignalLabel        domain.SignalLabel          `json:"signalLabel"`
	SignalQuality      domain.SignalQuality        `json:"signalQuality"`
	HistoricalHitRate  float64                     `json:"historicalHitRate"`
	ModelVersion       string                      `json:"modelVersion"`
	Explanation        string                      `json:"explanation"`
	ModelComponents    []domain.ModelComponent     `json:"modelComponents"`
	TopFeatures        []domain.FeatureAttribution `json:"topFeatures"`
}

func New(endpoint, model string) *Client {
	return &Client{
		endpoint: endpoint,
		model:    model,
		http: &http.Client{
			Timeout: 1200 * time.Millisecond,
		},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.endpoint != ""
}

func (c *Client) Model() string {
	if c == nil || c.model == "" {
		return "depth-fallback-v1"
	}
	return c.model
}

func (c *Client) Predict(ctx context.Context, input Request) (Response, error) {
	if !c.Enabled() {
		return Response{}, fmt.Errorf("inference endpoint not configured")
	}

	body, err := json.Marshal(map[string]any{
		"model":    c.Model(),
		"symbol":   input.Symbol,
		"market":   input.Market,
		"features": input.Features,
	})
	if err != nil {
		return Response{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/predict", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("inference status %d", resp.StatusCode)
	}

	var output Response
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return Response{}, err
	}

	return output, nil
}
