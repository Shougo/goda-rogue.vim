// Command goda-rogue is the binary of goda-rogue.vim: goda.vim builds it
// from this directory on its first start, or with goda_rogue#build().
package main

import (
	"github.com/Shougo/goda-rogue.vim/rogue"
	"github.com/Shougo/goda.vim/host"
)

func main() { host.Main(rogue.New()) }
