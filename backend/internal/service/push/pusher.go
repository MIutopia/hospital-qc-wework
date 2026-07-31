package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"hospital-qc-wework/internal/config"
	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/model"
	"hospital-qc-wework/pkg/tokenbucket"

	"github.com/rs/zerolog/log"
)

// Pusher 企业微信消息推送服务
type Pusher struct {
	cfg        *config.WeWorkConfig
	tokenMgr   *TokenManager
	httpClient *http.Client
	pushLogDAO *dao.PushLogDAO
	limiter    *tokenbucket.Bucket
}

// CardMessage 模板卡片消息
type CardMessage struct {
	Touser   string       `json:"touser"`
	MsgType  string       `json:"msgtype"`
	AgentID  int          `json:"agentid"`
	Card     TemplateCard `json:"template_card"`
}

// TemplateCard 模板卡片
type TemplateCard struct {
	CardType            string              `json:"card_type"`
	Source              Source              `json:"source"`
	MainTitle           Title               `json:"main_title"`
	EmphasisContent     EmphasisContent     `json:"emphasis_content"`
	SubTitleText        string              `json:"sub_title_text"`
	HorizontalContent   []HorizontalContent `json:"horizontal_content_list"`
	CardAction          CardAction          `json:"card_action"`
	TaskID              string              `json:"task_id"`
}

type Source struct {
	Desc     string `json:"desc"`
	DescColor int    `json:"desc_color"`
}

type Title struct {
	Title string `json:"title"`
}

type EmphasisContent struct {
	Title string `json:"title"`
	Desc  string `json:"desc"`
}

type HorizontalContent struct {
	KeyName string `json:"keyname"`
	Value   string `json:"value"`
}

type CardAction struct {
	Type int    `json:"type"`
	URL  string `json:"url"`
}

// pushResponse 企业微信推送响应
type pushResponse struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
	MsgID   string `json:"msgid,omitempty"`
}

// NewPusher 创建推送服务
func NewPusher(cfg *config.WeWorkConfig, tokenMgr *TokenManager, pushLogDAO *dao.PushLogDAO) *Pusher {
	return &Pusher{
		cfg:      cfg,
		tokenMgr: tokenMgr,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		pushLogDAO: pushLogDAO,
		limiter:    tokenbucket.New(10, 100), // 10次/秒，桶容量100
	}
}

// SendCard 发送模板卡片消息
func (p *Pusher) SendCard(caseItem *model.InpatientCase, defectSummary *model.DefectSummary,
	defects []model.DefectItem, doctor *model.DoctorWeWork, token string) error {

	// 令牌桶限频
	if !p.limiter.WaitMax(1, 30*time.Second) {
		return fmt.Errorf("推送限频等待超时")
	}

	// 构建消息
	levelSummary := fmt.Sprintf("A级 %d项", defectSummary.LevelA)
	if defectSummary.LevelB > 0 {
		levelSummary += fmt.Sprintf(" / B级 %d项", defectSummary.LevelB)
	}
	if defectSummary.LevelC > 0 {
		levelSummary += fmt.Sprintf(" / C级 %d项", defectSummary.LevelC)
	}

	patientName := safeDeidentify(caseItem.PatientName)
	msg := CardMessage{
		Touser:  doctor.WeWorkUserID,
		MsgType: "template_card",
		AgentID: p.cfg.AgentID,
		Card: TemplateCard{
			CardType: "text_notice",
			Source: Source{
				Desc:      "质控科",
				DescColor: 1,
			},
			MainTitle: Title{
				Title: "住院病例质控提醒",
			},
			EmphasisContent: EmphasisContent{
				Title: fmt.Sprintf("%d项缺陷", defectSummary.Total),
				Desc:  "需整改",
			},
			SubTitleText: fmt.Sprintf("患者：%s | 住院号：%s", patientName, caseItem.CaseNo),
			HorizontalContent: []HorizontalContent{
				{KeyName: "缺陷等级", Value: levelSummary},
				{KeyName: "责任医生", Value: safeString(doctor.DoctorName)},
				{KeyName: "入院日期", Value: caseItem.AdmitTime.Format("2006-01-02")},
			},
			CardAction: CardAction{
				Type: 1,
				URL:  fmt.Sprintf("%s/report?caseId=%d&token=%s", p.cfg.APIBaseURL, caseItem.ID, token),
			},
			TaskID: fmt.Sprintf("QC%d", caseItem.ID),
		},
	}

	return p.send(msg, caseItem.ID, doctor.WeWorkUserID)
}

// send 发送消息到企业微信 API
func (p *Pusher) send(msg CardMessage, caseID int64, receiverUserID string) error {
	token, err := p.tokenMgr.GetToken()
	if err != nil {
		return fmt.Errorf("获取 access_token 失败: %w", err)
	}

	url := fmt.Sprintf("%s/cgi-bin/message/send?access_token=%s", p.cfg.APIBaseURL, token)

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	resp, err := p.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("请求企业微信 API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	var pr pushResponse
	if err := json.Unmarshal(respBody, &pr); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	// 记录推送日志
	respStr := string(respBody)
	p.pushLogDAO.Create(&model.PushLog{
		CaseID:         caseID,
		ReceiverUserID: receiverUserID,
		PushType:       model.PushTypeCard,
		PushContent:    &respStr,
	})

	if pr.Errcode != 0 {
		// 更新状态为失败
		p.pushLogDAO.UpdateStatus(0, model.PushStatusFailed, respStr)
		return fmt.Errorf("企业微信 API 错误: %d - %s", pr.Errcode, pr.Errmsg)
	}

	return nil
}

// safeDeidentify 脱敏患者姓名（保留首字）
func safeDeidentify(name string) string {
	if len(name) == 0 {
		return "**"
	}
	runes := []rune(name)
	if len(runes) <= 1 {
		return string(runes[0]) + "**"
	}
	return string(runes[0]) + "**"
}

func safeString(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
