package reporter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/RedTeamPentesting/monsoon/response"
)

const (
	_ int = iota + 30 // black
	red
	green
	yellow
	blue
	_ // magenta
	cyan
	_ // white
)

func colorStatusCode(statusCode int, format string) string {
	var color int

	switch statusCode / 100 {
	case 1:
		color = blue
	case 2:
		color = green
	case 3:
		color = cyan
	case 4:
		color = yellow
	case 5:
		color = red
	}

	if format == "" {
		format = "%d"
	}

	return fmt.Sprintf("\033[%dm"+format+"\033[0m", color, statusCode)
}

func colored(color int, s string) string {
	return fmt.Sprintf("\033[%dm%s\033[0m", color, s)
}

func Bold(s string) string {
	return "\033[1m" + s + "\033[0m"
}

func Dim(s string) string {
	return "\033[2m" + s + "\033[0m"
}

func FormatResponse(r response.Response) string {
	if r.Error != nil {
		// don't print anything if the request has been cancelled
		if errors.Is(r.Error, context.Canceled) {
			return ""
		}

		return fmt.Sprintf("    %s %8d %8d   %s %s", Bold(colored(red, "Err")), 0, 0,
			Bold(fmt.Sprintf("%-8v", r.Item)), colored(red, cleanedErrorString(r.Error)))
	}

	res := r.HTTPResponse
	status := fmt.Sprintf("%s %8d %8d   %s", colorStatusCode(res.StatusCode, "%7d"),
		r.Header.Bytes, r.Body.Bytes, Bold(fmt.Sprintf("%-8v", r.Item)))
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		loc, ok := res.Header["Location"]
		if ok {
			status += ", " + Dim("Location: ") + loc[0]
		}
	}
	if len(r.Extract) > 0 {
		status += Dim(" data: ") + Bold(strings.Join(quote(r.Extract), ", "))
	}
	return status
}

func quote(strs []string) []string {
	res := make([]string, 0, len(strs))
	for _, s := range strs {
		r := strconv.Quote(strings.TrimSpace(s))
		r = r[1 : len(r)-1]
		res = append(res, r)
	}
	return res
}

func cleanedErrorString(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}

	if errors.Is(err, io.EOF) {
		return "connection closed (EOF)"
	}

	errStr := err.Error()

	for {
		cleanedErrStr := errStr

		for _, prefix := range []string{"net/http", "net/url"} {
			cleanedErrStr = strings.TrimPrefix(cleanedErrStr, prefix+": ")
		}

		if cleanedErrStr == errStr {
			break
		} else {
			errStr = cleanedErrStr
		}
	}

	return errStr
}
