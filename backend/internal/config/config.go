package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Database    DBConfig          `yaml:"database"`
	HISDatabase HISDBConfig       `yaml:"his_database"`
	WeWork      WeWorkConfig      `yaml:"wework"`
	JWT         JWTConfig         `yaml:"jwt"`
	Redis       RedisConfig       `yaml:"redis"`
	QC          QCConfig          `yaml:"qc"`
	Log         LogConfig         `yaml:"log"`
}

type ServerConfig struct {
	Port         int           `yaml:"port"`
	Mode         string        `yaml:"mode"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type DBConfig struct {
	Server          string        `yaml:"server"`
	Port            int           `yaml:"port"`
	Name            string        `yaml:"name"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"` // 可由院方直接填写，环境变量 DB_PASS 优先
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	Encrypt         bool          `yaml:"encrypt"`
}

type HISDBConfig struct {
	Server       string        `yaml:"server"`
	Port         int           `yaml:"port"`
	Name         string        `yaml:"name"`
	User         string        `yaml:"user"`
	Password     string        `yaml:"password"` // 可由院方直接填写，环境变量 HIS_DB_PASS 优先
	SyncInterval time.Duration `yaml:"sync_interval"`
	SyncTime     string        `yaml:"sync_time"`
}

type WeWorkConfig struct {
	CorpID              string        `yaml:"corp_id"`
	AgentID             int           `yaml:"agent_id"`
	AgentSecret         string        `yaml:"-"` // 从环境变量读取
	TokenRefreshInterval time.Duration `yaml:"token_refresh_interval"`
	APIBaseURL          string        `yaml:"api_base_url"`
}

type JWTConfig struct {
	Secret      string `yaml:"-"` // 从环境变量读取
	ExpireHours int    `yaml:"expire_hours"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type QCConfig struct {
	Cron          string `yaml:"cron"`
	BatchSize     int    `yaml:"batch_size"`
	Concurrency   int    `yaml:"concurrency"`
	QuietStart    string `yaml:"quiet_start"`
	QuietEnd      string `yaml:"quiet_end"`
	PushCron      string `yaml:"push_cron"`
}

type LogConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	Output   string `yaml:"output"`
	FilePath string `yaml:"file_path"`
}

// DSN 返回 SQL Server 连接字符串
// SQL Server 2014 仅支持 TLS 1.0，Go 1.18+ 默认禁用了它
// 使用 encrypt=disable 完全禁用传输层加密（医院内网单机部署，无中间人风险）
// go-mssqldb 中 encrypt=false 仍会加密登录包从而触发 TLS 握手，必须用 DISABLE
func (d DBConfig) DSN() string {
	// 优先从环境变量读取密码
	password := d.Password
	if envPwd := os.Getenv("DB_PASS"); envPwd != "" {
		password = envPwd
	}
	return fmt.Sprintf("server=%s;port=%s;user id=%s;password=%s;database=%s;encrypt=disable",
		d.Server, itoa(d.Port), d.User, password, d.Name)
}

// HISDSN 返回 HIS 数据库连接字符串
func (d HISDBConfig) HISDSN() string {
	password := d.Password
	if envPwd := os.Getenv("HIS_DB_PASS"); envPwd != "" {
		password = envPwd
	}
	return fmt.Sprintf("server=%s;port=%s;user id=%s;password=%s;database=%s;encrypt=disable",
		d.Server, itoa(d.Port), d.User, password, d.Name)
}

// Load 加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// 从环境变量覆盖敏感字段
	cfg.Database.Password = envOrDefault("DB_PASS", cfg.Database.Password)
	cfg.HISDatabase.Password = envOrDefault("HIS_DB_PASS", cfg.HISDatabase.Password)
	cfg.HISDatabase.User = envOrDefault("HIS_DB_USER", cfg.HISDatabase.User)
	cfg.WeWork.CorpID = envOrDefault("WEWORK_CORP_ID", cfg.WeWork.CorpID)
	cfg.WeWork.AgentSecret = envOrDefault("WEWORK_AGENT_SECRET", cfg.WeWork.AgentSecret)
	cfg.JWT.Secret = envOrDefault("JWT_SECRET", cfg.JWT.Secret)

	// AgentID 从环境变量覆盖（int 类型）
	if agentID := os.Getenv("WEWORK_AGENT_ID"); agentID != "" {
		id := 0
		if _, err := fmt.Sscanf(agentID, "%d", &id); err == nil {
			cfg.WeWork.AgentID = id
		}
	}

	// 设置默认值
	setDefaults(cfg)

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 20
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 5
	}
	if cfg.Database.ConnMaxLifetime == 0 {
		cfg.Database.ConnMaxLifetime = 5 * time.Minute
	}
	if cfg.JWT.ExpireHours == 0 {
		cfg.JWT.ExpireHours = 24
	}
	if cfg.QC.Concurrency == 0 {
		cfg.QC.Concurrency = 10
	}
	if cfg.QC.BatchSize == 0 {
		cfg.QC.BatchSize = 100
	}
	if cfg.QC.Cron == "" {
		cfg.QC.Cron = "10 6 * * *"
	}
	if cfg.QC.PushCron == "" {
		cfg.QC.PushCron = "30 6 * * *"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	if cfg.WeWork.TokenRefreshInterval == 0 {
		cfg.WeWork.TokenRefreshInterval = 3500 * time.Second
	}
	if cfg.WeWork.APIBaseURL == "" {
		cfg.WeWork.APIBaseURL = "https://qyapi.weixin.qq.com"
	}
}

func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func itoa(n int) string {
	if n == 0 {
		return "1433"
	}
	return fmt.Sprintf("%d", n)
}
