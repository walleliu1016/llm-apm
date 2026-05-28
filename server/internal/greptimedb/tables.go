package greptimedb

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CreateTablesSQL returns SQL statements to create core tables.
func CreateTablesSQL() []string {
	return []string{
		// Hook events table
		`CREATE TABLE IF NOT EXISTS apm_hook_events (
			ts TIMESTAMP TIME INDEX,
			session_id STRING,
			event_type STRING,
			agent_source STRING,
			agent_id STRING,
			parent_agent_id STRING,
			agent_depth BIGINT DEFAULT 0,
			turn_id STRING DEFAULT '',
			tool_name STRING,
			tool_input STRING,
			tool_result STRING,
			tool_use_id STRING,
			cwd STRING,
			error_flag BOOLEAN DEFAULT false,
			tenant_id STRING DEFAULT '',
			extra JSON
		) ENGINE=mito WITH (
			'append_mode' = 'true'
		)`,

		// Messages table
		`CREATE TABLE IF NOT EXISTS apm_messages (
			ts TIMESTAMP TIME INDEX,
			session_id STRING,
			message_type STRING,
			msg_role STRING,
			content STRING,
			model STRING,
			tool_name STRING,
			tool_use_id STRING,
			input_tokens BIGINT DEFAULT 0,
			output_tokens BIGINT DEFAULT 0,
			cache_read_tokens BIGINT DEFAULT 0,
			cache_creation_tokens BIGINT DEFAULT 0,
			tenant_id STRING DEFAULT ''
		) ENGINE=mito WITH (
			'append_mode' = 'true'
		)`,

		// Turns table
		`CREATE TABLE IF NOT EXISTS apm_turns (
			ts TIMESTAMP TIME INDEX,
			turn_id STRING,
			session_id STRING,
			start_ts TIMESTAMP,
			end_ts TIMESTAMP,
			user_prompt STRING,
			agent_response STRING,
			input_tokens BIGINT DEFAULT 0,
			output_tokens BIGINT DEFAULT 0,
			cost_usd DOUBLE DEFAULT 0,
			tool_count BIGINT DEFAULT 0,
			has_error BOOLEAN DEFAULT false,
			tenant_id STRING DEFAULT ''
		) ENGINE=mito`,

		// Anomalies table
		`CREATE TABLE IF NOT EXISTS apm_anomalies (
			ts TIMESTAMP TIME INDEX,
			session_id STRING,
			anomaly_type STRING,
			severity STRING,
			description STRING,
			suggested_cause STRING,
			related_event_id STRING,
			tenant_id STRING DEFAULT '',
			extra JSON
		) ENGINE=mito WITH (
			'append_mode' = 'true'
		)`,
	}
}

// InitTables creates all required tables in GreptimeDB (localhost).
func InitTables(httpPort int) error {
	return InitTablesAt("127.0.0.1", httpPort)
}

// InitTablesAt creates all required tables in GreptimeDB at specified host.
func InitTablesAt(host string, httpPort int) error {
	sqlURL := fmt.Sprintf("http://%s:%d/v1/sql", host, httpPort)

	for _, sql := range CreateTablesSQL() {
		if err := execSQL(sqlURL, sql); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}
	return nil
}

func execSQL(urlStr, sql string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	form := url.Values{}
	form.Set("sql", sql)

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr,
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("non-200 status: %d, body: %s", resp.StatusCode, body)
	}
	return nil
}

// escapeSQL escapes single quotes for SQL strings.
func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}