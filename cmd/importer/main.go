package main

import (
	"911/internal/model"
	"911/internal/okx"
	"911/internal/service"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings" // 🟢 聚合逻辑需要用到 strings 包，请确保保留
	"time"
)

func main() {
	configFile := flag.String("config", "config.json", "Path to config file")
	ledgerPath := flag.String("out", "data/ledger.csv", "Path to ledger csv")
	flag.Parse()

	// 1. 加载配置
	cfgData, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("无法读取配置文件: %v", err)
	}
	var cfg okx.Config
	json.Unmarshal(cfgData, &cfg)

	// 2. 获取本地最新时间戳 (用于去重)
	lastTimestamp := getLastRecordTimestamp(*ledgerPath)
	if !lastTimestamp.IsZero() {
		fmt.Printf("📅 本地最新记录时间: %s\n", lastTimestamp.Format("2006-01-02 15:04:05"))
	}

	// 3. API 拉取 (归档模式)
	client := okx.NewClient(cfg)
	rawBills, err := client.FetchBills()
	if err != nil {
		log.Fatalf("获取数据失败: %v", err)
	}
	fmt.Printf("✅ API 返回原始流水: %d 条\n", len(rawBills))

	// 4. 🟢 核心：数据聚合 (Merge Bills by OrderID)
	// 这里会调用下方的 aggregateAndMapBills 函数进行合并
	transactions := aggregateAndMapBills(rawBills)
	
	// 打印聚合效果
	fmt.Printf("🔄 聚合后交易记录: %d 条 (合并了 %d 条零碎流水)\n", 
		len(transactions), len(rawBills)-len(transactions))

	// 5. 过滤与去重
	var newTransactions []model.Transaction
	for _, trans := range transactions {
		// 过滤 0 金额交易
		if trans.Amount == 0 {
			continue
		}
		// 🟢 核心：时间去重 (只写入比 CSV 中更新的数据)
		if !trans.Timestamp.After(lastTimestamp) {
			continue
		}
		newTransactions = append(newTransactions, trans)
	}

	// 6. 写入
	if len(newTransactions) > 0 {
		appendNewRecords(*ledgerPath, newTransactions)
	} else {
		fmt.Println("✨ 没有发现比本地账本更新的记录 (All up to date).")
	}
}

// 🟢 核心函数：将分散的流水聚合为逻辑交易
func aggregateAndMapBills(bills []okx.Bill) []model.Transaction {
	// Key 是 OrdId (订单号), Value 是聚合后的 Transaction 指针
	mergedMap := make(map[string]*model.Transaction)
	
	var resultList []model.Transaction // 最终结果
	var standaloneList []model.Transaction // 无法聚合的（如资金费）

	for _, bill := range bills {
		amount, _ := strconv.ParseFloat(bill.BalChg, 64)
		tsInt, _ := strconv.ParseInt(bill.Ts, 10, 64)
		ts := time.UnixMilli(tsInt)
		
		// 1. 优先判断是否属于“交易聚合”范畴
		// 只要有 OrdId，无论 OKX 标记它是什么类型（Fee, Withdrawal, etc.），都视为交易的一部分
		if bill.OrdId != "" {
			if existing, found := mergedMap[bill.OrdId]; found {
				// A. 已存在：合并金额
				existing.Amount += amount 
				
				// 时间取最新的
				if ts.After(existing.Timestamp) {
					existing.Timestamp = ts
				}
				
				// 备注合并 (避免重复)
				if !strings.Contains(existing.Note, bill.InstId) {
					existing.Note += " " + bill.InstId
				}
			} else {
				// B. 新订单：创建聚合记录
				// 强制类型为 PNL，因为这是交易产生的变动
				t := &model.Transaction{
					Timestamp: ts,
					Type:      model.TypePnL, 
					Amount:    amount,
					Asset:     bill.Ccy,
					Note:      fmt.Sprintf("Trade (%s)", bill.InstId),
				}
				mergedMap[bill.OrdId] = t
			}
		} else {
			// 2. 没有 OrdId 的，归为孤立事件 (Standalone)
			// 如：资金费 (Funding Fee)、真正的出入金、划转
			transType := determineType(bill.Type)
			
			// 如果是资金费(Type 8)，我们在 Note 里标明
			note := getNoteFromType(bill.Type)
			if bill.InstId != "" {
				note = fmt.Sprintf("%s (%s)", note, bill.InstId)
			}

			t := model.Transaction{
				Timestamp: ts,
				Type:      transType,
				Amount:    amount,
				Asset:     bill.Ccy,
				Note:      note,
			}
			standaloneList = append(standaloneList, t)
		}
	}

	// 将 Map 中的聚合结果转回 List
	for _, t := range mergedMap {
		resultList = append(resultList, *t)
	}
	
	// 加上孤立记录
	resultList = append(resultList, standaloneList...)
	
	return resultList
}

func determineType(billType string) model.TransactionType {
	switch billType {
	case "1": return model.TypeDeposit
	case "2": return model.TypeWithdrawal
	default:  return model.TypePnL
	}
}

func getNoteFromType(billType string) string {
	switch billType {
	case "1": return "Deposit"
	case "2": return "Withdrawal"
	case "8": return "Funding Fee"
	default:  return "Auto Import"
	}
}

func getLastRecordTimestamp(filePath string) time.Time {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return time.Time{}
	}
	txs, err := service.LoadTransactions(filePath)
	if err != nil || len(txs) == 0 {
		return time.Time{}
	}
	return txs[len(txs)-1].Timestamp
}

func appendNewRecords(filePath string, newTxs []model.Transaction) {
	fileMode := os.O_APPEND | os.O_WRONLY
	needHeader := false
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fileMode = os.O_CREATE | os.O_WRONLY
		needHeader = true
	}

	f, err := os.OpenFile(filePath, fileMode, 0644)
	if err != nil {
		log.Fatalf("无法打开文件: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if needHeader {
		w.Write([]string{"timestamp", "type", "amount", "asset", "note"})
	}

	// 排序：时间正序写入
	sort.Slice(newTxs, func(i, j int) bool {
		return newTxs[i].Timestamp.Before(newTxs[j].Timestamp)
	})

	count := 0
	for _, tx := range newTxs {
		record := []string{
			tx.Timestamp.Format(time.RFC3339),
			string(tx.Type),
			fmt.Sprintf("%.8f", tx.Amount),
			tx.Asset,
			tx.Note,
		}
		w.Write(record)
		count++
	}
	w.Flush()
	fmt.Printf("📥 成功导入 %d 条新记录！\n", count)
}