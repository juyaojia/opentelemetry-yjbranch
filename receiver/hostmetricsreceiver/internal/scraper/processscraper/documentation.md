[comment]: <> (Code generated by mdatagen. DO NOT EDIT.)

# hostmetricsreceiver/process

## Metrics

These are the metrics available for this scraper.

| Name | Description | Unit | Type | Attributes |
| ---- | ----------- | ---- | ---- | ---------- |
| **process.cpu.time** | Total CPU seconds broken down by different states. | s | Sum(Double) | <ul> <li>state</li> </ul> |
| **process.disk.io** | Disk bytes transferred. | By | Sum(Int) | <ul> <li>direction</li> </ul> |
| **process.memory.physical_usage** | The amount of physical memory in use. | By | Sum(Int) | <ul> </ul> |
| **process.memory.virtual_usage** | Virtual memory size. | By | Sum(Int) | <ul> </ul> |

**Highlighted metrics** are emitted by default. Other metrics are optional and not emitted by default.
Any metric can be enabled or disabled with the following scraper configuration:

```yaml
metrics:
  <metric_name>:
    enabled: <true|false>
```

## Attributes

| Name | Description |
| ---- | ----------- |
| direction | Direction of flow of bytes (read or write). |
| state | Breakdown of CPU usage by type. |
