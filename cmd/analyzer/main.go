package main

import (
	"911/internal/model"
	"911/internal/service"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter" // 使用标准库，无需外部依赖
)

func main() {
	// 1. 定义命令行参数
	ledgerPath := flag.String("in", "data/ledger.csv", "Path to the ledger CSV file")
	flag.Parse()

	// 2. 加载数据
	transactions, err := service.LoadTransactions(*ledgerPath)
	if err != nil {
		log.Fatalf("❌ 错误: 无法加载账本文件: %v", err)
	}

	// 3. 执行分析
	status := service.AnalyzePortfolio(transactions)

	// 4. 输出仪表盘
	printDashboard(status)
}

func printDashboard(s model.PortfolioStatus) {
	fmt.Println("")
	fmt.Println("========================================")
	fmt.Println("   🏎️  PROJECT 911: DASHBOARD")
	fmt.Println("========================================")

	// 使用标准库 tabwriter
	// 参数说明: output, minwidth, tabwidth, padding, padchar, flags
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// 打印表头
	fmt.Fprintln(w, "METRIC\tVALUE\tNOTE")
	fmt.Fprintln(w, "------\t-----\t----")

	// 1. 初始本金
	fmt.Fprintf(w, "Initial Capital\t$%.2f\tSeed Money\n", s.InitialCapital)

	// 2. 当前余额
	fmt.Fprintf(w, "Current Balance\t$%.2f\tProgress: %.2f%%\n", s.CurrentBalance, s.Progress())

	// 3. 累计盈亏
	pnlSign := ""
	if s.TotalPnL >= 0 {
		pnlSign = "+"
	}
	// 计算胜率显示
	winRateStr := "N/A"
	totalTrades := s.WinCount + s.LossCount
	if totalTrades > 0 {
		winRateStr = fmt.Sprintf("%.1f%% (%d/%d)", s.WinRate(), s.WinCount, totalTrades)
	}
	fmt.Fprintf(w, "Net PnL\t%s$%.2f\tWin Rate: %s\n", pnlSign, s.TotalPnL, winRateStr)

	// 4. 已出金
	fmt.Fprintf(w, "Harvested\t$%.2f\tRealized Life 🏖️\n", s.TotalHarvested)

	// 5. 目标
	fmt.Fprintf(w, "TARGET (911)\t$%.0f\tThe Dream\n", s.Target)

	// 刷新缓冲区，将内容输出到终端
	w.Flush()

	fmt.Println("========================================")
	fmt.Println("")
}
