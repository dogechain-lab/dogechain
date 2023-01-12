package delwhite

import (
	"bytes"
	"fmt"

	"github.com/dogechain-lab/dogechain/command/helper"
)

type Result struct {
	Contracts  []string `json:"contracts"`
	NumDeleted int64    `json:"num_deleted"`
	Error      error    `json:"error"`
}

func (r *Result) GetOutput() string {
	var buffer bytes.Buffer

	buffer.WriteString("\n[WHITELIST CONTRACT DELETED]\n")
	buffer.WriteString(helper.FormatKV([]string{
		fmt.Sprintf("Contracts deleted|%d", r.NumDeleted),
	}))

	if len(r.Contracts) > 0 {
		buffer.WriteString("\n\n[LIST OF CONTRACTS]\n")
		buffer.WriteString(helper.FormatList(r.Contracts))
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
