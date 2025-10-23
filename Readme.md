# metric-gate
A Prometheus scrape proxy that can filter and aggregate metrics at the source, reducing cardinality before ingestion.
```mermaid
graph LR
prometheus --> metric-gate 
subgraph Pod
  metric-gate -. localhost .-> target
end
```
- Filtering is done using [metric_relabel_configs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config).
- Aggregation is done via `labeldrop` leading to `sum() without(label)` like result. Note, that it works for Counters and Histograms, but has no sense for Gauges.

Could be used in three modes:
- [sidecar](), as a container in the same pod with a single target (as above)
- [dns](), returns aggregated result of multiple targets
- [subset](), separate endpoints for customized subsets of metrics

### Why?
Consider the following example:

`ingress-nginx` exposes [6 Histograms](https://kubernetes.github.io/ingress-nginx/user-guide/monitoring/#request-metrics) (of 12 buckets each) for each Ingress object. Now suppose you have k8s cluster with 1k Ingresses, each having 10 Paths defined:

Cardinality: 1000 ingresses * 6 histogram * 12 buckets * 10 path = 720k metrics

The resulting size of http response on `/metrics` endpoint is 276Mb. Which is being pulled by Prometheus every scrape_interval (default 15s) leading to constant ~40Mbit/s traffic (compressed) on each replica of ingress-nginx Pod.

Sure, metrics could be filtered at Prometheus side in `metric_relabel_configs`, but it will not reduce the amount of data being pulled from target. And then aggregation could be done via `recording rules`, but one cannot drop already ingested data.

### sidecar mode
In this case, `metric-gate` could be used as a sidecar container, which would get original metrics via fast `localhost` connection, apply filtering and aggregation, and then return smaller response to Prometheus.

Let's reduce cardinality 10x by removing `path` label from `ingress-nginx` metrics above:

```ini
# before
metric{ingress="test", path="/api", ...} 5
metric{ingress="test", path="/ui", ...} 2

# after
metric{ingress="test", ...} 7
```
That could be done via dropping the label from all the metrics:
```yaml
- action: labeldrop
  regex: path
```
Or, you can target specific metrics by setting label to empty value:
```yaml
- action: replace
  source_labels: [ingress, __name__]
  regex: test;metric
  target_label: path
  replacement: "" # drops the label
```

Usual filtering is also works.  
Example of dropping all histograms except when `status="2xx"`:
```yaml
- action: drop
  source_labels: [status, __name__]
  regex: "[^2]xx;nginx_ingress_controller_.*_bucket"
```

### dns mode
When you prefix `--upstream` scheme with `dns+` (as in [thanos](https://thanos.io/tip/components/query.md/)) and set it to dns name which resolves to multiple IPs, `metric-gate` will return aggregated metrics from all the targets.
```mermaid
graph LR
prometheus --> metric-gate
metric-gate --> dns
metric-gate -.-> deployment-pod-a & deployment-pod-b
subgraph Deployment
  deployment-pod-a
  deployment-pod-b
end
```
In k8s you can use [headless Service](https://kubernetes.io/docs/concepts/services-networking/service/#headless-services) to expose all the Pods IPs for some LabelSelector:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: ingress-nginx-controller-metrics
spec:
  clusterIP: None # headless Service
  selector:
    app.kubernetes.io/name: ingress-nginx
  publishNotReadyAddresses: true # try to collect metrics from non-Ready Pods too
```

When request comes to `/metrics` endpoint of `metric-gate`, it (re)resolves `--upstream` dns to set of IPs and fan out to all of them at the same time. Timeout of those subrequests is configurable via `--scrape-timeout` flag (default 12s).

Continuing with our example above, this way you can reduce cardinality to the number of `ingress-nginx-controller` replicas.
To do that, disable direct scrape of each replica Pod by Prometheus, and scrape only `metric-gate` instead.  
Problems with this approach:
- In this mode `/source` endpoint returns single random upstream IP output.
- There is "automatic availability monitoring" based on [up](https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series) metric in Prometheus, which can detect when specific Target is down. In this case it only provides status of `metric-gate` itself, not the each `ingress-nginx-controller` replica.
- All the metrics are aggregated from all the replicas, so information like `process_start_time_seconds` which only makes sense for single replica is not available anymore.

Some of these issues could be solved by `subset` mode, read below.

### subset mode
This allows splitting single `origin` scrape output into multiple endpoints, each with a different set of metrics.

Take again our Ingress example above. [Docs](https://kubernetes.github.io/ingress-nginx/user-guide/monitoring/#exposed-metrics) assign metrics to the following groups:
- `Request metrics`, this is the main number of series on each replica (~720k). We want to aggregate them across all the replicas.
- `Nginx process metrics` and `Controller metrics`, 68 metrics in total. These only have sense for each specific replica.

To aggregate the first and directly scrape the second, we can stack `sidecar` and `dns` modes together:
```mermaid
graph LR
    prometheus --> metric-gate
    metric-gate --> dns
    prometheus --> ma & mb
    metric-gate -.-> ma & mb
    subgraph Deployment
        pod-a
        pod-b
    end
    subgraph pod-b
        mb["metric-gate-b"] -.localhost.-> tb["target-b"]
    end
    subgraph pod-a
        ma["metric-gate-a"] -.localhost.-> ta["target-a"]
    end
```

Let's take a look at the `metric-gate-a` configuration in this case:
```yaml
  containers:
    - name: metric-gate
      image: sepa/metric-gate
      args:
        - --port=8079
        - |
          --relabel=
            - action: keep # drop Request metrics
              source_labels: [__name__]
              regex: (go|nginx_ingress_controller_nginx_process|process)_.*
        - |
          --subsets=
            requests:
            - action: drop # keep only Request metrics
              source_labels: [__name__]
              regex: (go|nginx_ingress_controller_nginx_process|process)_.*
            - action: labeldrop
              regex: path # aggregate path
```
This way Prometheus scrapes usual `/metrics` endpoint, which is configured by `--relabel`, and has only per-process metrics.
While answering to this request, `metric-gate` uses the same upstream response to fill out all the configured `--subsets`.
In this case only one `subset` named "requests" is configured, which results then could be accessed at `/metrics/requests` endpoint.
Now downstream aggregating `metric-gate` could use it to get only `Request metrics` from each replica.

Note that data on `/metrics/requests` is available immediately, and accessing it does not generate new subrequest to the upstream. That is done to reduce both cpu/network load to upstream. But it also means, the data could be stale (it only refreshes on `/metrics` scrapes). To prevent time skew on graphs, `timestamp` of upstream request is added to all the metrics returned by `subset` endpoints (if they don't have it yet)

The diagram above is just one of the examples. We can drop `metric-gate` sidecars, and scrape metrics from Targets directly by Prometheus and aggregating `metric-gate` (each filtering own subset of metrics in `metric_relabel_configs`). That would lead to two scrapes  per-scrape-interval, and twice as much cpu/network load on each replica just from a metrics collection. Sidecars are shown here to demonstrate that we can aggregate pre-filtered results, while having single scrape for Targets.

### Usage
Available as a [docker image](https://hub.docker.com/r/sepa/metric-gate):
```
$ docker run sepa/metric-gate -h
Usage of ./metric-gate:
  -f, --file string           Analyze file for metrics and label cardinality and exit
  -p, --port int              Port to serve aggregated metrics on (default 8080)
      --relabel string        metric_relabel_configs contents
      --relabel-file string   metric_relabel_configs file path
      --subsets
  -H, --upstream string       Source URL to get metrics from (default "http://localhost:10254/metrics")
      --scrape-timeout
  -v, --version               Show version and exit
```
Run it near your target, and set `--upstream` to correct port.  

[metric_relabel_configs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config) could be provided via 2 methods:
- via configMap and `--relabel-file` flag with full path to the file
- via `--relabel` flag with yaml contents like so:
    ```yaml
    # ... k8s pod spec
    containers:
    - name: metric-gate
      image: sepa/metric-gate
      args:
        - --upstream=localhost:8081/metrics
        - |
          --relabel=
            - action: labeldrop
              regex: path
    # ...
    ```
The same applies to `--subsets` flag, but it should be a `map[string][]relabel_config` with `subset` name as a key. This key name then used to access filtered metrics via `/metrics/<name>` endpoint.

Available endpoints:
![](https://habrastorage.org/webt/5r/ev/v0/5revv0vjiqxwbqmn9yskosnpktm.png)

To find metrics to aggregate, you can use `/analyze` endpoint:
```
109632 nginx_ingress_controller_response_duration_seconds_bucket
  1313 ingress
  656 service
  334 namespace
  15 path
  12 le
  7 method
  5 status [4xx, 2xx, 3xx, 5xx, 1xx]

...
```
It shows the number of series for each metric, and then the number of values for each label. 

You can also run it as a cli-tool to analyze a file with metrics:
```
docker run --rm -v /path/to/metrics.txt:/metrics.txt sepa/metric-gate --file=/metrics.txt
```

### Alternatives
- [vmagent](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/) can do aggregation to new metric names and then send remote-write to Prometheus.  
How to relabel metrics to the original form?
Possibly use `vmserver` instead of `vmagent`, to scrape `/federate` endpoint instead of remote-write, to allow for relabeling in scrape config on prometheus side
- [vector.dev](https://vector.dev/docs/reference/configuration/transforms/aggregate/) cannot aggregate metrics like `sum() without(label)`, only filtering
- [otelcol](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/4968#issuecomment-2148753123) possible, but needs relabeling on the prometheus side to have metrics with original names
- [grafana alloy](https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.transform/) same as otelcol
- [exporter_aggregator](https://github.com/tynany/exporter_aggregator) scape metrics from a list of Prometheus exporter endpoints and aggregate the values of any metrics with the same name and label/s. Same as `dns` mode, but with static list of upstreams.
