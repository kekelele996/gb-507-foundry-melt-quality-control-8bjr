package router

import (
	"log/slog"
	"net/http"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/config"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/handler"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/middleware"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/repository"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(middleware.RequestContext(logger))
	if len(cfg.TrustedProxies) > 0 {
		_ = engine.SetTrustedProxies(cfg.TrustedProxies)
	} else {
		_ = engine.SetTrustedProxies(nil)
	}

	securityRepository := repository.NewSecurityRepository(db)
	securityService := service.NewSecurityService(securityRepository, cfg)
	furnaceRepository := repository.NewFurnaceRepository(db)
	heatRepository := repository.NewHeatRepository(db)
	chemicalSampleRepository := repository.NewChemicalSampleRepository(db)
	qualityDecisionRepository := repository.NewQualityDecisionRepository(db)
	furnaceService := service.NewFurnaceService(furnaceRepository, securityService)
	heatService := service.NewHeatService(heatRepository, furnaceRepository, chemicalSampleRepository, securityService)
	chemicalSampleService := service.NewChemicalSampleService(chemicalSampleRepository, heatRepository, securityService)
	qualityDecisionService := service.NewQualityDecisionService(qualityDecisionRepository, heatRepository, chemicalSampleRepository, securityService)
	furnaceHandler := handler.NewFurnaceHandler(furnaceService)
	heatHandler := handler.NewHeatHandler(heatService)
	chemicalSampleHandler := handler.NewChemicalSampleHandler(chemicalSampleService)
	qualityDecisionHandler := handler.NewQualityDecisionHandler(qualityDecisionService)
	systemHandler := handler.NewSystemHandler(securityService, furnaceService, heatService, chemicalSampleService, qualityDecisionService, db, redisClient)

	engine.GET("/healthz", systemHandler.Health)
	loginLimiter := middleware.NewLimiter(redisClient, cfg.LoginRequestLimit)
	engine.POST("/api/auth/login", loginLimiter.Middleware(), systemHandler.Login)

	limiter := middleware.NewLimiter(redisClient, cfg.RequestLimit)
	api := engine.Group("/api")
	api.Use(limiter.Middleware(), middleware.Authenticate(cfg))
	api.GET("/overview", systemHandler.Overview)
	api.GET("/audits", middleware.RequireRoles(model.RoleReviewer, model.RoleAdmin), systemHandler.Audits)
	api.GET("/session", systemHandler.Session)
	api.GET("/runtime", middleware.RequireRoles(model.RoleReviewer, model.RoleAdmin), systemHandler.Runtime)
	api.GET("/audit-summary", middleware.RequireRoles(model.RoleReviewer, model.RoleAdmin), systemHandler.AuditSummary)
	api.GET("/audits/:entityType/:id", middleware.RequireRoles(model.RoleReviewer, model.RoleAdmin), systemHandler.EntityHistory)
	furnaceHandler.Register(api)
	heatHandler.Register(api)
	chemicalSampleHandler.Register(api)
	qualityDecisionHandler.Register(api)

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "route_not_found"})
	})
	return engine
}
