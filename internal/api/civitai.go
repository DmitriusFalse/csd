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

type ModelInfoFetcher interface {
	FetchModelInfo(modelVersionID int, apiKey string) (*models.CivitaiModelResponse, error)
}

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
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
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
		return nil, models.ClassifyHTTPError(0, "Failed to create request: "+err.Error())
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if models.IsNetworkError(err) {
			return nil, models.NewAPIError(
				models.ErrCodeNetwork,
				fmt.Sprintf("Сетевая ошибка при подключении к Civitai: %s", err.Error()),
				0, true,
			)
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, models.ClassifyHTTPError(resp.StatusCode, string(body))
	}

	var tRPCResp tRPCResponse
	if err := json.Unmarshal(body, &tRPCResp); err != nil {
		return nil, models.NewAPIError(
			models.ErrCodeServerError,
			"Ошибка парсинга ответа от Civitai: "+err.Error(),
			resp.StatusCode, false,
		)
	}

	if tRPCResp.Error != nil {
		return nil, models.NewAPIError(
			models.ErrCodeServerError,
			"API Civitai вернул ошибку: "+tRPCResp.Error.Message,
			resp.StatusCode, false,
		)
	}

	var data tRPCModelVersion
	if err := json.Unmarshal(tRPCResp.Result.Data, &data); err != nil {
		return nil, models.NewAPIError(
			models.ErrCodeServerError,
			"Ошибка парсинга данных модели: "+err.Error(),
			0, false,
		)
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
