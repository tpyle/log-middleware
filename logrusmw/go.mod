module github.com/tpyle/log-middleware/logrusmw/v3

go 1.27.0

require (
	github.com/sirupsen/logrus v1.10.2
	github.com/tpyle/log-middleware/v3 v3.1.0
)

require golang.org/x/sys v0.48.0 // indirect

// Builds in this repository use the local core module. Consumers ignore this
// directive and use the required version above.
replace github.com/tpyle/log-middleware/v3 => ../
