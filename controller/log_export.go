package controller

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

const (
	usageLogExportMaxRows  = 50000
	usageLogExportMaxRange = 31 * 24 * time.Hour
)

var usageLogExportHeaders = []string{
	"Time", "Type", "Username", "Group", "Model", "Input Tokens", "Output Tokens",
	"Total Tokens", "Quota", "Amount", "Use Time (s)", "Stream", "Channel ID",
	"Channel", "Token Name", "Request ID", "Upstream Request ID",
}

func ExportAllLogs(c *gin.Context) {
	exportUsageLogs(c, true)
}

func ExportUserLogs(c *gin.Context) {
	exportUsageLogs(c, false)
}

func parseUsageLogExportRange(c *gin.Context) (int64, int64, bool) {
	startTimestamp, err := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	if err != nil || startTimestamp <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "start_timestamp is required"})
		return 0, 0, false
	}
	endTimestamp, err := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if err != nil || endTimestamp <= 0 || endTimestamp < startTimestamp {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "valid end_timestamp is required"})
		return 0, 0, false
	}
	if time.Duration(endTimestamp-startTimestamp)*time.Second > usageLogExportMaxRange {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "export time range cannot exceed 31 days"})
		return 0, 0, false
	}
	return startTimestamp, endTimestamp, true
}

func GetUsageLogExportOptions(c *gin.Context) {
	startTimestamp, endTimestamp, ok := parseUsageLogExportRange(c)
	if !ok {
		return
	}
	options, err := model.GetUsageLogExportOptions(startTimestamp, endTimestamp, c.Query("username"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": options})
}

func exportUsageLogs(c *gin.Context, isAdmin bool) {
	startTimestamp, endTimestamp, ok := parseUsageLogExportRange(c)
	if !ok {
		return
	}

	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "csv")))
	if format != "csv" && format != "json" && format != "xlsx" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "format must be csv, json, or xlsx"})
		return
	}

	logType, _ := strconv.Atoi(c.Query("type"))
	modelName := c.Query("model_name")
	tokenName := c.Query("token_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")

	var logs []*model.Log
	var total int64
	var err error
	if isAdmin {
		channel, _ := strconv.Atoi(c.Query("channel"))
		logs, total, err = model.GetAllLogs(
			logType, startTimestamp, endTimestamp, modelName, c.Query("username"), tokenName,
			0, usageLogExportMaxRows+1, channel, group, requestId, upstreamRequestId,
		)
	} else {
		logs, total, err = model.GetUserLogs(
			c.GetInt("id"), logType, startTimestamp, endTimestamp, modelName, tokenName,
			0, usageLogExportMaxRows+1, group, requestId, upstreamRequestId,
		)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if total > usageLogExportMaxRows || len(logs) > usageLogExportMaxRows {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("export contains %d rows; narrow the time range or add username/token filters (maximum %d)", total, usageLogExportMaxRows),
		})
		return
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	filename := "usage-logs-" + stamp + "." + format
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("X-Content-Type-Options", "nosniff")

	switch format {
	case "json":
		data, marshalErr := common.Marshal(logs)
		if marshalErr != nil {
			common.ApiError(c, marshalErr)
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
	case "xlsx":
		data, buildErr := buildUsageLogStatement(logs)
		if buildErr != nil {
			common.ApiError(c, buildErr)
			return
		}
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
	default:
		data, buildErr := buildUsageLogCSV(logs)
		if buildErr != nil {
			common.ApiError(c, buildErr)
			return
		}
		c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
	}
}

