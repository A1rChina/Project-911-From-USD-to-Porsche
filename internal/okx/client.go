package okx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	BaseURL = "https://www.okx.com"
)

type Config struct {
	ApiKey     string `json:"api_key"`
	SecretKey  string `json:"secret_key"`
	Passphrase string `json:"passphrase"`
	Simulated  bool   `json:"is_simulated"`
}

type BillResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []Bill `json:"data"`
}

// Bill 单条流水记录
type Bill struct {
	BillID  string `json:"billId"`
	Ts      string `json:"ts"`
	Type    string `json:"type"`
	SubType string `json:"subType"`
	Pnl     string `json:"pnl"`
	BalChg  string `json:"balChg"`
	Ccy     string `json:"ccy"`
	InstId  string `json:"instId"`
	OrdId   string `json:"ordId"` // 核心聚合字段
	Notes   string `json:"notes"`
}

type Client struct {
	Config Config
	Client *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{
		Config: cfg,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchBills 自动分页获取归档数据 (3个月)
// 包含智能限流重试机制
func (c *Client) FetchBills() ([]Bill, error) {
	requestPath := "/api/v5/account/bills-archive"

	var allBills []Bill
	var afterCursor string

	fmt.Println("📡 开始从 OKX 拉取归档数据 (Archive Mode)...")

	pageCount := 1
	for {
		params := "?limit=100"
		if afterCursor != "" {
			params += "&after=" + afterCursor
		}

		fullURL := BaseURL + requestPath + params
		
		req, err := http.NewRequest("GET", fullURL, nil)
		if err != nil {
			return nil, err
		}

		// 1. 签名
		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.999Z")
		message := timestamp + "GET" + requestPath + params
		sign := computeHmacSha256(message, c.Config.SecretKey)

		// 2. Header
		req.Header.Set("OK-ACCESS-KEY", c.Config.ApiKey)
		req.Header.Set("OK-ACCESS-SIGN", sign)
		req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
		req.Header.Set("OK-ACCESS-PASSPHRASE", c.Config.Passphrase)
		if c.Config.Simulated {
			req.Header.Set("x-simulated-trading", "1")
		}

		// 3. 发送
		resp, err := c.Client.Do(req)
		if err != nil {
			return nil, err
		}
		
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// 4. 智能处理限流错误
		if resp.StatusCode != 200 {
			errMsg := string(body)
			// 如果是限流错误 (Code 50011 或 HTTP 429)
			if strings.Contains(errMsg, "50011") || resp.StatusCode == 429 {
				fmt.Printf("   ⚠️ 触发限流 (Rate Limit)，暂停 5 秒后重试第 %d 页...\n", pageCount)
				time.Sleep(5 * time.Second)
				continue // 保持 afterCursor 不变，重试当前页
			}
			return nil, fmt.Errorf("API HTTP Error: %s", errMsg)
		}

		var result BillResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		
		// 再次检查业务层面的错误码
		if result.Code != "0" {
			if result.Code == "50011" {
				fmt.Printf("   ⚠️ 触发限流 (Biz Code)，暂停 5 秒后重试第 %d 页...\n", pageCount)
				time.Sleep(5 * time.Second)
				continue
			}
			return nil, fmt.Errorf("OKX Biz Error: %s", result.Msg)
		}

		// 5. 追加数据
		if len(result.Data) > 0 {
			allBills = append(allBills, result.Data...)
			fmt.Printf("   -> 第 %d 页获取成功 (本页 %d 条)...\n", pageCount, len(result.Data))
			
			// 更新游标
			afterCursor = result.Data[len(result.Data)-1].BillID
			pageCount++
		} else {
			break
		}

		if len(result.Data) < 100 {
			break
		}
		
		// 每次成功后稍微休息一下，降低触发限流概率
		time.Sleep(1 * time.Second) 
	}

	return allBills, nil
}

// computeHmacSha256 计算签名
func computeHmacSha256(message string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}