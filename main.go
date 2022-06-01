package main

import (
	_ "embed"

	"github.com/dogechain-lab/dogechain/command/root"
	"github.com/dogechain-lab/dogechain/licenses"
)

var (
	//go:embed LICENSE
	license string
)

func main() {
	licenses.SetLicense(license)

	root.NewRootCommand().Execute()
}
