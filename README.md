![GitHub Logo](images/logo.png)
# Configurator
Simple k8s controller, that is intended to run in a container, that
will listen for specific CRD and inject the configuration into sibling services (in the same container) in a runtime.

### How to install into k8s cluster:
1. Deploy the CRD itself. CRD manifest is stored here - `k8s-manifests/crd.yaml`

2. Add to your service account (under which you will run the configurator service) necessary rights to work
with `generics.rtcfg.dvext.io`. Example of role & cluster role can be found in `k8s-manifests/rights.yaml`

3. Prepare CRD (for example):
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
Comment on CRD structure:
 * `metadata->namespace` filed of the CRD limit the scope. `configurator` process only CRDs that are in the same namespace
 * `selector->matchLabels` contains list of pod labels. If the list match with the target (where configurator running) POD labels,
 then this CRD is processed, otherwise skipped.
 * `spec->service` contains supervised service name, if the service name match the one configured in `configurator` config file,
 then CRD is processed, otherwise skipped.
 * `spec->config->parameters` - here you should set arbitrary list of keys with only valid values "true" or "false".
 The keys themselves should match to `configurator` parameters (section `config->parameters`) as prefix.
 I.e. parameter `alternative_configs.trace: "true"`, key is `alternative_configs.trace`. The key match to `configurator`
 config section (see p.4 below) as prefix in parameters:
 ```yaml
  config:
    parameters:
      alternative_configs.trace.interpreter:    "bash -c"
      alternative_configs.trace.enableCommand:  "echo -n '/etc/telegraf/collector.template.trace' > /var/run/s6/container_environment/COLLECTOR_TEMPLATE"
      alternative_configs.trace.disableCommand: "rm -rf /var/run/s6/container_environment/COLLECTOR_TEMPLATE"
      alternative_configs.trace.reloadCommand:  "s6-svc -d /services/s_collector/run && s6-svc -u /services/s_collector/run"
 ```

4. Adjust configuration file of `configurator` to match with prepared CRD:
Configuration file holds array of elements, each describes the settings of how to inject runtime config into
particular service, for example:
```yaml
#First managed process
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

#Second managed process
- pid_finder:
    supervised_service_name: "nginx"
    service_pattern: '^s6 supervise.*$'
    parent_pattern: '^nginx$'
  config:
    parameters:
      alternative_configs.trace.interpreter: "bash -c"
      alternative_configs.trace.enableCommand: "echo -n '/etc/telegraf/collector.template.trace' > /var/run/s6/container_environment/COLLECTOR_TEMPLATE"
      alternative_configs.trace.disableCommand: "rm -rf /var/run/s6/container_environment/COLLECTOR_TEMPLATE"
      alternative_configs.trace.reloadCommand: "s6-svc -d /services/s_collector/run && s6-svc -u /services/s_collector/run"

#And so on...
- ...
```

__Note the following convention__
* For each key in CRD under `spec->config->parameters` it should be found 1-1 match of the key with prefixes under,
`config->paramters` section. I.e. CRD `spec->config->parameters` key `alternative_configs.trace` match the following parameters:
 ```yaml
  config:
    parameters:
      alternative_configs.trace.interpreter: "bash -c"
      alternative_configs.trace.enableCommand: "echo -n '/etc/telegraf/collector.template.trace' > /var/run/s6/container_environment/COLLECTOR_TEMPLATE"
      alternative_configs.trace.disableCommand: "rm -rf /var/run/s6/container_environment/COLLECTOR_TEMPLATE"
      alternative_configs.trace.reloadCommand: "s6-svc -d /services/s_collector/run && s6-svc -u /services/s_collector/run"
 ```
 Every parameter hase one of the predefined suffixes:
 - `.interpreter` - define which interpreter to use for processing commands
 - `.enableCommand` - command to be performed if corresponding key in CRD set to `true`
 - `.disableCommand` - command to be performed if corresponding key in CRD set to `false` or CRD is deleted
 - `.reloadCommand` - command to be performed  after enable or disable command (this is effectively to apply new settings)

4. Run `configurartor` in a pod in k8s. Make sure you provide necessary rights (see p. 2) to service account under which you've run
the service.
__`configurator` cli interface__:
Example 1: Run inside cluster (`--kubeconfig` parameter is not specified): 
```bash
$ configurator --config=./configs/config.yaml --namespace=my-namespace --podname=my-pod-name -v 2
```

Example 2: Running outside of cluster (just for debugging mode)
```bash
$ export KUBECONFIG=~/config.conf
$ configurator --config=./configs/config.yaml --namespace=my-namespace --podname=my-pod-name -v 2

#Or
$ configurator --kubeconfig=~/.kube/config.conf --config=./configs/config.yaml --namespace=my-namespace --podname=my-pod-name -v 2
```

__`configurator` commad line parameters:__
```bash
Command line options:
  -kubeconfig string
        Path to a kubeconfig. If not specified, then env. var KUBECONFIG examined. Only required if out-of-cluster.
  -config string
        Path to controller config file (mandatory).
  -namespace string
        k8s namespace where this application runs (mandatory).
  -podname string
        k8s pod name where this application runs (mandatory).
  -version
        Print version and exit
 
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