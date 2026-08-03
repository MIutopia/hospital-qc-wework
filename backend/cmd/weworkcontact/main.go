// weworkcontact 企业微信通讯录成员拉取测试工具（开发联调用）
//
// 背景：院方库内无企业微信相关人员信息表，医生-企微映射（任务 2.4）依赖信息科
// 通讯录 CSV 一直未到位。本工具直接从企业微信通讯录 API 拉取成员，验证：
//   1. 当前应用 secret 是否具备通讯录读取权限（无权限会返回 60011 等错误码）
//   2. 能否拉取指定成员详情（user/get）
//   3. 能否枚举部门与成员（department/list + user/simplelist），为后续自动建映射做摸底
//
// 用法：
//   go run ./cmd/weworkcontact -config config.yaml                          # 枚举部门 + 成员（摸底）
//   go run ./cmd/weworkcontact -config config.yaml -userid <userid>         # 拉取指定成员详情
//
// 凭证优先级：环境变量 WEWORK_CORP_ID / WEWORK_AGENT_ID / WEWORK_AGENT_SECRET
//            > config.yaml 的 wework.corp_id / agent_secret（agent_secret 在
//              config.Load 中被 yaml:"-" 忽略，此处为测试便利回退读取文件原值；
//              secret 本体只用于拼接 URL，不打印）
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"hospital-qc-wework/internal/config"
	"hospital-qc-wework/internal/service/push"

	"gopkg.in/yaml.v3"
)

// ---- 企业微信通讯录 API 响应结构 ----

type apiResp struct {
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
}

// memberResp 对应 /cgi-bin/user/get
type memberResp struct {
	apiResp
	UserID     string `json:"userid"`
	Name       string `json:"name"`
	Mobile     string `json:"mobile"`
	Email      string `json:"email"`
	Position   string `json:"position"`
	Department []int  `json:"department"`
	Status     int    `json:"status"`
}

// deptResp 对应 /cgi-bin/department/list
type deptResp struct {
	apiResp
	Department []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		ParentID int    `json:"parentid"`
	} `json:"department"`
}

// simpleListResp 对应 /cgi-bin/user/simplelist（仅 userid/name/部门，权限要求较低）
type simpleListResp struct {
	apiResp
	UserList []struct {
		UserID     string `json:"userid"`
		Name       string `json:"name"`
		Department []int  `json:"department"`
	} `json:"userlist"`
}

// 本地回退解析 config.yaml 中 wework.agent_secret（config.Load 因 yaml:"-" 不加载）
type rawWeWork struct {
	AgentSecret string `yaml:"agent_secret"`
}

const apiBase = "https://qyapi.weixin.qq.com"

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	userID := flag.String("userid", "", "要拉取的成员 userid（空则枚举部门+成员）")
	deptID := flag.Int("department", 1, "枚举成员时的部门 ID")
	fetchChild := flag.Bool("fetch-child", true, "枚举成员时是否包含子部门")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 环境变量未注入 agent_secret 时，回退读取 config.yaml 文件原值（测试便利）
	if cfg.WeWork.AgentSecret == "" {
		if raw := readRawAgentSecret(*configPath); raw != "" {
			cfg.WeWork.AgentSecret = raw
		}
	}

	if cfg.WeWork.CorpID == "" || cfg.WeWork.AgentSecret == "" {
		fmt.Fprintln(os.Stderr, "企业微信凭证未配置：请设置环境变量 WEWORK_CORP_ID / WEWORK_AGENT_SECRET（或在 config.yaml 的 wework 段填写 corp_id / agent_secret）")
		os.Exit(1)
	}

	// 复用现有 TokenManager 获取 access_token（凭证已校验非空）
	tokenMgr := push.NewTokenManager(&cfg.WeWork)
	token, err := tokenMgr.GetToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取 access_token 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[OK] access_token 获取成功 (corp %s)\n\n", maskCorpID(cfg.WeWork.CorpID))

	httpClient := &http.Client{Timeout: 15 * time.Second}

	if *userID != "" {
		getMember(httpClient, token, *userID)
	} else {
		enumerate(httpClient, token, *deptID, *fetchChild)
	}
}

// getMember 拉取指定成员详情
func getMember(c *http.Client, token, userID string) {
	fmt.Printf("=== 拉取指定成员详情 user/get?userid=%s ===\n", userID)
	var m memberResp
	if err := getJSON(c, fmt.Sprintf("/cgi-bin/user/get?access_token=%s&userid=%s", token, url.QueryEscape(userID)), &m); err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] 请求失败: %v\n", err)
		os.Exit(1)
	}
	if m.Errcode != 0 {
		fmt.Fprintf(os.Stderr, "[FAIL] errcode=%d errmsg=%s\n", m.Errcode, m.Errmsg)
		printErrcodeGuide(m.Errcode)
		os.Exit(1)
	}
	fmt.Println("[OK] 指定成员拉取成功：")
	fmt.Printf("  userid     : %s\n", m.UserID)
	fmt.Printf("  姓名       : %s\n", m.Name)
	fmt.Printf("  职务       : %s\n", m.Position)
	fmt.Printf("  部门ID     : %v\n", m.Department)
	fmt.Printf("  手机号     : %s\n", maskMobile(m.Mobile))
	fmt.Printf("  邮箱       : %s\n", maskEmail(m.Email))
	fmt.Printf("  账号状态   : %d (1=已激活, 2=已禁用, 4=未激活, 5=退出)\n", m.Status)
}

