package main

import (
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	jsontime "github.com/liamylian/jsontime/v2/v2"
)

var json = jsontime.ConfigWithCustomTimeFormat

func init() {
	jsontime.SetDefaultTimeFormat(time.RFC3339, time.Local)
	jsontime.AddTimeFormatAlias("dateonly", "2006-01-02")
	jsontime.AddTimeFormatAlias("timeonly", "15:04")
}

type receipt struct {
	Retailer     string    `json:"retailer"`
	PurchaseDate time.Time `json:"purchaseDate" time_format:"dateonly"`
	PurchaseTime time.Time `json:"purchaseTime" time_format:"timeonly"`
	Items        []Item    `json:"items"`
	Total        float64   `json:"total,string"`
}

type Item struct {
	ShortDescription string  `json:"shortDescription"`
	Price            float64 `json:"price,string"`
}

var receipts = make(map[string]receipt)

func main() {
	router := gin.Default()
	router.POST("/receipts/process", addReceipt)
	router.GET("/receipts/:id/points", getReceiptScore)

	router.Run("localhost:8080")
}

func addReceipt(c *gin.Context) {
	rawData, err := c.GetRawData()
	var newReceipt receipt

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": err.Error()})
		return
	}

	err = json.Unmarshal(rawData, &newReceipt)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": err.Error()})
		return
	}

	// I am not personally aware of the collision chance here, but best to be sure
	var receiptId string
	for {
		receiptId = uuid.New().String()
		if _, ok := receipts[receiptId]; !ok {
			break
		}
	}
	receipts[receiptId] = newReceipt
	c.JSON(http.StatusOK, gin.H{"id": receiptId})
}

func getReceiptScore(c *gin.Context) {
	receiptId := c.Param("id")

	if receiptId == "" {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"err": "Invalid ID"})
		return
	}

	score := 0
	scoredReceipt, ok := receipts[receiptId]

	if !ok {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"err": "Invalid ID"})
		return
	}

	// 1 point per alphanumeric character in retailer's name
	score += len(alphanumericOnly(scoredReceipt.Retailer))

	// 50 points if total is exact dollar
	if math.Mod(scoredReceipt.Total, 1.0) == 0 {
		score += 50
	}

	// 25 points if total is a multiple of 0.25
	if math.Mod(scoredReceipt.Total, 0.25) == 0 {
		score += 25
	}

	// 5 points for every two items on the receipt.
	score += (len(scoredReceipt.Items) / 2) * 5

	// 6 points if the day in the purchase date is odd.
	if scoredReceipt.PurchaseDate.Day()%2 == 1 {
		score += 6
	}

	// 10 points if the time of purchase is after 2:00pm and before 4:00pm.
	// Developer's note: I have chosen to assume 2:00pm is after 2:00pm, and 4:00pm is not before 4:00pm, as paradoxical as this may read.
	if scoredReceipt.PurchaseTime.Hour() >= 14 && scoredReceipt.PurchaseTime.Hour() < 16 {
		score += 10
	}

	// If the trimmed length of the item description is a multiple of 3, multiply the price by 0.2 and round up to the nearest integer. The result is the number of points earned.
	for _, item := range scoredReceipt.Items {
		if len(strings.TrimSpace(item.ShortDescription))%3 == 0 {
			score += int(math.Ceil(item.Price * 0.2))
		}
	}

	c.IndentedJSON(http.StatusOK, gin.H{"score": score})
}

func alphanumericOnly(str string) string {
	return regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(str, "")
}
