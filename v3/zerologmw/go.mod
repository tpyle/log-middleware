module github.com/tpyle/log-middleware/v3/zerologmw

go 1.27.0

require (
	github.com/rs/zerolog v1.35.1
	github.com/tpyle/log-middleware/v3 v3.1.0
)

require (
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	golang.org/x/sys v0.48.0 // indirect
)

// Builds in this repository use the local core module. Consumers ignore this
// directive and use the required version above.
replace github.com/tpyle/log-middleware/v3 => ../..
