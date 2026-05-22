package greptimedb

// DefaultConfig returns minimal GreptimeDB standalone config.
// Disables unused protocols, keeps HTTP and MySQL.
func DefaultConfig() string {
	return `
[http]
addr = "127.0.0.1:14000"

[grpc]
addr = "127.0.0.1:14001"

[mysql]
addr = "127.0.0.1:14002"

[storage]
type = "File"
data_dir = "{{DATA_DIR}}"

[logging]
level = "info"
`
}