module github.com/tpyle/log-middleware/examples

go 1.27.0

require (
	github.com/rs/zerolog v1.35.1
	github.com/sirupsen/logrus v1.10.2
	github.com/tpyle/log-middleware/v3 v3.1.0
	github.com/tpyle/log-middleware/v3/logrusmw v1.0.0
	github.com/tpyle/log-middleware/v3/zerologmw v1.0.0
)

require (
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	golang.org/x/sys v0.48.0 // indirect
)

// The examples always build against the code in this repository.
replace (
	github.com/tpyle/log-middleware/v3 => ../
	github.com/tpyle/log-middleware/v3/logrusmw => ../v3/logrusmw
	github.com/tpyle/log-middleware/v3/zerologmw => ../v3/zerologmw
)
