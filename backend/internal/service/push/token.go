package push

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"hospital-qc-wework/internal/config"

	"github.com/rs/zerolog/log"
)

// TokenManager access_token 管理器（多级缓存）
type TokenManager struct {
	cfg        *config.WeWorkConfig
	httpClient *http.Client

	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
}

// tokenResponse 企业微信 access_token 响应
type tokenResponse struct {
	Errcode     int    `json:"errcode"`
	Errmsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewTokenManager 创建 Token 管理器
func NewTokenManager(cfg *config.WeWorkConfig) *TokenManager {
	return &TokenManager{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetToken 获取 access_token（优先返回缓存的 token）
func (m *TokenManager) GetToken() (string, error) {
	// 读锁检查缓存
	m.mu.RLock()
	if m.accessToken != "" && time.Now().Before(m.expiresAt) {
		token := m.accessToken
		m.mu.RUnlock()
		return token, nil
	}
	m.mu.RUnlock()

	// 缓存过期或不存在，加写锁刷新
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查（防止多个 goroutine 同时进入）
	if m.accessToken != "" && time.Now().Before(m.expiresAt) {
		return m.accessToken, nil
	}

	// 请求新 token
	token, expiresIn, err := m.fetchToken()
	if err != nil {
		return "", err
	}

	m.accessToken = token
	// 提前 100 秒过期，保证边界安全
	m.expiresAt = time.Now().Add(time.Duration(expiresIn-100) * time.Second)

	log.Info().Time("expiresAt", m.expiresAt).Msg("access_token 刷新成功")
	return m.accessToken, nil
}

// RefreshLoop 定时刷新 access_token（启动一个后台协程）
func (m *TokenManager) RefreshLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 立即刷新一次
	if _, err := m.GetToken(); err != nil {
		log.Warn().Err(err).Msg("首次 access_token 刷新失败，将在下一周期重试")
	}

	for range ticker.C {
		// 强制刷新（清除缓存）
		m.mu.Lock()
		m.accessToken = ""
		m.expiresAt = time.Time{}
		m.mu.Unlock()

		if _, err := m.GetToken(); err != nil {
			log.Warn().Err(err).Msg("access_token 定时刷新失败")
		}
	}
}

// fetchToken 请求企业微信 API 获取 access_token
func (m *TokenManager) fetchToken() (string, int, error) {
	url := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		m.cfg.APIBaseURL, m.cfg.CorpID, m.cfg.AgentSecret)

	resp, err := m.httpClient.Get(url)
	if err != nil {
		return "", 0, fmt.Errorf("请求 access_token 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("读取响应失败: %w", err)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}

	if tr.Errcode != 0 {
		return "", 0, fmt.Errorf("企业微信 API 错误: %d - %s", tr.Errcode, tr.Errmsg)
	}

	return tr.AccessToken, tr.ExpiresIn, nil
}
