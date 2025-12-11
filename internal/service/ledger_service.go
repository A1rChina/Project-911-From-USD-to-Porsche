package service

import (
	"911/internal/model"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

// TargetPorschePrice 设定目标金额
const TargetPorschePrice = 120000.0

// LoadTransactions 从 CSV 读取原始数据
func LoadTransactions(filePath string) ([]model.Transaction, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开账本文件 [%s]: %v", filePath, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// 允许变长字段，虽然我们不建议
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV 解析失败: %v", err)
	}

	var transactions []model.Transaction

	// 遍历 CSV 行，跳过 Header (第一行)
	for i, row := range records {
		if i == 0 {
			continue // Skip Header
		}
		// 简单的防错检查
		if len(row) < 5 {
			continue
		}

		// 1. 解析时间 (RFC3339: 2025-12-12T14:00:00Z)
		ts, err := time.Parse(time.RFC3339, row[0])
		if err != nil {
			// 如果解析失败，尝试兼容简单的 YYYY-MM-DD 格式，或者直接报错
			return nil, fmt.Errorf("Line %d 时间格式错误 (需用 RFC3339): %v", i+1, err)
		}

		// 2. 解析金额
		amt, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			return nil, fmt.Errorf("Line %d 金额错误: %v", i+1, err)
		}

		transactions = append(transactions, model.Transaction{
			Timestamp: ts,
			Type:      model.TransactionType(row[1]),
			Amount:    amt,
			Asset:     row[3],
			Note:      row[4],
		})
	}

	return transactions, nil
}

// AnalyzePortfolio 计算账户核心指标
func AnalyzePortfolio(txs []model.Transaction) model.PortfolioStatus {
	status := model.PortfolioStatus{
		Target: TargetPorschePrice,
	}

	for _, tx := range txs {
		// 1. 更新当前余额
		// (无论是入金+，盈利+，亏损-，还是出金-，Amount 的正负号已经决定了余额变动)
		status.CurrentBalance += tx.Amount

		// 2. 分类统计逻辑
		switch tx.Type {
		case model.TypeDeposit:
			status.InitialCapital += tx.Amount

		case model.TypeWithdrawal:
			// 出金在 CSV 中记录为负数 (e.g. -50)，为了统计“提取了多少”，我们要取绝对值
			status.TotalHarvested += math.Abs(tx.Amount)

		case model.TypePnL:
			status.TotalPnL += tx.Amount
			// 统计胜率
			if tx.Amount > 0 {
				status.WinCount++
			} else if tx.Amount < 0 {
				status.LossCount++
			}
		}
	}

	return status
}
