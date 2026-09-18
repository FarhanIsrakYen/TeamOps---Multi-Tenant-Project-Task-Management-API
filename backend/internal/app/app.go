package app

import (
	"context"
	"net/http"
	"time"

	"github.com/example/teamops/backend/internal/cache"
	"github.com/example/teamops/backend/internal/config"
	"github.com/example/teamops/backend/internal/database"
	"github.com/example/teamops/backend/internal/middleware"
	audithandler "github.com/example/teamops/backend/internal/modules/audit/handler"
	auditrepo "github.com/example/teamops/backend/internal/modules/audit/repository"
	auditsvc "github.com/example/teamops/backend/internal/modules/audit/service"
	authguard "github.com/example/teamops/backend/internal/modules/auth/guard"
	authhandler "github.com/example/teamops/backend/internal/modules/auth/handler"
	authrepo "github.com/example/teamops/backend/internal/modules/auth/repository"
	authsvc "github.com/example/teamops/backend/internal/modules/auth/service"
	orgguard "github.com/example/teamops/backend/internal/modules/organizations/guard"
	orghandler "github.com/example/teamops/backend/internal/modules/organizations/handler"
	orgrepo "github.com/example/teamops/backend/internal/modules/organizations/repository"
	orgsvc "github.com/example/teamops/backend/internal/modules/organizations/service"
	projectguard "github.com/example/teamops/backend/internal/modules/projects/guard"
	projecthandler "github.com/example/teamops/backend/internal/modules/projects/handler"
	projectrepo "github.com/example/teamops/backend/internal/modules/projects/repository"
	projectsvc "github.com/example/teamops/backend/internal/modules/projects/service"
	taskguard "github.com/example/teamops/backend/internal/modules/tasks/guard"
	taskhandler "github.com/example/teamops/backend/internal/modules/tasks/handler"
	taskrepo "github.com/example/teamops/backend/internal/modules/tasks/repository"
	tasksvc "github.com/example/teamops/backend/internal/modules/tasks/service"
	usershandler "github.com/example/teamops/backend/internal/modules/users/handler"
	usersrepo "github.com/example/teamops/backend/internal/modules/users/repository"
	userssvc "github.com/example/teamops/backend/internal/modules/users/service"
	"github.com/example/teamops/backend/internal/platform/openapi"
	sharedauth "github.com/example/teamops/backend/internal/shared/auth"
	"github.com/example/teamops/backend/internal/shared/errors"
	"github.com/example/teamops/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

type App struct {
	Router         *gin.Engine
	AuthRepository *authrepo.Repository
	AuditRecorder  *auditsvc.Recorder
}

func New(cfg config.Config, db *pgxpool.Pool, cacheClient *cache.Cache, log zerolog.Logger) *App {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	_ = r.SetTrustedProxies(cfg.TrustedProxies)
	r.HandleMethodNotAllowed = true
	middleware.RegisterMetrics()
	r.Use(
		middleware.RequestID(),
		middleware.Recovery(log),
		middleware.SecureHeaders(cfg.Environment == "production"),
		middleware.Logging(log),
		middleware.Metrics(),
		middleware.CORS(cfg.CORSOrigins),
		middleware.BodyLimit(cfg.RequestBodyMaxBytes),
	)
	healthHandler := func(c *gin.Context) { response.OK(c, gin.H{"status": "ok"}) }
	readyHandler := func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c, 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			log.Error().Err(err).Msg("readiness database check failed")
			response.Error(c, apperror.New(http.StatusServiceUnavailable, "not_ready", "service is not ready"))
			return
		}
		if err := cacheClient.Ping(ctx); err != nil {
			log.Warn().Err(err).Msg("readiness redis check degraded")
			response.OK(c, gin.H{"status": "ready", "checks": gin.H{"database": "ok", "redis": "degraded"}})
			return
		}
		response.OK(c, gin.H{"status": "ready", "checks": gin.H{"database": "ok", "redis": "ok"}})
	}
	r.GET("/health", healthHandler)
	r.GET("/ready", readyHandler)
	// Keep the previous probe paths as backwards-compatible aliases.
	r.GET("/health/live", healthHandler)
	r.GET("/health/ready", readyHandler)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.GET("/openapi.yaml", func(c *gin.Context) { c.Data(http.StatusOK, "application/yaml", openapi.Document) })
	auditRepository := auditrepo.New(db)
	auditRecorder := auditsvc.NewRecorder(auditRepository, log, cfg.AuditQueueSize, cfg.AuditWorkers, cfg.AuditWriteTimeout)
	authRepository := authrepo.New(db)
	usersRepository := usersrepo.New(db)
	orgRepository := orgrepo.New(db)
	projectRepository := projectrepo.New(db)
	taskRepository := taskrepo.New(db)
	tokens := sharedauth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, cfg.AccessTokenTTL)
	authService := authsvc.New(usersRepository, authRepository, database.NewTransactor(db), tokens, cfg.RefreshTokenTTL, cfg.PasswordHashCost, auditRecorder)
	loginProtector := authguard.NewLoginProtector(cacheClient, cfg.LoginFailureLimit, cfg.LoginFailureWindow)
	organizationGuard := orgguard.New(orgRepository, cacheClient, cfg.MembershipCacheTTL)
	projectGuard := projectguard.New(organizationGuard)
	taskGuard := taskguard.New(organizationGuard)
	orgService := orgsvc.New(orgRepository, auditRecorder, database.NewTransactor(db), organizationGuard, cacheClient, cfg.OrganizationCacheTTL)
	projectService := projectsvc.New(projectRepository, organizationGuard, projectGuard, cacheClient, cfg.ProjectCacheTTL, auditRecorder)
	taskService := tasksvc.New(taskRepository, taskRepository, taskRepository, projectRepository, organizationGuard, projectGuard, taskGuard, auditRecorder)
	auditService := auditsvc.New(auditRepository, organizationGuard)
	usersService := userssvc.New(usersRepository)
	authH := authhandler.New(authService, loginProtector)
	orgH := orghandler.New(orgService)
	projectH := projecthandler.New(projectService)
	taskH := taskhandler.New(taskService)
	auditH := audithandler.New(auditService)
	usersH := usershandler.New(usersService)
	v1 := r.Group("/api/v1")
	v1.Use(middleware.RateLimit(cacheClient, "api", cfg.RateLimitPerMin, time.Minute))
	authRoutes := v1.Group("/auth")
	authRoutes.Use(middleware.RateLimit(cacheClient, "auth", cfg.AuthRateLimitPerMin, time.Minute))
	authRoutes.POST("/register", authH.Register)
	authRoutes.POST("/login", middleware.RateLimit(cacheClient, "login", cfg.LoginRateLimitPerMin, time.Minute), authH.Login)
	authRoutes.POST("/refresh", authH.Refresh)
	authRoutes.POST("/logout", authH.Logout)
	secured := v1.Group("")
	secured.Use(authguard.RequireAuthenticatedUser(tokens))
	secured.GET("/me", usersH.Me)
	secured.GET("/organizations", orgH.List)
	secured.POST("/organizations", orgH.Create)
	secured.GET("/organizations/:organizationId", orgH.Get)
	secured.PATCH("/organizations/:organizationId", orgH.Update)
	secured.DELETE("/organizations/:organizationId", orgH.Delete)
	secured.GET("/organizations/:organizationId/members", orgH.Members)
	secured.POST("/organizations/:organizationId/members", orgH.AddMember)
	secured.DELETE("/organizations/:organizationId/members/:userId", orgH.RemoveMember)
	secured.GET("/organizations/:organizationId/projects", projectH.List)
	secured.POST("/organizations/:organizationId/projects", projectH.Create)
	secured.GET("/projects/:projectId", projectH.Get)
	secured.PATCH("/projects/:projectId", projectH.Update)
	secured.DELETE("/projects/:projectId", projectH.Delete)
	secured.GET("/projects/:projectId/tasks", taskH.List)
	secured.POST("/projects/:projectId/tasks", taskH.Create)
	secured.GET("/tasks/:taskId", taskH.Get)
	secured.PATCH("/tasks/:taskId", taskH.Update)
	secured.DELETE("/tasks/:taskId", taskH.Delete)
	secured.GET("/tasks/:taskId/comments", taskH.Comments)
	secured.POST("/tasks/:taskId/comments", taskH.AddComment)
	secured.PUT("/tasks/:taskId/labels", taskH.SetLabels)
	secured.GET("/organizations/:organizationId/labels", taskH.Labels)
	secured.POST("/organizations/:organizationId/labels", taskH.CreateLabel)
	secured.GET("/organizations/:organizationId/audit-logs", auditH.List)
	r.NoRoute(func(c *gin.Context) {
		response.Error(c, apperror.New(http.StatusNotFound, "route_not_found", "route not found"))
	})
	r.NoMethod(func(c *gin.Context) {
		response.Error(c, apperror.New(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed"))
	})
	return &App{Router: r, AuthRepository: authRepository, AuditRecorder: auditRecorder}
}
