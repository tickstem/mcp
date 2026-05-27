package main

import "time"

// DeletedResult is returned by tools that permanently remove a resource.
type DeletedResult struct {
	Message string `json:"message"`
}

// PingResult is returned by ping_heartbeat.
type PingResult struct {
	Message string `json:"message"`
}

// Assertion defines a condition on a monitor check response.
type Assertion struct {
	Source     string `json:"source"`
	Comparison string `json:"comparison"`
	Target     string `json:"target"`
}

// Monitor is returned by uptime monitor tools.
type Monitor struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	URL          string      `json:"url"`
	IntervalSecs int         `json:"interval_secs"`
	TimeoutSecs  int         `json:"timeout_secs"`
	Status       string      `json:"status"`
	NextCheckAt  *time.Time  `json:"next_check_at,omitempty"`
	SSLExpiresAt *time.Time  `json:"ssl_expires_at,omitempty"`
	Assertions   []Assertion `json:"assertions"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// MonitorList wraps a list of monitors.
type MonitorList struct {
	Monitors []Monitor `json:"monitors"`
}

// MonitorCheck is a single uptime check result.
type MonitorCheck struct {
	ID           string     `json:"id"`
	MonitorID    string     `json:"monitor_id"`
	Status       string     `json:"status"`
	StatusCode   *int       `json:"status_code,omitempty"`
	DurationMs   int64      `json:"duration_ms"`
	Error        string     `json:"error,omitempty"`
	SSLExpiresAt *time.Time `json:"ssl_expires_at,omitempty"`
	CheckedAt    time.Time  `json:"checked_at"`
}

// MonitorCheckList wraps a list of monitor checks.
type MonitorCheckList struct {
	Checks []MonitorCheck `json:"checks"`
}

// Heartbeat is returned by heartbeat monitor tools.
type Heartbeat struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Token          string     `json:"token"`
	IntervalSecs   int        `json:"interval_secs"`
	GraceSecs      int        `json:"grace_secs"`
	Status         string     `json:"status"`
	LastPingedAt   *time.Time `json:"last_pinged_at"`
	NextExpectedAt *time.Time `json:"next_expected_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// HeartbeatList wraps a list of heartbeats.
type HeartbeatList struct {
	Heartbeats []Heartbeat `json:"heartbeats"`
}

// HeartbeatPing is a single ping record.
type HeartbeatPing struct {
	ID          string    `json:"id"`
	HeartbeatID string    `json:"heartbeat_id"`
	PingedAt    time.Time `json:"pinged_at"`
}

// HeartbeatPingList wraps a list of heartbeat pings.
type HeartbeatPingList struct {
	Pings []HeartbeatPing `json:"pings"`
}
