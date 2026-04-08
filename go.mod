module github.com/whereMyCleanCode/smpp-smsc

go 1.25.1

replace github.com/whereMyCleanCode/go-smpp/v2 => ../go-smpp

require (
	github.com/bwmarrin/snowflake v0.3.0
	github.com/mattn/go-isatty v0.0.20
	github.com/maypok86/otter/v2 v2.3.0
	github.com/whereMyCleanCode/go-smpp/v2 v2.0.0-00010101000000-000000000000
	go.uber.org/zap v1.27.1
	golang.org/x/text v0.35.0
	golang.org/x/time v0.15.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
