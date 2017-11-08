package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/fd0/termstatus"
)

// Filter decides which response to print.
type Filter interface {
	Print(Response) bool
}

// Reporter prints the Responses to stdout.
type Reporter struct {
	term *termstatus.Terminal
	f    Filter
}

// NewReporter returns a new reporter.
func NewReporter(ctx context.Context, term *termstatus.Terminal, f Filter) *Reporter {
	return &Reporter{term: term, f: f}
}

// HTTPStats collects statistics about several HTTP responses.
type HTTPStats struct {
	Start       time.Time
	StatusCodes map[int]int
	Errors      int
	Responses   int
	Count       int

	lastRPS time.Time
	rps     float64
}

func formatSeconds(secs float64) string {
	sec := int(secs)
	hours := sec / 3600
	sec -= hours * 3600
	min := sec / 60
	sec -= min * 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", hours, min, sec)
	}

	return fmt.Sprintf("%dm%02ds", min, sec)
}

// Report returns a report about the received HTTP status codes.
func (h *HTTPStats) Report() (res []string) {
	status := fmt.Sprintf("%v requests", h.Responses)
	dur := time.Since(h.Start) / time.Second
	if dur > 0 && time.Since(h.lastRPS) > time.Second {
		h.rps = float64(h.Responses) / float64(dur)
		h.lastRPS = time.Now()
	}
	if h.rps > 0 {
		status += fmt.Sprintf(", %.0f req/s", h.rps)
	}

	if h.Count > 0 {
		todo := h.Count - h.Responses
		status += fmt.Sprintf(", %d todo", todo)

		if h.rps > 0 {
			rem := float64(todo) / h.rps
			status += fmt.Sprintf(", %s remaining", formatSeconds(rem))
		}
	}

	res = append(res, status)

	for code, count := range h.StatusCodes {
		res = append(res, fmt.Sprintf("%v: %v", code, count))
	}
	sort.Sort(sort.StringSlice(res[1:]))

	return res
}

// Display shows incoming Responses.
func (r *Reporter) Display(ctx context.Context, wg *sync.WaitGroup, ch <-chan Response, countChannel <-chan int) {
	defer wg.Done()

	r.term.Printf("%7s %7s %7s %7s %7s\n", "bytes", "words", "lines", "header", "status")

	stats := &HTTPStats{
		Start:       time.Now(),
		StatusCodes: make(map[int]int),
	}

	for response := range ch {
		select {
		case c := <-countChannel:
			stats.Count = c
		default:
		}

		stats.Responses++
		if response.Error != nil {
			stats.Errors++
		} else {
			stats.StatusCodes[response.HTTPResponse.StatusCode]++
		}

		if r.f.Print(response) {
			r.term.Printf("%v\n", response)
		}

		r.term.SetStatus(stats.Report())
	}

	r.term.Print("\n")
	r.term.Printf("process %d HTTP requests in %v\n", stats.Responses, formatSeconds(time.Since(stats.Start).Seconds()))
	for _, line := range stats.Report()[1:] {
		r.term.Print(line)
	}
}
