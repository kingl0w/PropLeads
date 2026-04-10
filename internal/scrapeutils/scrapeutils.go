package scrapeutils

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const ElementWaitTimeout = 30 * time.Second

func CustomLogger(format string, args ...interface{}) {
    if !strings.Contains(format, "could not unmarshal event") {
        log.Printf(format, args...)
    }
}

func ParseFloat(s string) float64 {
    s = strings.ReplaceAll(s, "$", "")
    s = strings.ReplaceAll(s, ",", "")
    s = strings.TrimSpace(s)

    f, err := strconv.ParseFloat(s, 64)
    if err != nil {
        return 0
    }
    return f
}

func WaitVisibleWithTimeout(selector string, selType chromedp.QueryOption, timeout time.Duration) chromedp.ActionFunc {
    return func(ctx context.Context) error {
        ctx, cancel := context.WithTimeout(ctx, timeout)
        defer cancel()
        return chromedp.WaitVisible(selector, selType).Do(ctx)
    }
}