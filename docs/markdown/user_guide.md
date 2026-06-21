# Plomvix User Guide: Connections, SDKs, & Observability Integrations

Plomvix is a high-performance database supporting relational queries, time-series metrics, and schema-less log storage. Because Plomvix implements the **PostgreSQL Wire Protocol v3.0**, standard PostgreSQL clients, application libraries, and observability agents can connect to and query it seamlessly.

---

## 1. Connecting via SQL Clients

Plomvix listens by default on `127.0.0.1:5432` (or as configured in `config.toml`). Standard PostgreSQL administration and query tools can interface with it directly.

### Command Line (`psql`)
Connect using the standard PostgreSQL command-line tool. Since the default configuration uses the `trust` authentication type, any username will be accepted without a password:

```bash
psql -h 127.0.0.1 -p 5432 -U postgres -d plomvix
```

### GUI Clients (DBeaver, TablePlus, pgAdmin)
To connect using a graphic database manager, configure a new **PostgreSQL** connection with the following fields:
* **Host**: `127.0.0.1` (or your server's IP address)
* **Port**: `5432`
* **Database**: `plomvix`
* **Username**: `postgres` (or any string)
* **Password**: *Leave blank*
* **SSL**: `Disable`

---

## 2. Ingestion Routing & Zero-DDL (Schema-on-Write)

Plomvix supports **Zero-DDL auto-table registration** for time-series metrics and log ingestion. When an `INSERT` statement targets a table that does not exist in the Global Catalog, Plomvix automatically registers and instantiates the table.

### Routing Disambiguation Rule
The Global Router intercepts the insertion and evaluates the table name to route it to the appropriate pluggable engine:
1. **Logs Engine:** If the table name contains the case-insensitive substring `"log"` or `"logs"` (e.g., `syslog`, `app_logs`, `nginx_log_tracker`), it is auto-created under the **Logs Engine**.
2. **Metrics Engine:** All other missing tables (e.g., `system_cpu`, `temperature_readings`, `http_requests`) are auto-created under the **Metrics Engine** by default.

---

## 3. Observability Agent Integrations (Metrics)

Stream system statistics, application metrics, or environment readings directly into Plomvix.

### Telegraf Ingestion (InfluxData)
Telegraf collects agent metrics and can stream them using standard relational outputs. Configure the `outputs.postgresql` plugin to target the Plomvix instance:

```toml
# /etc/telegraf/telegraf.conf

[[outputs.postgresql]]
  ## PostgreSQL connection string.
  connection = "host=127.0.0.1 port=5432 user=postgres dbname=plomvix sslmode=disable"
  
  ## Schema to write into.
  schema = "public"
  
  ## Tags to write as JSON or KV parameters.
  tags_as_json = true
  
  ## Disable transaction safety check fallback if unsupported.
  # (Recommended to keep disable to speed up writes)
  sslmode = "disable"
```

### Vector Ingestion (Datadog)
Vector can route metric streams to database backends. Configure Vector's `postgresql` sink to stream data points into a metrics table (e.g., `server_metrics`):

```yaml
# vector.yaml

sources:
  my_metric_source:
    type: host_metrics
    collectors:
      - cpu
      - memory

sinks:
  plomvix_metrics:
    type: postgresql
    inputs:
      - my_metric_source
    connection_string: "postgresql://postgres@127.0.0.1:5432/plomvix?sslmode=disable"
    table: "host_metrics" # Defaults to the Metrics Engine
```

---

## 4. Observability Agent Integrations (Logs)

Stream syslog data, application logs, container output, or server audits using logs collectors.

### Vector Ingestion (Logs)
To route logs via Vector, direct the sink to a table containing `"log"` or `"logs"` in its name to ensure it utilizes the optimized Logs Engine:

```yaml
# vector.yaml

sources:
  app_log_source:
    type: file
    include:
      - "/var/log/nginx/*.log"

sinks:
  plomvix_logs:
    type: postgresql
    inputs:
      - app_log_source
    connection_string: "postgresql://postgres@127.0.0.1:5432/plomvix?sslmode=disable"
    table: "nginx_app_logs" # Routes automatically to Logs Engine
```

### Fluent Bit Ingestion (Logs)
Fluent Bit can stream log payloads directly to Plomvix using the PostgreSQL output plugin. Specify a table name ending in `_logs`:

```toml
# fluent-bit.conf

[INPUT]
    Name   cpu
    Interval_Sec 1

[OUTPUT]
    Name     pgsql
    Match    *
    Host     127.0.0.1
    Port     5432
    User     postgres
    Database plomvix
    Table    fluentbit_cpu_logs  # Routed to Logs Engine
```

---

## 5. Programmatic Connection Code (Go SDK Example)

You can write application logic to connect, write, and scan data using PostgreSQL client libraries in Go. Below is a complete, compilation-ready example using the `pgx` driver.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	ctx := context.Background()

	// 1. Establish connection to Plomvix using standard connection string
	connStr := "postgresql://postgres@127.0.0.1:5432/plomvix?sslmode=disable"
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to Plomvix: %v", err)
	}
	defer conn.Close(ctx)

	fmt.Println("Successfully connected to Plomvix!")

	// 2. Metrics Engine Example (Auto-created table 'server_load')
	metricTime := time.Now().Unix()
	_, err = conn.Exec(ctx, 
		"INSERT INTO server_load (time, tags, metric_name, value) VALUES ($1, $2, $3, $4)",
		metricTime, "host=server-01,region=us-east", "cpu_utilization", 84.5,
	)
	if err != nil {
		log.Fatalf("Metrics insert failed: %v", err)
	}
	fmt.Println("Inserted metric point successfully.")

	// Query metrics back
	rows, err := conn.Query(ctx, 
		"SELECT time, tags, metric_name, value FROM server_load WHERE time >= $1", 
		metricTime-60,
	)
	if err != nil {
		log.Fatalf("Metrics query failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t int64
		var tags, name string
		var val float64
		if err := rows.Scan(&t, &tags, &name, &val); err != nil {
			log.Fatalf("Failed to scan metric row: %v", err)
		}
		fmt.Printf("[Metric] Time: %d, Tags: %s, Name: %s, Val: %.2f\n", t, tags, name, val)
	}

	// 3. Logs Engine Example (Auto-created table 'app_service_logs')
	logTime := time.Now().Unix()
	logPayload := `{"message":"database connection timed out", "level":"error", "service":"billing"}`
	_, err = conn.Exec(ctx, 
		"INSERT INTO app_service_logs (time, severity, body) VALUES ($1, $2, $3)",
		logTime, "ERROR", logPayload,
	)
	if err != nil {
		log.Fatalf("Logs insert failed: %v", err)
	}
	fmt.Println("Inserted log record successfully.")

	// Query logs using substring filter (LIKE operator)
	logRows, err := conn.Query(ctx, 
		"SELECT time, severity, attributes, body FROM app_service_logs WHERE body LIKE $1", 
		"%connection timed out%",
	)
	if err != nil {
		log.Fatalf("Logs query failed: %v", err)
	}
	defer logRows.Close()

	for logRows.Next() {
		var t int64
		var sev, attrs, body string
		if err := logRows.Scan(&t, &sev, &attrs, &body); err != nil {
			log.Fatalf("Failed to scan log row: %v", err)
		}
		fmt.Printf("[Log] Time: %d, Severity: %s, Attributes: %s, Body: %s\n", t, sev, attrs, body)
	}
}
```

---

## 6. Installation & Service Setup

### Automated Linux Service Installation
The `install.sh` script automatically detects your host CPU architecture (AMD64 or ARM64) and init system (Systemd or OpenRC), invokes Go to compile Plomvix statically from local source, configures the `plomvix` system user/directories, and registers it as a daemon.

To run the installation:

```bash
# Execute the installer from the workspace root (requires root privileges)
sudo ./scripts/install.sh
```

### Managed Services Configuration
Once installed, Plomvix operates as a service:
* **Systemd (Ubuntu/WSL):** Managed via `systemctl start/stop/status plomvix`.
* **OpenRC (Alpine):** Managed via `rc-service plomvix start/stop/status`.
* **WSL Fallback:** Controlled using `plomvix-service start/stop/status`.

