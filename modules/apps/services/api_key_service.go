package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/apps/dto"
	"github.com/longlbh/appdist-be/utils"
)

const (
	apiKeyPrefix  = "gmo_"
	apiKeyLength  = 32
	apiKeyPreview = 6
)

var errAPIKeyNotFound = common.ErrNotFound("API key")

// CreateAPIKey sinh key mới cho App. Plaintext chỉ trả về đúng lần này; DB
// chỉ giữ sha256 — lộ DB không lộ key dùng được.
//
// Một key dùng chung cho mọi platform của App.
func (s *AppService) CreateAPIKey(ctx context.Context, user sqlc.User, appID uuid.UUID, p dto.CreateAPIKeyPayload) (*dto.APIKeyDTO, error) {
	if _, err := s.Access(ctx, user, appID); err != nil {
		return nil, err
	}

	random, err := utils.RandomString(apiKeyLength, utils.ReadableAlphabet)
	if err != nil {
		return nil, common.ErrInternal(err)
	}
	token := apiKeyPrefix + random

	k, err := s.store.CreateAPIKey(ctx, sqlc.CreateAPIKeyParams{
		AppID:        appID,
		Label:        p.Label,
		TokenHash:    utils.SHA256Hex(token),
		TokenPreview: token[len(token)-apiKeyPreview:],
		CreatedBy:    user.ID,
	})
	if err != nil {
		return nil, common.ErrDB(err)
	}

	res := apiKeyDTO(k, user.Email)
	res.Token = token
	return &res, nil
}

// ListAPIKeys trả các key chưa thu hồi — không bao giờ có plaintext.
func (s *AppService) ListAPIKeys(ctx context.Context, user sqlc.User, appID uuid.UUID) ([]dto.APIKeyDTO, error) {
	if _, err := s.Access(ctx, user, appID); err != nil {
		return nil, err
	}
	rows, err := s.store.ListAPIKeys(ctx, appID)
	if err != nil {
		return nil, common.ErrDB(err)
	}
	out := make([]dto.APIKeyDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, apiKeyDTO(r.AppApiKey, r.CreatedByEmail))
	}
	return out, nil
}

// RevokeAPIKey thu hồi ngay: request kế tiếp dùng key này bị 401.
func (s *AppService) RevokeAPIKey(ctx context.Context, user sqlc.User, appID, keyID uuid.UUID) error {
	if _, err := s.Access(ctx, user, appID); err != nil {
		return err
	}
	n, err := s.store.RevokeAPIKey(ctx, sqlc.RevokeAPIKeyParams{AppID: appID, ID: keyID})
	if err != nil {
		return common.ErrDB(err)
	}
	if n == 0 {
		return errAPIKeyNotFound
	}
	return nil
}

func apiKeyDTO(k sqlc.AppApiKey, createdBy string) dto.APIKeyDTO {
	return dto.APIKeyDTO{
		ID:           k.ID,
		AppID:        k.AppID,
		Label:        k.Label,
		TokenPreview: k.TokenPreview,
		CreatedAt:    k.CreatedAt,
		CreatedBy:    createdBy,
		LastUsedAt:   db.TimePtr(k.LastUsedAt),
	}
}
