# Configurator
Simple k8s controller, that is intented to run in a container, that
will listen for specific CRD and inject configuration
into container services in a runtime.

CRD example:
```yaml
apiVersion: rtcfg.dvext.io/v1alpha1
kind: Generic
metadata:
  name: example-foo
  namespace: proxy-lg
selector:
  matchLabels:
    app: rt-cfg-ctrl-test
spec:
  service: collector
  config:
    parameters:
      alternative_configs.trace: "true"
```


```
Usage of configurator:
$ export KUBECONFIG=~/config.conf
$ configurator --config=./configs/config.yaml --namespace=my-namespace --podname=my-pod-name -v 2

Command line options:
  -kubeconfig string
         Path to a kubeconfig. If not specified, then env. var KUBECONFIG examined. Only required if out-of-cluster.
  -config string
        Path to controller config file (mandatory).
  -namespace string
        k8s namespace where this application runs (mandatory).
  -podname string
        k8s pod name where this application runs (mandatory).
 
 
  -add_dir_header
        If true, adds the file directory to the header of the log messages
  -alsologtostderr
        log to standard error as well as files
  -log_backtrace_at value
        when logging hits line file:N, emit a stack trace
  -log_dir string
        If non-empty, write log files in this directory
  -log_file string
        If non-empty, use this log file
  -log_file_max_size uint
        Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
  -logtostderr
        log to standard error instead of files (default true)
  -skip_headers
        If true, avoid header prefixes in the log messages
  -skip_log_headers
        If true, avoid headers when opening log files
  -stderrthreshold value
        logs at or above this threshold go to stderr (default 2)
  -v value
        number for the log level verbosity
  -vmodule value
        comma-separated list of pattern=N settings for file-filtered logging
```

Configuration file example:
```yaml
- pid_finder:
    supervised_service_name: "collector"
    service_pattern: '^s6 supervise.*$'
    parent_pattern: '^collector$'

  config:

    parameters:
      alternative_configs.trace.interpreter: "bash -c"
      alternative_configs.trace.enableCommand: "echo -n '/etc/telegraf/collector.template.trace' > /var/run/s6/container_environment/COLLECTOR_TEMPLATE"
      alternative_configs.trace.disableCommand: "rm -rf /var/run/s6/container_environment/COLLECTOR_TEMPLATE"
      alternative_configs.trace.reloadCommand: "s6-svc -d /services/s_collector/run && s6-svc -u /services/s_collector/run"
```
