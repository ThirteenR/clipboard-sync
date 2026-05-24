module clipboard-sync

go 1.25.0

replace golang.design/x/clipboard => ./third_party/golang.design/x/clipboard

require golang.design/x/clipboard v0.7.1

require (
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grandcat/zeroconf v1.0.0 // indirect
	github.com/miekg/dns v1.1.27 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
)
