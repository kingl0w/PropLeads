package scrapeutils

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Define the timeout duration as a constant
const ElementWaitTimeout = 30 * time.Second

// CustomLogger filters out unwanted log messages
func CustomLogger(format string, args ...interface{}) {
    if !strings.Contains(format, "could not unmarshal event") {
        log.Printf(format, args...)
    }
}

// ParseFloat safely parses a string to float64
func ParseFloat(s string) float64 {
    // Remove dollar sign and commas
    s = strings.ReplaceAll(s, "$", "")
    s = strings.ReplaceAll(s, ",", "")
    s = strings.TrimSpace(s)

    // Parse the string to float64
    f, err := strconv.ParseFloat(s, 64)
    if err != nil {
        // Handle error or return a default value
        return 0
    }
    return f
}

// WaitVisibleWithTimeout waits for an element to be visible with a timeout
func WaitVisibleWithTimeout(selector string, selType chromedp.QueryOption, timeout time.Duration) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        ctx, cancel := context.WithTimeout(ctx, timeout)
        defer cancel()
        return chromedp.WaitVisible(selector, selType).Do(ctx)
    }
}