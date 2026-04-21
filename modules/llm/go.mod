module hblabs.co/falcon/modules/llm

replace hblabs.co/falcon/common => ../../common

replace hblabs.co/falcon/modules/interfaces => ../../modules/interfaces

go 1.26.1

require (
	github.com/sirupsen/logrus v1.9.4
	hblabs.co/falcon/common v0.0.0-00010101000000-000000000000
)

require golang.org/x/sys v0.43.0 // indirect
