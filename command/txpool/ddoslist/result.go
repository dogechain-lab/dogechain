package ddoslist

import (
	"fmt"
	"strings"

	"github.com/dogechain-lab/dogechain/command/helper"
)

type Result struct {
	Blacklist map[string]int64 `json:"blacklist"`
	Whitelist map[string]int64 `json:"whitelist"`
	Error     error            `json:"error"`
}

func (r *Result) GetOutput() string {
	var builder strings.Builder

	if len(r.Blacklist) > 0 {
		builder.WriteString("\n[CONTRACT BLACKLIST - ADDR : ATTACK COUNT]\n")

		blackAddrCounts := make([]string, 0, len(r.Whitelist))
		for addr, count := range r.Blacklist {
			blackAddrCounts = append(blackAddrCounts, fmt.Sprintf("%s : %d", addr, count))
		}

		builder.WriteString(helper.FormatList(blackAddrCounts))
	}

	if len(r.Whitelist) > 0 {
		builder.WriteString("\n\n[CONTRACT WHITELIST]\n")

		whiteAddrs := make([]string, 0, len(r.Whitelist))
		for addr := range r.Whitelist {
			whiteAddrs = append(whiteAddrs, addr)
		}

		builder.WriteString(helper.FormatList(whiteAddrs))
	}

	if r.Error != nil {
		builder.WriteString("\n\n[ERROR]\n")
		builder.WriteString(helper.FormatKV([]string{
			fmt.Sprintf("Error|%s", r.Error.Error()),
		}))
	}

	builder.WriteString("\n")

	return builder.String()
}
