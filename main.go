package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
	"trade-clipper/trading"
)

func main() {
	inputVideoPath := flag.String("iv", "", "TOS Recording")
	inputStatementPath := flag.String("is", "", "TOS Broker Statement")
	outputPath := flag.String("o", "", "Output Directory")
	flag.Parse()

	if *inputVideoPath == "" {
		fmt.Println("Use -iv to provide an video recording")
		os.Exit(1)
	}
	if *inputStatementPath == "" {
		fmt.Println("Use -is to provide a TOS broker statement")
		os.Exit(1)
	}
	if *outputPath == "" {
		fmt.Println("Use -o to provide an output directory")
		os.Exit(1)
	}

	portfolio := trading.NewPortfolio(*inputStatementPath)
	trades := portfolio.GetTrades()

	for _, trade := range trades {
		clipTrade(*trade, *inputVideoPath, *outputPath)
	}
}

func clipTrade(trade trading.Trade, recordingFile string, recordingDir string) {
	recordingStartTime := getStartTimeOfRecording(recordingFile)
	tradeName := trade.OpenTime.Format("2006-01-02-15-04-05") + "-" + trade.Ticker
	outputDir := recordingDir + "/" + tradeName

	fmt.Println("Clipping: " + tradeName)

	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Println("Failed to create directory for output: " + err.Error())
	}

	startOffset := trade.OpenTime.Sub(recordingStartTime) - time.Duration(5*time.Second)
	clipLength := trade.CloseTime.Sub(trade.OpenTime) + time.Duration(10*time.Second)

	var stderr bytes.Buffer
	var stdout bytes.Buffer

	filters := getScreenshotFilters(trade)
	inputFile := recordingFile
	clipOutputFile := outputDir + "/" + tradeName + ".mkv"

	clipCmd := []string{"-ss", fmtDuration(startOffset), "-t", fmtDuration(clipLength), "-i", inputFile,
		"-filter_complex", "drawbox=x=300:y=0:w=200:h=17:color=black@1.0:t=fill",
		"-c:v", "libx264", "-c:a", "aac", "-strict", "experimental", "-b:a", "192k", clipOutputFile}
	entryScreenshotCmd := []string{"-ss", "6", "-i", clipOutputFile, "-filter_complex", filters,
		"-vframes", "1", "-q:v", "2", "-y", outputDir + "/" + tradeName + "-Entry.jpg"}
	exitScreenshotCmd := []string{"-ss", fmtDuration(clipLength - time.Duration(3*time.Second)), "-i", clipOutputFile,
		"-filter_complex", filters, "-vframes", "1", "-q:v", "2", "-y", outputDir + "/" + tradeName + "-Exit.jpg"}

	allCmds := [][]string{clipCmd, entryScreenshotCmd, exitScreenshotCmd}

	for _, cmd := range allCmds {
		cmd := exec.Command("ffmpeg", cmd...)
		cmd.Stderr = &stderr
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			fmt.Println("Failed to run ffmpeg command: " + stderr.String())
		}
	}
}

func getScreenshotFilters(trade trading.Trade) string {
	timeFormat := "2006-01-02 15\\:04\\:05"
	filters :=
		fmt.Sprintf(`color=c=black:s=350x300,drawtext=text='%s':x=20:y=50:fontsize=20:fontcolor=white:fontfile='$fontFile',
							drawtext=text='%s':x=40:y=80:fontsize=20:fontcolor=white:fontfile='$fontFile',
							drawtext=text='%s':x=40:y=110:fontsize=20:fontcolor=white:fontfile='$fontFile',
							drawtext=text='%s':x=40:y=140:fontsize=20:fontcolor=white:fontfile='$fontFile',
							drawtext=text='%s':x=40:y=170:fontsize=20:fontcolor=white:fontfile='$fontFile',
							drawtext=text='%s':x=40:y=200:fontsize=20:fontcolor=white:fontfile='$fontFile'[txt];
							[0][txt]overlay=x=0:y=main_h-overlay_h`,
			trade.Ticker,
			"Open\\: "+trade.OpenTime.Format(timeFormat),
			"Close\\: "+trade.CloseTime.Format(timeFormat),
			fmt.Sprintf("%d", trade.TotalShareCount)+" @"+fmt.Sprintf("%.3f", trade.GetOpeningPriceAvg()),
			"Exit\\: "+fmt.Sprintf("%.3f", trade.GetClosingPriceAvg()),
			"Profit\\: "+fmt.Sprintf("%.2f", trade.GetProfit()))

	return filters
}

func fmtDuration(d time.Duration) string {
	hour := int(d.Hours())
	minute := int(d.Minutes()) % 60
	second := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
}

func getStartTimeOfRecording(recordingFilePath string) time.Time {
	layout := "2006-01-02_15-04-05.mkv"

	fileObj, err := os.Stat(recordingFilePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	t, err := time.Parse(layout, fileObj.Name())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	loc, err := time.LoadLocation("Local")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)

	return t
}
