module github.com/nammn/go-licenses/testdata/modules/skipurl07

go 1.23.0

require example.invalid/licensed v0.0.0

replace example.invalid/licensed => ./licensed
