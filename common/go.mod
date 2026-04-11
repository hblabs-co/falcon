module hblabs.co/falcon/common

replace hblabs.co/falcon/modules/interfaces => ../modules/interfaces

go 1.26.1

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/joho/godotenv v1.5.1
	github.com/matoous/go-nanoid/v2 v2.1.0
	github.com/nats-io/nats.go v1.50.0
	github.com/sirupsen/logrus v1.9.4
	go.mongodb.org/mongo-driver/v2 v2.5.0
	hblabs.co/falcon/modules/interfaces v0.0.0-00010101000000-000000000000
)

require (
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/nats-io/nkeys v0.4.15 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.35.0 // indirect
)