// enumerate 枚举部门 + 部门成员（摸底：看通讯录里实际有哪些人、有哪些 userid）
func enumerate(c *http.Client, token string, deptID int, fetchChild bool) {
	fmt.Println("=== 步骤一：枚举部门 department/list ===")
	var d deptResp
	if err := getJSON(c, fmt.Sprintf("/cgi-bin/department/list?access_token=%s", token), &d); err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] 请求失败: %v\n", err)
		os.Exit(1)
	}
	if d.Errcode != 0 {
		fmt.Fprintf(os.Stderr, "[FAIL] 部门列表失败 errcode=%d errmsg=%s\n", d.Errcode, d.Errmsg)
		printErrcodeGuide(d.Errcode)
		os.Exit(1)
	}
	fmt.Printf("[OK] 共 %d 个部门：\n", len(d.Department))
	for _, dept := range d.Department {
		fmt.Printf("  id=%-6d %-20s (parent=%d)\n", dept.ID, dept.Name, dept.ParentID)
	}

	fmt.Printf("\n=== 步骤二：枚举部门 %d 成员 user/simplelist (fetch_child=%v) ===\n", deptID, fetchChild)
	var sl simpleListResp
	if err := getJSON(c, fmt.Sprintf("/cgi-bin/user/simplelist?access_token=%s&department_id=%d&fetch_child=%v", token, deptID, fetchChild), &sl); err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] 请求失败: %v\n", err)
		os.Exit(1)
	}
	if sl.Errcode != 0 {
		fmt.Fprintf(os.Stderr, "[FAIL] 成员列表失败 errcode=%d errmsg=%s\n", sl.Errcode, sl.Errmsg)
		os.Exit(1)
	}
	fmt.Printf("[OK] 部门 %d 下共 %d 个成员：\n", deptID, len(sl.UserList))
	for _, u := range sl.UserList {
		fmt.Printf("  userid=%-20s 姓名=%s  部门=%v\n", u.UserID, u.Name, u.Department)
	}
	if len(sl.UserList) > 0 {
		fmt.Println("\n提示：可用 -userid <上面某个 userid> 测试拉取指定成员详情（user/get）。")
	}
}

// printErrcodeGuide 针对通讯录 API 常见错误码给出排查指引
func printErrcodeGuide(errcode int) {
	switch errcode {
	case 60011:
		fmt.Println("  → 当前 secret 无通讯录读取权限。请在企微管理后台确认：")
		fmt.Println("     应用管理 → 应用 → 权限 勾选「通讯录-读取成员」；")
		fmt.Println("     或用「我的企业 → 通讯录同步」中的通讯录同步 secret 替换 agent_secret。")
	case 60020:
		fmt.Println("  → 当前访问 IP 不在企业微信后台配置的可信 IP 白名单内。请在企微管理后台：")
		fmt.Println("     应用管理 → 应用 → 企业可信IP 添加本机公网 IP / 部署服务器 IP；")
		fmt.Println("     或直接用已加白的服务器执行本工具（部署时应用也运行在那台机器）。")
	case 60111:
		fmt.Println("  → 指定 userid 不存在。先用不带 -userid 的模式枚举通讯录，拿真实 userid 再试。")
	default:
		fmt.Printf("  → 其他错误，可到 https://open.work.weixin.qq.com/devtool/query 输入 errcode=%d 查询\n", errcode)
	}
}

// ---- 辅助 ----

func getJSON(c *http.Client, path string, out interface{}) error {
	resp, err := c.Get(apiBase + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// readRawAgentSecret 读取 config.yaml 中 wework.agent_secret 原值（不回显）
func readRawAgentSecret(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var raw struct {
		WeWork rawWeWork `yaml:"wework"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ""
	}
	return raw.WeWork.AgentSecret
}

func maskCorpID(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}

// maskMobile 手机号打码：138****8000
func maskMobile(s string) string {
	r := []rune(s)
	if len(r) < 7 {
		return "****"
	}
	return string(r[:3]) + "****" + string(r[len(r)-4:])
}

// maskEmail 邮箱打码：zhangs****@example.com
func maskEmail(s string) string {
	at := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return "****"
	}
	local, domain := s[:at], s[at:]
	if len(local) <= 3 {
		return "****" + domain
	}
	return local[:3] + "****" + domain
}
