package config

import (
	"path/filepath"

	"github.com/DmitriusFalse/csd/internal/models"
)

var baseModelFolders = map[string]string{
	"SD 1.5":  "SD15",
	"SD 2.0":  "SD20",
	"SDXL":    "SDXL",
	"SDXL 1.0": "SDXL",
	"Pony":    "Pony",
	"Flux":    "Flux",
	"Flux.1":  "Flux",
	"SD3":     "SD3",
	"SD3.5":   "SD3",
}

func resolveBaseModelFolder(baseModel string) string {
	if folder, ok := baseModelFolders[baseModel]; ok {
		return folder
	}
	return baseModel
}

func GetSavePath(root string, modelType models.ModelType, baseModel string, isNSFW bool, nsfwSuffix string) string {
	var typeFolder string
	switch modelType {
	case models.ModelTypeCheckpoint:
		typeFolder = "Stable-diffusion"
	case models.ModelTypeLORA, models.ModelTypeLoCon:
		typeFolder = "Lora"
	case models.ModelTypeVAE:
		typeFolder = "VAE"
	case models.ModelTypeTextualInversion:
		typeFolder = "embeddings"
	case models.ModelTypeControlNet:
		typeFolder = "ControlNet"
	default:
		typeFolder = "Other"
	}

	baseFolder := resolveBaseModelFolder(baseModel)

	path := filepath.Join(root, typeFolder, baseFolder)
	if isNSFW {
		path += nsfwSuffix
	}

	return path
}
