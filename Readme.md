### prom-scrape-proxy

Alternatives:
- add [vmagent](https://docs.victoriametrics.com/victoriametrics/stream-aggregation/) as a sidecar to each `ingress` pod and then on prometheus side receive remote-write. 
How to relabel metrics to original form? 
Possibly use `vmserver` instead of `vmagent`, to skip remote-write and scrape `/federate` endpoint, then do relabel in scrape config on prometheus side.
- [vector.dev](https://vector.dev/docs/reference/configuration/transforms/aggregate/) cannot aggregate metrics like `sum() without(label)`
- [otelcol](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/4968#issuecomment-2148753123) possible, but need to do relabel on prometheus side to have metrics with original names.
- [grafana alloy](https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.transform/) same as otelcol
- [exporter_aggregator](https://github.com/tynany/exporter_aggregator) scape metrics from a list of Prometheus exporter endpoints and aggregate the values of any metrics with the same name and label/s