func usageLogExportRow(log *model.Log) []string {
	totalTokens := log.PromptTokens + log.CompletionTokens
	amount := float64(log.Quota)
	displayType := operation_setting.GetQuotaDisplayType()
	if displayType != operation_setting.QuotaDisplayTypeTokens && common.QuotaPerUnit > 0 {
		amount = amount / common.QuotaPerUnit * operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
	}
	return []string{
		time.Unix(log.CreatedAt, 0).UTC().Format(time.RFC3339),
		strconv.Itoa(log.Type), spreadsheetSafeText(log.Username), spreadsheetSafeText(log.Group), spreadsheetSafeText(log.ModelName),
		strconv.Itoa(log.PromptTokens), strconv.Itoa(log.CompletionTokens), strconv.Itoa(totalTokens),
		strconv.Itoa(log.Quota), strconv.FormatFloat(amount, 'f', 6, 64), strconv.Itoa(log.UseTime),
		strconv.FormatBool(log.IsStream), strconv.Itoa(log.ChannelId), spreadsheetSafeText(log.ChannelName), spreadsheetSafeText(log.TokenName),
		spreadsheetSafeText(log.RequestId), spreadsheetSafeText(log.UpstreamRequestId),
	}
}

func spreadsheetSafeText(value string) string {
	if value == "" {
		return value
	}
	if strings.ContainsRune("=+-@", rune(value[0])) || value[0] == '\t' || value[0] == '\r' {
		return "'" + value
	}
	return value
}

func buildUsageLogCSV(logs []*model.Log) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buffer)
	if err := writer.Write(usageLogExportHeaders); err != nil {
		return nil, err
	}
	for _, log := range logs {
		if err := writer.Write(usageLogExportRow(log)); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildUsageLogStatement(logs []*model.Log) ([]byte, error) {
	file := excelize.NewFile()
	defer file.Close()
	const sheet = "Statement"
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}

	titleStyle, err := file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F2937"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	headerStyle, err := file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2563EB"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
	})
	if err != nil {
		return nil, err
	}

	file.SetCellValue(sheet, "A1", "Usage Reconciliation Statement")
	file.MergeCell(sheet, "A1", "Q1")
	file.SetCellStyle(sheet, "A1", "Q1", titleStyle)
	file.SetRowHeight(sheet, 1, 30)
	file.SetCellValue(sheet, "A2", "Generated at")
	file.SetCellValue(sheet, "B2", time.Now().UTC().Format(time.RFC3339))
	file.SetCellValue(sheet, "D2", "Currency")
	file.SetCellValue(sheet, "E2", operation_setting.GetQuotaDisplayType())

	for column, header := range usageLogExportHeaders {
		cell, _ := excelize.CoordinatesToCellName(column+1, 4)
		file.SetCellValue(sheet, cell, header)
	}
	file.SetCellStyle(sheet, "A4", "Q4", headerStyle)
	file.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 4, TopLeftCell: "A5", ActivePane: "bottomLeft"})
	file.AutoFilter(sheet, "A4:Q4", nil)

	for index, log := range logs {
		row := index + 5
		for column, value := range usageLogExportRow(log) {
			cell, _ := excelize.CoordinatesToCellName(column+1, row)
			switch column {
			case 1, 5, 6, 7, 8, 10, 12:
				number, _ := strconv.Atoi(value)
				file.SetCellValue(sheet, cell, number)
			case 9:
				number, _ := strconv.ParseFloat(value, 64)
				file.SetCellValue(sheet, cell, number)
			default:
				file.SetCellValue(sheet, cell, value)
			}
		}
	}
	totalRow := len(logs) + 5
	file.SetCellValue(sheet, fmt.Sprintf("E%d", totalRow), "Total")
	for _, column := range []string{"F", "G", "H", "I", "J"} {
		cell := fmt.Sprintf("%s%d", column, totalRow)
		if len(logs) == 0 {
			file.SetCellValue(sheet, cell, 0)
			continue
		}
		file.SetCellFormula(sheet, cell, fmt.Sprintf("SUM(%s5:%s%d)", column, column, totalRow-1))
	}
	file.SetColWidth(sheet, "A", "A", 22)
	file.SetColWidth(sheet, "B", "B", 9)
	file.SetColWidth(sheet, "C", "E", 18)
	file.SetColWidth(sheet, "F", "J", 15)
	file.SetColWidth(sheet, "K", "Q", 20)

	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
