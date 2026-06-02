package main

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestNoOSExitInMain(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, noosexitinmainAnalyzer,
		"github.com/zhebrikov/shortener/test/exit_outside_main",
		"github.com/zhebrikov/shortener/test/logfatal_outside_main",
		"github.com/zhebrikov/shortener/test/import_alias_outside_main",
		"github.com/zhebrikov/shortener/test/panic_outside_main",
		"github.com/zhebrikov/shortener/test/ok",
	)
}
