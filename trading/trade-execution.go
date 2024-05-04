package trading

import "time"

type TradeExecution struct {
	ExecTime  time.Time
	Spread    string
	Side      TradeSide
	Qty       int
	PosEffect TradeOperation
	Symbol    string
	Exp       string
	Strike    string
	Type      string
	Price     float64
	NetPrice  float64
	OrderType string
}

type TradeExecutions []TradeExecution

func (t TradeExecutions) Len() int {
	return len(t)
}

func (t TradeExecutions) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t TradeExecutions) Less(i, j int) bool {
	return t[i].ExecTime.Before(t[j].ExecTime)
}
