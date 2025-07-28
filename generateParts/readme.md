# Hash-Based Table Partitioning Tool

This tool partitions database trace files based on table names using a configurable JSON file.

## Overview

The `hash.go` tool reads a trace file containing database operations and distributes them across multiple partition files based on table assignments defined in a JSON configuration file.

## Usage

```bash
go run hash.go -t <trace_file> -c <config_file>
```

### Command Line Options

- `-t` : Trace file location (default: `./serverInput.txt`)
- `-c` : Table configuration file (default: `./table_config.json`)

### Example

```bash
go run hash.go -t serverInput.txt -c table_config.json
```

## Configuration File Format

Create a JSON configuration file (`table_config.json`) to define table-to-partition mappings:

```json
{
  "tables": [
    {
      "name": "rankings",
      "partition_id": 0
    },
    {
      "name": "uservisits", 
      "partition_id": 1
    },
    {
      "name": "products",
      "partition_id": 2
    }
  ],
  "total_partitions": 3,
  "default_partitioning": "hash"
}
```

### Configuration Fields

- **`tables`**: Array of explicit table-to-partition mappings
  - `name`: Table name as it appears in trace file keys
  - `partition_id`: Target partition number (0-indexed)
- **`total_partitions`**: Total number of output partition files to create
- **`default_partitioning`**: Strategy for unmapped tables (`"hash"` or fallback to partition 0)

## Input Format

The tool expects trace files with lines in the format:
```
SET table_name/key_suffix value
```

Example:
```
SET rankings/page1 data1
SET uservisits/user123 data2
SET products/item456 data3
```

## Output

The tool creates partition files in the `Output/` directory:
- `Output/serverInput_0.txt`
- `Output/serverInput_1.txt`
- `Output/serverInput_N.txt`

Each file contains the SET operations for tables assigned to that partition.

## How It Works

1. **Explicit Mapping**: Tables listed in the config are assigned to their specified partition
2. **Hash-based Fallback**: Unknown tables are hashed and distributed across partitions
3. **Table Extraction**: Table names are extracted from keys using the first part before '/'

## Example Workflow

1. Create your configuration file:
```json
{
  "tables": [
    {"name": "users", "partition_id": 0},
    {"name": "orders", "partition_id": 1}
  ],
  "total_partitions": 2,
  "default_partitioning": "hash"
}
```

2. Run the partitioning tool:
```bash
go run hash.go -t myTrace.txt -c table_config.json
```

3. Check the `Output/` directory for partition files:
- `Output/serverInput_0.txt` (contains `users` table data)
- `Output/serverInput_1.txt` (contains `orders` table data)

The tool will display a summary showing which tables were assigned to which partitions.