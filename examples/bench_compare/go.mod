module github.com/lcylpzls/logx/examples/bench_compare

go 1.26.5

require (
	github.com/lcylpzls/logx v1.0.0
	github.com/lcylpzls/testx v1.2.3
	github.com/sirupsen/logrus v1.9.4
	go.uber.org/zap v1.28.0
)

require (
	github.com/lcylpzls/errx v1.4.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
)

replace github.com/lcylpzls/logx => ../../
