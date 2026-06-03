package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/DmitriusFalse/csd/internal/logger"
	"github.com/DmitriusFalse/csd/internal/models"
	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	logger.Log = zap.NewNop()
	os.Exit(m.Run())
}

func TestParseModelTypeCheckpoint(t *testing.T) {
	if got := ParseModelType("checkpoint"); got != models.ModelTypeCheckpoint {
		t.Errorf("expected Checkpoint, got %s", got)
	}
	if got := ParseModelType("Checkpoint"); got != models.ModelTypeCheckpoint {
		t.Errorf("expected Checkpoint (case), got %s", got)
	}
}

func TestParseModelTypeLora(t *testing.T) {
	if got := ParseModelType("lora"); got != models.ModelTypeLORA {
		t.Errorf("expected LORA, got %s", got)
	}
}

func TestParseModelTypeLoCon(t *testing.T) {
	if got := ParseModelType("locon"); got != models.ModelTypeLoCon {
		t.Errorf("expected LoCon, got %s", got)
	}
}

func TestParseModelTypeVAE(t *testing.T) {
	if got := ParseModelType("vae"); got != models.ModelTypeVAE {
		t.Errorf("expected VAE, got %s", got)
	}
}

func TestParseModelTypeTextualInversion(t *testing.T) {
	if got := ParseModelType("textualinversion"); got != models.ModelTypeTextualInversion {
		t.Errorf("expected TextualInversion, got %s", got)
	}
	if got := ParseModelType("embedding"); got != models.ModelTypeTextualInversion {
		t.Errorf("expected TextualInversion for embedding, got %s", got)
	}
}

func TestParseModelTypeControlNet(t *testing.T) {
	if got := ParseModelType("controlnet"); got != models.ModelTypeControlNet {
		t.Errorf("expected Controlnet, got %s", got)
	}
}

func TestParseModelTypeUnknown(t *testing.T) {
	got := ParseModelType("unknown_type")
	if string(got) != "unknown_type" {
		t.Errorf("expected 'unknown_type', got '%s'", got)
	}
}

func TestFetchModelInfoSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer token, got: %s", r.Header.Get("Authorization"))
		}

		resp := tRPCResponse{}
		resp.Result.Data = json.RawMessage(`{
			"id": 123,
			"name": "Test Version",
			"type": "Checkpoint",
			"nsfw": false,
			"modelId": 456,
			"modelName": "Test Model",
			"baseModel": "SDXL",
			"files": [{"id": 1, "name": "model.safetensors", "sizeKB": 5000}],
			"images": [{"url": "https://example.com/img.png", "type": "image"}]
		}`)
		respBytes, _ := json.Marshal(resp)
		w.Write(respBytes)
	}))
	defer server.Close()

	client := NewTestClient(server.URL)
	result, err := client.FetchModelInfo(123, "test-key")
	if err != nil {
		t.Fatalf("FetchModelInfo failed: %v", err)
	}

	if result.ID != 123 {
		t.Errorf("expected ID 123, got %d", result.ID)
	}
	if result.ModelName != "Test Model" {
		t.Errorf("expected ModelName 'Test Model', got '%s'", result.ModelName)
	}
	if result.Type != "Checkpoint" {
		t.Errorf("expected Type 'Checkpoint', got '%s'", result.Type)
	}
	if len(result.Files) != 1 || result.Files[0].ID != 1 {
		t.Errorf("unexpected files: %+v", result.Files)
	}
}

func TestFetchModelInfoAuthPropagated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		resp := tRPCResponse{}
		resp.Result.Data = json.RawMessage(`{"id":1,"name":"M","type":"LORA","modelId":1,"modelName":"M"}`)
		respBytes, _ := json.Marshal(resp)
		w.Write(respBytes)
	}))
	defer server.Close()

	client := NewTestClient(server.URL)
	_, err := client.FetchModelInfo(1, "secret-key")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestFetchModelInfoNoAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("expected no Authorization header with empty key")
		}
		resp := tRPCResponse{}
		resp.Result.Data = json.RawMessage(`{"id":1,"name":"M","type":"LORA","modelId":1,"modelName":"M"}`)
		respBytes, _ := json.Marshal(resp)
		w.Write(respBytes)
	}))
	defer server.Close()

	client := NewTestClient(server.URL)
	_, err := client.FetchModelInfo(1, "")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestFetchModelInfoHTTP404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewTestClient(server.URL)
	_, err := client.FetchModelInfo(1, "")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestFetchModelInfoHTTP429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewTestClient(server.URL)
	_, err := client.FetchModelInfo(1, "")
	if err == nil {
		t.Fatal("expected error for 429")
	}
}

func TestFetchModelInfoInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := NewTestClient(server.URL)
	_, err := client.FetchModelInfo(1, "")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFetchModelInfoTRPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":{"message":"model not found","code":"NOT_FOUND"}}`))
	}))
	defer server.Close()

	client := NewTestClient(server.URL)
	_, err := client.FetchModelInfo(1, "")
	if err == nil {
		t.Fatal("expected error for tRPC error")
	}
}

func TestFetchModelInfoAllFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := tRPCResponse{}
		resp.Result.Data = json.RawMessage(`{
			"id": 999,
			"name": "Full Version",
			"type": "LORA",
			"nsfw": true,
			"nsfwLevel": "explicit",
			"modelId": 888,
			"modelName": "Full Model",
			"baseModel": "SD 1.5",
			"description": "A test model",
			"tags": [{"name":"style"},{"name":"character"}],
			"creator": {"username":"testuser","userId":42},
			"files": [{"id":10,"name":"file1.safetensors","sizeKB":1000}],
			"images": [{"url":"https://ex.com/a.png","type":"image"}]
		}`)
		respBytes, _ := json.Marshal(resp)
		w.Write(respBytes)
	}))
	defer server.Close()

	client := NewTestClient(server.URL)
	result, err := client.FetchModelInfo(999, "")
	if err != nil {
		t.Fatalf("FetchModelInfo failed: %v", err)
	}

	if result.ID != 999 || result.ModelID != 888 {
		t.Errorf("bad IDs: %+v", result)
	}
	if result.Nsfw != true {
		t.Error("expected NSFW true")
	}
	if result.BaseModel != "SD 1.5" {
		t.Errorf("bad BaseModel: %s", result.BaseModel)
	}
	if len(result.Tags) != 2 || result.Tags[0].Name != "style" {
		t.Errorf("bad tags: %+v", result.Tags)
	}
	if result.Creator.Username != "testuser" {
		t.Errorf("bad creator: %+v", result.Creator)
	}
	if len(result.Files) != 1 || result.Files[0].Name != "file1.safetensors" {
		t.Errorf("bad files: %+v", result.Files)
	}
	if len(result.PreviewImages) != 1 || result.PreviewImages[0].URL != "https://ex.com/a.png" {
		t.Errorf("bad images: %+v", result.PreviewImages)
	}
}
