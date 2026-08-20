# Tekton Scheduler (deprecated)

`TektonScheduler` has been replaced by [`TektonKueue`](./TektonKueue.md).
The legacy CRD and `TektonConfig.spec.scheduler` field are retained temporarily
for automatic upgrade migration. Use `TektonKueue` and
`TektonConfig.spec.kueue` for new configuration.
