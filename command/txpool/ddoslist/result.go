package ddoslist

import (
	"bytes"
	"fmt"

	"github.com/dogechain-lab/dogechain/command/helper"
)

type Result struct {
	Blacklist map[string]int64 `json:"blacklist"`
	Whitelist map[string]int64 `json:"whitelist"`
	Error     error            `json:"error"`
}

func (r *Result) GetOutput() string {
	var buffer bytes.Buffer

	if len(r.Blacklist) > 0 {
		buffer.WriteString("\n[CONTRACT BLACKLIST]\n")
		for addr, count := range r.Blacklist {
			buffer.WriteString(helper.FormatKV([]string{
				fmt.Sprintf("%s|%d", addr, count),
			}))
		}
	}

	if len(r.Whitelist) > 0 {
		buffer.WriteString("\n\n[CONTRACT WHITELIST]\n")
		for addr, count := range r.Whitelist {
			buffer.WriteString(helper.FormatKV([]string{
				fmt.Sprintf("%s|%d", addr, count),
			}))
		}
	}

	if r.Error != nil {
		buffer.WriteString("\n\n[ERROR]\n")
		buffer.WriteString(helper.FormatKV([]string{
			fmt.Sprintf("Error|%s", r.Error.Error()),
		}))
	}

	buffer.WriteString("\n")

	return buffer.String()
}
