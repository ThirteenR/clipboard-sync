package main

import "time"

const (
	ServiceName       = "_clipboardsync._tcp"
	ServicePort       = 8920
	PollInterval      = 500 * time.Millisecond
	HeartbeatInterval = 30 * time.Second
	HeartbeatTimeout  = 10 * time.Second
	DedupTTL          = 5 * time.Minute
)
