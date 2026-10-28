package services

import (
	"context"
	"net/url"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/ticket"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/modules/builds/dto"
)

// buildView là build kèm những gì cần để dựng DTO mà bảng builds không có.
type buildView struct {
	build         sqlc.Build
	uploaderEmail string
	appIconKey    string      // icon của App cha; rỗng với quick share
	appIsPublic   pgtype.Bool // NULL với quick share (luôn public)
}

// toDTOs dựng DTO cho nhiều build, tester group lấy bằng một query.
func (s *BuildService) toDTOs(ctx context.Context, views []buildView) ([]dto.BuildDTO, error) {
	out := make([]dto.BuildDTO, 0, len(views))
	if len(views) == 0 {
		return out, nil
	}

	ids := make([]int64, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.build.ID)
	}
	rows, err := s.store.ListBuildTesterGroups(ctx, ids)
	if err != nil {
		return nil, common.ErrDB(err)
	}
	groups := map[int64][]common.RefDTO{}
	for _, r := range rows {
		groups[r.BuildID] = append(groups[r.BuildID], common.RefDTO{ID: r.ID, Name: r.Name})
	}

	for _, v := range views {
		d := dto.FromSQLC(v.build)
		d.UploadedBy = v.uploaderEmail
		if g := groups[v.build.ID]; g != nil {
			d.TesterGroups = g
		}

		iconKey := v.appIconKey
		if v.build.AppID == nil {
			iconKey = v.build.IconKey.String
		}
		d.IconURL = s.presignIcon(ctx, iconKey)

		d.DownloadURL = s.cfg.FrontendURL + "/d/" + v.build.Token
		if v.appIsPublic.Valid && !v.appIsPublic.Bool {
			d.DownloadURL += "?" + ticket.QueryParam + "=" + url.QueryEscape(s.tickets.Sign(v.build.Token, ticketTTL))
		}
		out = append(out, d)
	}
	return out, nil
}

func (s *BuildService) presignIcon(ctx context.Context, key string) *string {
	if key == "" {
		return nil
	}
	u, err := s.storage.PresignGet(ctx, key, iconURLExpiry, "")
	if err != nil {
		return nil
	}
	return &u
}
