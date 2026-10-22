package integration

import "github.com/pramaan-oss/cold-email/internal/models"

// Connected apps and action schemas come from Pramaan discovery.
func Catalog() []models.IntegrationCatalogEntry         { return nil }
func ProviderLabel(p models.IntegrationProvider) string { return "Connected app" }
