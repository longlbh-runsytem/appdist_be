package appctx

import (
	"github.com/longlbh/appdist-be/components/storage"
	"github.com/longlbh/appdist-be/components/token"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/utils"
)

// AppContext gom các phụ thuộc dùng chung, dựng một lần lúc khởi động.
type AppContext interface {
	GetStore() db.Store
	GetStorage() storage.Storage
	GetTokenMaker() token.TokenMaker
	GetConfig() utils.Config
}

type appCtx struct {
	store      db.Store
	storage    storage.Storage
	tokenMaker token.TokenMaker
	cfg        utils.Config
}

func NewAppContext(store db.Store, st storage.Storage, tokenMaker token.TokenMaker, cfg utils.Config) AppContext {
	return &appCtx{store: store, storage: st, tokenMaker: tokenMaker, cfg: cfg}
}

func (a *appCtx) GetStore() db.Store              { return a.store }
func (a *appCtx) GetStorage() storage.Storage     { return a.storage }
func (a *appCtx) GetTokenMaker() token.TokenMaker { return a.tokenMaker }
func (a *appCtx) GetConfig() utils.Config         { return a.cfg }
