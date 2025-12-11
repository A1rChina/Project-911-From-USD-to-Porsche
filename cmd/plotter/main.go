package main

import (
	"911/internal/service"
	"flag"
	"fmt"
	"image/color"
	"log"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

func main() {
	// 1. 定义参数
	inPath := flag.String("in", "data/ledger.csv", "Input CSV file path")
	outPath := flag.String("out", "assets/equity_curve.png", "Output PNG file path")
	flag.Parse()

	// 2. 加载数据
	txs, err := service.LoadTransactions(*inPath)
	if err != nil {
		log.Fatalf("无法加载数据: %v", err)
	}

	if len(txs) == 0 {
		log.Fatalf("数据为空，无法绘图")
	}

	// 3. 准备绘图数据 (XY轴坐标点)
	pts := make(plotter.XYs, len(txs))

	var currentBalance float64 = 0

	// 注意：这里我们假设数据已经是按时间排序的 (CSV追加模式通常如此)
	// 如果不是，在 Service 层需要加一个 Sort
	for i, tx := range txs {
		// 累加金额，计算实时余额
		currentBalance += tx.Amount

		// X轴: 交易序列号 (第几笔交易) - 简单直观
		// (如果想用时间作X轴会复杂很多，V1版本建议用交易次数，更能反映交易频率)
		pts[i].X = float64(i + 1)
		pts[i].Y = currentBalance
	}

	// 4. 配置图表
	p := plot.New()

	p.Title.Text = "Project 911: Equity Curve"
	p.X.Label.Text = "Trade Count"
	p.Y.Label.Text = "Equity (USD)"

	// 添加网格线
	p.Add(plotter.NewGrid())

	// 创建曲线
	line, points, err := plotter.NewLinePoints(pts)
	if err != nil {
		log.Fatalf("创建曲线失败: %v", err)
	}

	// 样式调整
	line.Color = color.RGBA{R: 0, G: 128, B: 255, A: 255} // 科技蓝
	line.Width = vg.Points(2)                             // 线宽
	points.Shape = nil                                    // 不显示具体的点，只显示线，保持整洁

	// 添加目标线 (Porsche Price) - 可选
	targetLine, _ := plotter.NewLine(plotter.XYs{
		{X: 0, Y: 120000},
		{X: float64(len(txs) + 5), Y: 120000},
	})
	targetLine.Color = color.RGBA{R: 255, G: 0, B: 0, A: 100} // 红色虚线效果
	targetLine.LineStyle.Dashes = []vg.Length{vg.Points(5), vg.Points(5)}

	// 将元素加入画布
	p.Add(line, targetLine)

	// 5. 保存图片
	// 宽度 8 Inch, 高度 4 Inch
	if err := p.Save(8*vg.Inch, 4*vg.Inch, *outPath); err != nil {
		log.Fatalf("保存图片失败: %v", err)
	}

	fmt.Printf("✅ 绘图成功! 已保存至: %s\n", *outPath)
}
