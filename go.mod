module github.com/philiaspace/shikenphi

go 1.22.0

require (
	github.com/philiaspace/phi-core v0.0.0
	github.com/philiaspace/phi-exam-domain v0.0.0
	github.com/philiaspace/phi-gamification v0.0.0
	github.com/philiaspace/phi-middleware v0.0.0
	github.com/philiaspace/phi-utils v0.0.0
	go.mongodb.org/mongo-driver v1.14.0
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

replace (
	github.com/philiaspace/phi-core => ../phi-core
	github.com/philiaspace/phi-exam-domain => ../phi-exam-domain
	github.com/philiaspace/phi-gamification => ../phi-gamification
	github.com/philiaspace/phi-middleware => ../phi-middleware
	github.com/philiaspace/phi-utils => ../phi-utils
)
