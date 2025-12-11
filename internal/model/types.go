package model

import "time"

// TransactionType 定义交易类型枚举
type TransactionType string

const (
	TypeDeposit    TransactionType = "DEPOSIT"    // 入金
	TypeWithdrawal TransactionType = "WITHDRAWAL" // 出金 (Harvest)
	TypePnL        TransactionType = "PNL"        // 交易盈亏
)

// Transaction 对应 ledger.csv 中的一行
type Transaction struct {
	Timestamp time.Time       `json:"timestamp"`
	Type      TransactionType `json:"type"`
	Amount    float64         `json:"amount"` // 金额 (出金通常为负数)
	Asset     string          `json:"asset"`
	Note      string          `json:"note"`
}

// PortfolioStatus 账户当前的健康状态
type PortfolioStatus struct {
	InitialCapital float64 // 初始本金 (Seed)
	CurrentBalance float64 // 当前余额 (Asset Value)
	TotalPnL       float64 // 累计交易盈亏
	TotalHarvested float64 // 累计出金 (Realized Life)
	
	WinCount       int     // 盈利次数
	LossCount      int     // 亏损次数
	
	Target         float64 // 目标金额 (Porsche 911 Price)
}

// Progress 计算离保时捷的进度百分比
func (p PortfolioStatus) Progress() float64 {
	if p.Target == 0 {
		return 0
	}
	return (p.CurrentBalance / p.Target) * 100
}

// WinRate 计算胜率
func (p PortfolioStatus) WinRate() float64 {
	totalTrades := p.WinCount + p.LossCount
	if totalTrades == 0 {
		return 0
	}
	return float64(p.WinCount) / float64(totalTrades) * 100
}