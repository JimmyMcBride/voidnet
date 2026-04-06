package main

import (
	"flag"
	"fmt"
	"os"

	"voidnet/internal/app"
	"voidnet/internal/buildinfo"
	"voidnet/internal/content"
	"voidnet/internal/meta"
	"voidnet/internal/ui"
)

func main() {
	var (
		seedFlag     = flag.Int64("seed", 0, "run seed override")
		metaPathFlag = flag.String("meta-path", "", "meta progression file path")
		versionFlag  = flag.Bool("version", false, "print build version and exit")
	)
	flag.Parse()

	if *versionFlag {
		fmt.Println(buildinfo.Current().String())
		return
	}

	registry, err := content.Load()
	if err != nil {
		exitErr(err)
	}

	metaPath := *metaPathFlag
	if metaPath == "" {
		metaPath, err = meta.DefaultPath()
		if err != nil {
			exitErr(err)
		}
	}

	store := meta.NewStore(metaPath)
	state, err := store.Load()
	if err != nil {
		exitErr(err)
	}

	seed := *seedFlag
	fixedSeed := seed != 0
	if !fixedSeed {
		seed = app.NewSeed()
	}

	session := app.NewSession(registry, store, state, seed, fixedSeed)
	if err := ui.Run(session); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
