module github.com/omzlo/nocand

go 1.16

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/omzlo/clog v0.0.0-20200929154205-ef979337c74c
	github.com/omzlo/go-sscp v0.0.0-20210205211644-9300fad1816f
)

replace (
	github.com/omzlo/clog => ../clog
	github.com/omzlo/go-sscp => ../go-sscp
)
