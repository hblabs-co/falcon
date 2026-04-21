module hblabs.co/falcon/modules/qdrant

replace hblabs.co/falcon/common => ../../common

replace hblabs.co/falcon/modules/interfaces => ../../modules/interfaces

go 1.26.1

require hblabs.co/falcon/common v0.0.0-00010101000000-000000000000

require (
	github.com/matoous/go-nanoid/v2 v2.1.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	golang.org/x/sys v0.43.0 // indirect
	hblabs.co/falcon/modules/interfaces v0.0.0-00010101000000-000000000000 // indirect
)
