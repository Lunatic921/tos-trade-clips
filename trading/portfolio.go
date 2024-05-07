package trading

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Portfolio struct {
	directoryOrFile string
	Trades          Trades

	IncludeSwing bool
}

func NewPortfolio(directoryOrFile string) *Portfolio {
	p := &Portfolio{directoryOrFile: directoryOrFile, Trades: make([]*Trade, 0, 10000)}

	fileObj, err := os.Stat(p.directoryOrFile)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	if fileObj.IsDir() {
		p.parseTradeDirectory()
	} else {
		p.parseTradeFile(p.directoryOrFile)
	}

	return p
}

func (p *Portfolio) GetSharesTraded(year int, month int, day int) int {
	sharesTraded := 0

	trades := p.FilterTrades(year, month, day)

	for _, trade := range trades {
		sharesTraded += trade.TotalShareCount
	}

	return sharesTraded
}

func (p *Portfolio) GetTradingDays(year int, month int, day int) []time.Time {
	days := make([]time.Time, 0)

	trades := p.FilterTrades(year, month, day)

	for _, trade := range trades {
		foundDay := false

		loc, err := time.LoadLocation("Local")
		if err != nil {
			fmt.Println("Couldnt get current timezone")
			os.Exit(1)
		}

		newYear, newMonth, newDay := trade.CloseTime.Date()
		newDate := time.Date(newYear, newMonth, newDay, 0, 0, 0, 0, loc)

		for _, day := range days {
			oldYear, oldMonth, oldDay := day.Date()

			if newYear == oldYear && newMonth == oldMonth && oldDay == newDay {
				foundDay = true
				break
			}
		}

		if !foundDay {
			days = append(days, newDate)
		}
	}

	sort.Slice(days, func(i, j int) bool {
		return days[i].Before(days[j])
	})

	return days
}

func (p *Portfolio) GetGreenVsRedDays(year int, month int, day int) (greenDays int, redDays int) {
	greenDays = 0
	redDays = 0

	trades := p.FilterTrades(year, month, day)

	dailyProfitLoss := 0.0
	currDay := trades[0].OpenTime

	for _, trade := range trades {
		if trade.OpenTime.Year() != currDay.Year() || trade.OpenTime.Month() != currDay.Month() || trade.OpenTime.Day() != currDay.Day() {
			currDay = trade.OpenTime

			if dailyProfitLoss >= 0.0 {
				greenDays += 1
			} else {
				redDays += 1
			}

			dailyProfitLoss = 0
		}

		dailyProfitLoss += trade.GetProfit()
	}

	//Last day counted here
	if dailyProfitLoss >= 0.0 {
		greenDays += 1
	} else {
		redDays += 1
	}

	return greenDays, redDays
}

func (p *Portfolio) GetTrades() Trades {
	if p.IncludeSwing {
		return p.Trades
	} else {
		dayTrades := make(Trades, 0)
		lastSwing := 0

		for i, t := range p.Trades {
			if t.IsSwing() {
				dayTrades = append(dayTrades, p.Trades[lastSwing:i]...)
				lastSwing = i + 1
			}
		}

		dayTrades = append(dayTrades, p.Trades[lastSwing:]...)

		return dayTrades
	}
}

func (p *Portfolio) GetTradePl(year int, month int, day int) float64 {
	return p.GetProfit(year, -1, -1) / float64(len(p.FilterTrades(year, month, day)))
}

func (p *Portfolio) FilterTrades(year int, month int, day int) Trades {
	// -1 for any input variable means ignore it

	trades := make(Trades, 0)

	for _, trade := range p.Trades {
		fitsYear := year == -1 || trade.CloseTime.Year() == year
		fitsMonth := month == -1 || int(trade.CloseTime.Month()) == month
		fitsDay := day == -1 || trade.CloseTime.Day() == day

		if fitsYear && fitsMonth && fitsDay && !trade.isOpen() {
			if p.IncludeSwing || !trade.IsSwing() {
				trades = append(trades, trade)
			}
		}
	}

	return trades
}

func (p *Portfolio) GetWinPercentage(year int, month int, day int) float64 {
	wins := 0.0
	tradeCount := 0.0

	trades := p.FilterTrades(year, month, day)

	for _, trade := range trades {
		if trade.GetProfit() >= 0.0 {
			wins += 1.0
		}

		tradeCount += 1.0
	}

	return wins / tradeCount
}

func (p *Portfolio) GetProfit(year int, month int, day int) float64 {
	runningPl := 0.0

	trades := p.FilterTrades(year, month, day)

	for _, trade := range trades {
		runningPl += trade.GetProfit()
	}

	return runningPl
}

func (p *Portfolio) GetProfitPerShare(year int, month int, day int) float64 {
	sharesTraded := float64(p.GetSharesTraded(year, month, day))

	return sharesTraded / float64(len(p.FilterTrades(year, month, day)))
}

func (p *Portfolio) parseTradeDirectory() {
	files, err := os.ReadDir(p.directoryOrFile)
	if err != nil {
		fmt.Printf("Failed to open directory: %+v\n", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".csv") {
			p.parseTradeFile(p.directoryOrFile + "/" + file.Name())
		}

	}
}

func (p *Portfolio) parseTradeFile(filePath string) {

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	startLine, endLine := 0, 0

	for i, line := range lines {
		if line == "Account Trade History" {
			startLine = i + 2
			for j, line2 := range lines[startLine:] {
				if line2 == "" {
					endLine = j + startLine
					break
				}
			}
		}
	}

	reader := csv.NewReader(strings.NewReader(strings.Join(lines[startLine:endLine], "\n")))
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV data:", err)
		return
	}

	t := time.Now()
	timezone := t.Location()

	var trades TradeExecutions
	for _, record := range records {
		layout := "1/2/06 15:04:05"

		execTime, _ := time.ParseInLocation(layout, record[1], timezone)
		qty, _ := strconv.Atoi(record[4])
		price, _ := strconv.ParseFloat(record[10], 64)
		netPrice, _ := strconv.ParseFloat(record[11], 64)

		tradeExec := TradeExecution{
			ExecTime:  execTime,
			Spread:    record[2],
			Side:      TradeSide(record[3]),
			Qty:       qty,
			PosEffect: ParseOperation(record[5]),
			Symbol:    record[6],
			Exp:       record[7],
			Strike:    record[8],
			Type:      record[9],
			Price:     price,
			NetPrice:  netPrice,
			OrderType: record[12],
		}

		if tradeExec.Strike == "" {
			trades = append(trades, tradeExec)
		}

	}

	sort.Sort(trades)

	//Break trade executions into their respective Trades
	for _, tradeEx := range trades {
		foundTrade := false

		//Find an open trade
		for _, trade := range p.Trades {

			if trade.Ticker == tradeEx.Symbol && trade.isOpen() {
				foundTrade = true
				trade.execute(tradeEx)

				break
			}
		}

		if !foundTrade {
			trade := &Trade{Ticker: tradeEx.Symbol, Side: tradeEx.Side}
			trade.execute(tradeEx)

			p.Trades = append(p.Trades, trade)
		}
	}

	//Sort trades by when they were closed
	sort.Sort(p.Trades)
}
