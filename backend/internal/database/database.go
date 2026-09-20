package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/config"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, cfg config.Config, log *slog.Logger) (*gorm.DB, *redis.Client, error) {
	var dialector gorm.Dialector
	switch cfg.DatabaseDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DatabaseDSN)
	case "mysql":
		dialector = mysql.Open(cfg.DatabaseDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DatabaseDSN)
	default:
		return nil, nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
	logLevel := logger.Warn
	if cfg.Environment == "development" {
		logLevel = logger.Info
	}
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		db, err = gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil && sqlDB.PingContext(ctx) == nil {
				break
			}
			if dbErr != nil {
				err = dbErr
			} else {
				err = sqlDB.PingContext(ctx)
			}
		}
		log.Warn("database not ready", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect database: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, nil, err
	}
	if err := Seed(ctx, db); err != nil {
		return nil, nil, err
	}
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, nil, fmt.Errorf("connect redis: %w", err)
		}
	}
	return db, redisClient, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{}, &model.AuditLog{},
		&model.Furnace{}, &model.Heat{}, &model.ChemicalSample{}, &model.QualityDecision{},
	)
}

func Seed(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := seedUsers(tx); err != nil {
			return err
		}
		if err := seedFurnaces(tx); err != nil {
			return err
		}
		if err := seedHeats(tx); err != nil {
			return err
		}
		if err := seedSamples(tx); err != nil {
			return err
		}
		return seedDecisions(tx)
	})
}

