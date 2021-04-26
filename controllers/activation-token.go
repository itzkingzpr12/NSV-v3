package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/models"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/utils/logging"
	"gitlab.com/BIC_Dev/nitrado-server-manager-v3/viewmodels"
	"go.uber.org/zap"
)

// CreateActivationToken adds an activation token to the cache
func (c *Controller) CreateActivationToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx = logging.AddValues(ctx, zap.String("scope", logging.GetFuncName()))

	b := make([]byte, 4) //equals 8 charachters
	rand.Read(b)

	setupToken := models.ActivationToken{
		Token: hex.EncodeToString(b),
	}

	cacheKey := setupToken.CacheKey(c.Config.CacheSettings.ActivationToken.Base, setupToken.Token)
	setCacheErr := c.Cache.SetStruct(ctx, cacheKey, &setupToken, c.Config.CacheSettings.ActivationToken.TTL)
	if setCacheErr != nil {
		Error(ctx, w, setCacheErr.Message, setCacheErr.Err, http.StatusInternalServerError)
		return
	}

	Response(ctx, w, viewmodels.CreateActivationTokenResponse{
		Message:    "Created record",
		SetupToken: setupToken,
	}, http.StatusCreated)
}
