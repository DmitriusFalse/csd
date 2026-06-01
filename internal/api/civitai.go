package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/DmitriusFalse/csd/internal/models"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type CivitaiClient struct {
	httpClient *http.Client
}

func NewClient() *CivitaiClient {
	return &CivitaiClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type tRPCResponse struct {
	Result struct {
		Data json.RawMessage `json:"data"`
	} `json:"result"`
}

type tRPCModelVersion struct {
	ID          int                      `json:"id"`
	Name        string                   `json:"name"`
	Type        string                   `json:"type"`
	Nsfw        bool                     `json:"nsfw"`
	NsfwLevel   string                   `json:"nsfwLevel"`
	ModelID     int                      `json:"modelId"`
	ModelName   string                   `json:"modelName"`
	BaseModel   string                   `json:"baseModel"`
	Description string                   `json:"description"`
	Tags        []models.CivitaiTag      `json:"tags"`
	Creator     models.CivitaiCreator    `json:"creator"`
	Files       []models.CivitaiFile     `json:"files"`
	Images      []models.CivitaiPreviewImage `json:"images"`
}

func (c *CivitaiClient) FetchModelInfo(modelVersionID int, apiKey string) (*models.CivitaiModelResponse, error) {
	inputStr := fmt.Sprintf(`{"json":{"id":%d,"authed":%t}}`, modelVersionID, apiKey != "")
	url := fmt.Sprintf("https://civitai.com/api/trpc/modelVersion.getById?input=%s", inputStr)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var tRPCResp tRPCResponse
	if err := json.Unmarshal(body, &tRPCResp); err != nil {
		return nil, fmt.Errorf("parse tRPC: %w", err)
	}

	var data tRPCModelVersion
	if err := json.Unmarshal(tRPCResp.Result.Data, &data); err != nil {
		return nil, fmt.Errorf("parse data: %w", err)
	}

	logger.Log.Debug("Fetched model info",
		zap.Int("versionID", data.ID),
		zap.String("name", data.Name),
		zap.String("type", data.Type),
	)

	result := &models.CivitaiModelResponse{
		ID:            data.ID,
		Name:          data.Name,
		Type:          data.Type,
		Nsfw:          data.Nsfw,
		NsfwLevel:     data.NsfwLevel,
		ModelID:       data.ModelID,
		ModelName:     data.ModelName,
		BaseModel:     data.BaseModel,
		Description:   data.Description,
		Tags:          data.Tags,
		Creator:       data.Creator,
		Files:         data.Files,
		PreviewImages: data.Images,
	}

	return result, nil
}

func ParseModelType(raw string) models.ModelType {
	switch strings.ToLower(raw) {
	case "checkpoint":
		return models.ModelTypeCheckpoint
	case "lora":
		return models.ModelTypeLORA
	case "locon":
		return models.ModelTypeLoCon
	case "vae":
		return models.ModelTypeVAE
	case "textualinversion", "embedding":
		return models.ModelTypeTextualInversion
	case "controlnet":
		return models.ModelTypeControlNet
	default:
		return models.ModelType(raw)
	}
}