func seedUsers(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	password, err := bcrypt.GenerateFromPassword([]byte("Admin123!"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	users := []model.User{
		{Username: "admin", DisplayName: "系统管理员", PasswordHash: string(password), Role: model.RoleAdmin, Active: true},
		{Username: "reviewer", DisplayName: "质量复核员", PasswordHash: string(password), Role: model.RoleReviewer, Active: true},
		{Username: "operator", DisplayName: "现场操作员", PasswordHash: string(password), Role: model.RoleOperator, Active: true},
		{Username: "viewer", DisplayName: "只读观察员", PasswordHash: string(password), Role: model.RoleViewer, Active: true},
	}
	return db.Create(&users).Error
}

func seedFurnaces(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.Furnace{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.Furnace{
		{
			BaseModel: model.BaseModel{Code: "F-001", Name: "一号中频感应炉", Status: "available", Version: 1, Description: "球墨铸铁与灰铸铁主力炉台"},
			PlantArea: "熔炼一车间", FurnaceType: "中频感应炉", CapacityTonnes: 12, SupportedAlloys: "QT450-10,HT250",
			MaxTemperatureC: 1600, Operator: "熔炼甲班", LastInspectionAt: now.Add(-24 * time.Hour), Evidence: "INS-F001-20260821",
		},
		{
			BaseModel: model.BaseModel{Code: "F-002", Name: "二号电弧炉", Status: "charging", Version: 1, Description: "铸钢合金熔炼炉台"},
			PlantArea: "熔炼二车间", FurnaceType: "三相电弧炉", CapacityTonnes: 20, SupportedAlloys: "ZG270-500,ZG35CrMo",
			MaxTemperatureC: 1750, Operator: "熔炼乙班", LastInspectionAt: now.Add(-36 * time.Hour), Evidence: "INS-F002-20260820",
		},
		{
			BaseModel: model.BaseModel{Code: "F-003", Name: "三号保温炉", Status: "maintenance", Version: 1, Description: "计划检修中的备用炉台"},
			PlantArea: "熔炼一车间", FurnaceType: "工频保温炉", CapacityTonnes: 8, SupportedAlloys: "HT250",
			MaxTemperatureC: 1500, Operator: "设备保障组", LastInspectionAt: now.Add(-72 * time.Hour), Evidence: "WO-F003-8841",
		},
	}
	return db.Create(&items).Error
}

func seedHeats(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.Heat{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.Heat{
		newHeat("H-260822-01", "球墨铸铁试制炉次", "charged", "F-001", "QT450-10", "熔炼甲班", 9800, 1510, 3.40, 3.80, 2.20, 2.80, 0.05, 0.08, now.Add(-90*time.Minute)),
		newHeat("H-260822-02", "铸钢生产炉次", "sampling", "F-002", "ZG270-500", "熔炼乙班", 17500, 1640, 0.25, 0.35, 0.20, 0.50, 0.04, 0.04, now.Add(-3*time.Hour)),
		newHeat("H-260821-03", "灰铸铁待判炉次", "hold", "F-001", "HT250", "熔炼甲班", 10400, 1490, 3.10, 3.40, 1.80, 2.20, 0.08, 0.12, now.Add(-8*time.Hour)),
		newHeat("H-260821-04", "球墨铸铁已接收炉次", "accepted", "F-001", "QT450-10", "熔炼甲班", 9600, 1520, 3.40, 3.80, 2.20, 2.80, 0.05, 0.08, now.Add(-14*time.Hour)),
		newHeat("H-260821-05", "铸钢报废炉次", "rejected", "F-002", "ZG270-500", "熔炼乙班", 16800, 1650, 0.25, 0.35, 0.20, 0.50, 0.04, 0.04, now.Add(-20*time.Hour)),
		newHeat("H-260822-06", "灰铸铁待判炉次（碳差阻塞）", "hold", "F-001", "HT250", "熔炼甲班", 9900, 1490, 3.10, 3.40, 1.80, 2.20, 0.08, 0.12, now.Add(-70*time.Minute)),
		newHeat("H-260822-07", "球墨铸铁待判炉次（样本不足）", "hold", "F-001", "QT450-10", "熔炼甲班", 9700, 1505, 3.40, 3.80, 2.20, 2.80, 0.05, 0.08, now.Add(-50*time.Minute)),
	}
	return db.Create(&items).Error
}

func newHeat(code, name, status, furnaceCode, alloy, owner string, weight, temperature, carbonMin, carbonMax,
	siliconMin, siliconMax, sulfurMax, phosphorusMax float64, startedAt time.Time) model.Heat {
	return model.Heat{
		BaseModel:   model.BaseModel{Code: code, Name: name, Status: status, Version: 1, Description: "生产炉次及其冻结成分规格"},
		FurnaceCode: furnaceCode, AlloyGrade: alloy, Owner: owner, ChargeWeightKg: weight, TargetTemperatureC: temperature,
		CarbonMinPct: carbonMin, CarbonMaxPct: carbonMax, SiliconMinPct: siliconMin, SiliconMaxPct: siliconMax,
		SulfurMaxPct: sulfurMax, PhosphorusMaxPct: phosphorusMax, StartedAt: startedAt, Evidence: "MES-" + code,
	}
}

func seedSamples(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.ChemicalSample{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.ChemicalSample{
		newSample("CS-260822-01", "炉前铸钢样本", "testing", "H-260822-02", "炉前包", "OES-2026.3", "operator", 0.31, 0.34, 0.72, 0.021, 0.025, now.Add(-35*time.Minute)),
		newSample("CS-260821-02", "灰铸铁终检样本A", "verified", "H-260821-03", "浇包前", "OES-2026.3", "reviewer", 3.24, 2.02, 0.71, 0.042, 0.076, now.Add(-7*time.Hour)),
		newSample("CS-260821-02B", "灰铸铁终检样本B", "verified", "H-260821-03", "浇注口", "OES-2026.3", "reviewer", 3.27, 2.00, 0.70, 0.045, 0.072, now.Add(-6*time.Hour)),
		newSample("CS-260821-03", "球墨铸铁放行样本A", "locked", "H-260821-04", "浇包前", "OES-2026.3", "reviewer", 3.61, 2.48, 0.29, 0.028, 0.051, now.Add(-13*time.Hour)),
		newSample("CS-260821-03B", "球墨铸铁放行样本B", "locked", "H-260821-04", "浇注口", "OES-2026.3", "reviewer", 3.64, 2.46, 0.30, 0.030, 0.049, now.Add(-12*time.Hour)),
		newSample("CS-260821-04", "铸钢超限样本", "verified", "H-260821-05", "炉前包", "OES-2026.3", "reviewer", 0.48, 0.33, 0.74, 0.061, 0.052, now.Add(-19*time.Hour)),
		newSample("CS-260822-06A", "灰铸铁复核样本A（碳差）", "verified", "H-260822-06", "浇包前", "OES-2026.3", "reviewer", 3.22, 2.05, 0.72, 0.050, 0.080, now.Add(-65*time.Minute)),
		newSample("CS-260822-06B", "灰铸铁复核样本B（碳差）", "verified", "H-260822-06", "浇注口", "OES-2026.3", "reviewer", 3.30, 2.03, 0.71, 0.048, 0.077, now.Add(-60*time.Minute)),
		newSample("CS-260822-07A", "球墨铸铁复核样本A（缺一）", "verified", "H-260822-07", "浇包前", "OES-2026.3", "reviewer", 3.58, 2.50, 0.30, 0.030, 0.055, now.Add(-45*time.Minute)),
	}
	return db.Create(&items).Error
}

func newSample(code, name, status, heatCode, point, method, analyst string, carbon, silicon, manganese,
	sulfur, phosphorus float64, sampledAt time.Time) model.ChemicalSample {
	return model.ChemicalSample{
		BaseModel: model.BaseModel{Code: code, Name: name, Status: status, Version: 1, Description: "炉前五元素光谱检测"},
		HeatCode:  heatCode, SamplePoint: point, MethodVersion: method, Analyst: analyst,
		CarbonPct: carbon, SiliconPct: silicon, ManganesePct: manganese, SulfurPct: sulfur,
		PhosphorusPct: phosphorus, SampledAt: sampledAt, Evidence: "LIMS-" + code,
	}
}

func seedDecisions(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.QualityDecision{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.QualityDecision{
		{
			BaseModel: model.BaseModel{Code: "QD-260821-02", Name: "球墨铸铁放行合议接收", Status: "accept", Version: 2, Description: "双样本复核合格，炉次放行"},
			HeatCode:  "H-260821-04", SampleCode: "CS-260821-03", PairedSampleCode: "CS-260821-03B", Reviewer: "reviewer",
			Reason:     "两份复核样本碳硅硫磷均在 QT450-10 范围内，碳硅差值均小于 0.05%",
			Conditions: "配对 CS-260821-03/CS-260821-03B；ΔC=0.030% ΔSi=0.020%，双样本四元素均在牌号范围内",
			DecidedAt:  now.Add(-12 * time.Hour), Evidence: "QMS-QD-260821-02",
		},
		{
			BaseModel: model.BaseModel{Code: "QD-260821-03", Name: "铸钢报废决定", Status: "scrap", Version: 2, Description: "碳硫磷多项超限"},
			HeatCode:  "H-260821-05", SampleCode: "CS-260821-04", Reviewer: "reviewer", Reason: "关键元素超出冻结规格且不具备返炉价值",
			Conditions: "隔离并登记报废", DecidedAt: now.Add(-18 * time.Hour), Evidence: "QMS-QD-260821-03",
		},
	}
	return db.Create(&items).Error
}
