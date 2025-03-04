# Transform Processor

| Status                   |                                                                                    |
|--------------------------|------------------------------------------------------------------------------------|
| Stability                | [alpha]                                                                            |
| Supported pipeline types | traces, metrics, logs                                                              |
| Distributions            | [contrib]                                                                          |
| Warnings                 | [Unsound Transformations, Identity Conflict, Orphaned Telemetry, Other](#warnings) |

The transform processor modifies telemetry based on configuration using the [OpenTelemetry Transformation Language](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl).

For each signal type, the processor takes a list of statements associated to a [Context type](#contexts) and executes the statements against the incoming telemetry in the order specified in the config.
Each statement can access and transform telemetry using functions and allow the use of a condition to help decide whether the function should be executed.

## Config

The transform processor allows configuring multiple context statements for traces, metrics, and logs.
The value of `context` specifies which [OTTL Context](#contexts) to use when interpreting the associated statements.
The statement strings, which must be OTTL compatible, will be passed to the OTTL and interpreted using the associated context. 
Each context will be processed in the order specified and each statement for a context will be executed in the order specified.

```yaml
transform:
  <trace|metric|log>_statements:
    - context: string
      statements:
        - string
        - string
        - string
    - context: string
      statements:
        - string
        - string
        - string
```

Proper use of contexts will provide increased performance and capabilities.  See [Contexts](#contexts) for more details.

Valid values for `context` are:

| Signal            | Context Values                                 |
|-------------------|------------------------------------------------|
| trace_statements  | `resource`, `scope`, `span`, and `spanevent`   |
| metric_statements | `resource`, `scope`, `metric`, and `datapoint` |
| log_statements    | `resource`, `scope`, and `log`                 |

## Example

The example takes advantage of context efficiency by grouping transformations with the context which it intends to transform.
See [Contexts](#contexts) for more details.

Example configuration:
```yaml
transform:
  trace_statements:
    - context: resource
      statements:
        - keep_keys(attributes, ["service.name", "service.namespace", "cloud.region", "process.command_line"])
        - replace_pattern(attributes["process.command_line"], "password\\=[^\\s]*(\\s?)", "password=***")
        - limit(attributes, 100, [])
        - truncate_all(attributes, 4096)
    - context: trace
      statements:
        - set(status.code, 1) where attributes["http.path"] == "/health"
        - set(name, attributes["http.route"])
        - replace_match(attributes["http.target"], "/user/*/list/*", "/user/{userId}/list/{listId}")
        - limit(attributes, 100, [])
        - truncate_all(attributes, 4096)

  metric_statements:
    - context: resource
      statements:
      - keep_keys(attributes, ["host.name"])
      - truncate_all(attributes, 4096)
    - context: metric
      statements:
        - set(description, "Sum") where type == "Sum"
    - context: datapoint
      statements:
        - limit(attributes, 100, ["host.name"])
        - truncate_all(attributes, 4096)
        - convert_sum_to_gauge() where metric.name == "system.processes.count"
        - convert_gauge_to_sum("cumulative", false) where metric.name == "prometheus_metric"
        
  log_statements:
    - context: resource
      statements:
        - keep_keys(resource.attributes, ["service.name", "service.namespace", "cloud.region"])
    - context: log
      statements:
        - set(severity_text, "FAIL") where body == "request failed"
        - replace_all_matches(attributes, "/user/*/list/*", "/user/{userId}/list/{listId}")
        - replace_all_patterns(attributes, "/account/\\d{4}", "/account/{accountId}")
        - set(body, attributes["http.route"])
```

## Grammar

You can learn more in-depth details on the capabilities and limitations of the OpenTelemetry Transformation Language used by the transform processor by reading about its [grammar](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl#grammar).

## Contexts

The transform processor utilizes the OTTL's contexts to transform Resource, Scope, Trace, SpanEvent, Metric, DataPoint, and Log telemetry.
The contexts allow the OTTL to interact with the underlying telemetry data in its pdata form.

- [Resource Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlresource)
- [Scope Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlscope)
- [Span Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlspan) <!-- markdown-link-check-disable-line -->
- [SpanEvent Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlspanevent)
- [Metric Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottlmetric)
- [DataPoint Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottldatapoint) <!-- markdown-link-check-disable-line -->
- [Log Context](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/contexts/ottllog) <!-- markdown-link-check-disable-line -->

Each context allows transformation of its type of telemetry.  
For example, statements associated to a `resource` context will be able to transform the resource's `attributes` and `dropped_attributes_count`.

Contexts __NEVER__ supply access to individual items "lower" in the protobuf definition.
- This means statements associated to a `resource` __WILL NOT__ be able to access the underlying instrumentation scopes.
- This means statements associated to a `scope` __WILL NOT__ be able to access the underlying telemetry slices (spans, metrics, or logs).
- Similarly, statements associated to a  `metric` __WILL NOT__ be able to access individual datapoints, but can access the entire datapoints slice.
- Similarly, statements associated to a  `trace` __WILL NOT__ be able to access individual SpanEvents, but can access the entire SpanEvents slice.

For practical purposes, this means that a context cannot make decisions on its telemetry based on telemetry "lower" in the structure.
For example, __the following context statement is not possible__ because it attempts to use individual datapoint attributes in the condition of a statements that is associated to a `metric`

```yaml
metric_statements:
- context: metric
  statements:
  - set(description, "test passed") where datapoints.attributes["test"] == "pass"
```

Context __ALWAYS__ supply access to the items "higher" in the protobuf definition that are associated to the telemetry being transformed.
- This means that statements associated to a `datapoint` have access to a datapoint's metric, instrumentation scope, and resource.
- This means that statements associated to a `spanevent` have access to a spanevent's span, instrumentation scope, and resource.
- This means that statements associated to a `trace`/`metric`/`log` have access to the telemetry's instrumentation scope, and resource.
- This means that statements associated to a `scope` have access to the scope's resource.

For example, __the following context statement is possible__ because `datapoint` statements can access the datapoint's metric.

```yaml
metric_statements:
- context: datapoint
  statements:
    - set(metric.description, "test passed") where attributes["test"] == "pass"
```

Whenever possible, associate your statements to the context that the statement intend to transform.
Although you can modify resource attributes associated to a span using the `trace` context, it is more efficient to use the `resource` context.
This is because contexts are nested: the efficiency comes because higher-level contexts can avoid iterating through any of the contexts at a lower level. 

## Supported functions:

Since the transform processor utilizes the OTTL's contexts for Traces, Metrics, and Logs, it is able to utilize functions that expect pdata in addition to any common functions. These common functions can be used for any signal.
<!-- markdown-link-check-disable-next-line -->
- [OTTL Functions](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/ottlfuncs)

In addition to OTTL functions, the processor defines its own functions to help with transformations specific to this processor:

**Metrics only functions**
- [convert_sum_to_gauge](#convert_sum_to_gauge)
- [convert_gauge_to_sum](#convert_gauge_to_sum)
- [convert_summary_count_val_to_sum](#convert_summary_count_val_to_sum)
- [convert_summary_sum_val_to_sum](#convert_summary_sum_val_to_sum)

## convert_sum_to_gauge

`convert_sum_to_gauge()`

Converts incoming metrics of type "Sum" to type "Gauge", retaining the metric's datapoints. Noop for metrics that are not of type "Sum".

**NOTE:** This function may cause a metric to break semantics for [Gauge metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#gauge). Use at your own risk.

Examples:

- `convert_sum_to_gauge()`

## convert_gauge_to_sum

`convert_gauge_to_sum(aggregation_temporality, is_monotonic)`

Converts incoming metrics of type "Gauge" to type "Sum", retaining the metric's datapoints and setting its aggregation temporality and monotonicity accordingly. Noop for metrics that are not of type "Gauge".

`aggregation_temporality` is a string (`"cumulative"` or `"delta"`) that specifies the resultant metric's aggregation temporality. `is_monotonic` is a boolean that specifies the resultant metric's monotonicity. 

**NOTE:** This function may cause a metric to break semantics for [Sum metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#sums). Use at your own risk.

Examples:

- `convert_gauge_to_sum("cumulative", false)`


- `convert_gauge_to_sum("delta", true)`

## convert_summary_count_val_to_sum

`convert_summary_count_val_to_sum(aggregation_temporality, is_monotonic)`

The `convert_summary_count_val_to_sum` function creates a new Sum metric from a Summary's count value.

`aggregation_temporality` is a string (`"cumulative"` or `"delta"`) representing the desired aggregation temporality of the new metric. `is_monotonic` is a boolean representing the monotonicity of the new metric.

The name for the new metric will be `<summary metric name>_count`. The fields that are copied are: `timestamp`, `starttimestamp`, `attibutes`, and `description`. The new metric that is created will be passed to all functions in the metrics statements list.  Function conditions will apply.

**NOTE:** This function may cause a metric to break semantics for [Sum metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#sums). Use at your own risk.

Examples:

- `convert_summary_count_val_to_sum("delta", true)`


- `convert_summary_count_val_to_sum("cumulative", false)`

## convert_summary_sum_val_to_sum

`convert_summary_sum_val_to_sum(aggregation_temporality, is_monotonic)`

The `convert_summary_sum_val_to_sum` function creates a new Sum metric from a Summary's sum value.

`aggregation_temporality` is a string (`"cumulative"` or `"delta"`) representing the desired aggregation temporality of the new metric. `is_monotonic` is a boolean representing the monotonicity of the new metric.

The name for the new metric will be `<summary metric name>_sum`. The fields that are copied are: `timestamp`, `starttimestamp`, `attibutes`, and `description`. The new metric that is created will be passed to all functions in the metrics statements list.  Function conditions will apply.

**NOTE:** This function may cause a metric to break semantics for [Sum metrics](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md#sums). Use at your own risk.

Examples:

- `convert_summary_sum_val_to_sum("delta", true)`


- `convert_summary_sum_val_to_sum("cumulative", false)`

## Contributing

See [CONTRIBUTING.md](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/transformprocessor/CONTRIBUTING.md).


## Warnings

The transform processor's implementation of the [OpenTelemetry Transformation Language]https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/processing.md#opentelemetry-transformation-language) (OTTL) allows users to modify all aspects of their telemetry.  Some specific risks are listed below, but this is not an exhaustive list.  In general, understand your data before using the transform processor.  

- [Unsound Transformations](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/standard-warnings.md#unsound-transformations): Several Metric-only functions allow you to transform one metric data type to another or create new metrics from an existing metrics.  Transformations between metric data types are not defined in the [metrics data model](https://github.com/open-telemetry/opentelemetry-specification/blob/main//specification/metrics/data-model.md).  These functions have the expectation that you understand the incoming data and know that it can be meaningfully converted to a new metric data type or can meaningfully be used to create new metrics.
  - Although the OTTL allows the `set` function to be used with `metric.data_type`, its implementation in the transform processor is NOOP.  To modify a data type you must use a function specific to that purpose.
- [Identity Conflict](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/standard-warnings.md#identity-conflict): Transformation of metrics have the potential to affect the identity of a metric leading to an Identity Crisis. Be especially cautious when transforming metric name and when reducing/changing existing attributes.  Adding new attributes is safe.
- [Orphaned Telemetry](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/standard-warnings.md#orphaned-telemetry): The processor allows you to modify `span_id`, `trace_id`, and `parent_span_id` for traces and `span_id`, and `trace_id` logs.  Modifying these fields could lead to orphaned spans or logs.

[alpha]: https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
